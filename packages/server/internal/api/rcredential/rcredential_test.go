package rcredential

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/credvault"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream/memory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/muser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	credentialv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type credentialFixture struct {
	ctx     context.Context
	base    *testutil.BaseDBQueries
	handler CredentialRPC

	cs     scredential.CredentialService
	userID idwrap.IDWrap
}

func newCredentialFixture(t *testing.T) *credentialFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()

	// Create credential service with default vault
	vault := credvault.NewDefault()
	cs := scredential.NewCredentialService(base.Queries, scredential.WithVault(vault))
	credReader := scredential.NewCredentialReader(base.DB, scredential.WithDecrypter(vault))

	// Create streamers for events
	credStream := memory.NewInMemorySyncStreamer[CredentialTopic, CredentialEvent]()
	openAiStream := memory.NewInMemorySyncStreamer[CredentialOpenAiTopic, CredentialOpenAiEvent]()
	geminiStream := memory.NewInMemorySyncStreamer[CredentialGeminiTopic, CredentialGeminiEvent]()
	anthropicStream := memory.NewInMemorySyncStreamer[CredentialAnthropicTopic, CredentialAnthropicEvent]()
	t.Cleanup(credStream.Shutdown)
	t.Cleanup(openAiStream.Shutdown)
	t.Cleanup(geminiStream.Shutdown)
	t.Cleanup(anthropicStream.Shutdown)

	// Create user
	userID := idwrap.NewNow()
	providerID := fmt.Sprintf("test-%s", userID.String())
	err := services.UserService.CreateUser(context.Background(), &muser.User{
		ID:           userID,
		Email:        fmt.Sprintf("%s@example.com", userID.String()),
		Password:     []byte("password"),
		ProviderID:   &providerID,
		ProviderType: muser.MagicLink,
		Status:       muser.Active,
	})
	require.NoError(t, err, "create user")

	userReader := sworkspace.NewUserReader(base.DB)

	handler := New(CredentialRPCDeps{
		DB: base.DB,
		Services: CredentialRPCServices{
			Credential: cs,
			User:       services.UserService,
			Workspace:  services.WorkspaceService,
		},
		Readers: CredentialRPCReaders{
			Credential: credReader,
			User:       userReader,
		},
		Streamers: CredentialRPCStreamers{
			Credential: credStream,
			OpenAi:     openAiStream,
			Gemini:     geminiStream,
			Anthropic:  anthropicStream,
		},
	})

	t.Cleanup(base.Close)

	return &credentialFixture{
		ctx:     mwauth.CreateAuthedContext(context.Background(), userID),
		base:    base,
		handler: handler,
		cs:      cs,
		userID:  userID,
	}
}

func (f *credentialFixture) createWorkspace(t *testing.T, name string) idwrap.IDWrap {
	t.Helper()

	services := f.base.GetBaseServices()
	envService := senv.NewEnvironmentService(f.base.Queries, f.base.Logger())

	workspaceID := idwrap.NewNow()
	envID := idwrap.NewNow()

	ws := &mworkspace.Workspace{
		ID:        workspaceID,
		Name:      name,
		Updated:   dbtime.DBNow(),
		ActiveEnv: envID,
		GlobalEnv: envID,
	}
	err := services.WorkspaceService.Create(f.ctx, ws)
	require.NoError(t, err, "create workspace")

	env := menv.Env{
		ID:          envID,
		WorkspaceID: workspaceID,
		Name:        "default",
		Type:        menv.EnvGlobal,
	}
	err = envService.CreateEnvironment(f.ctx, &env)
	require.NoError(t, err, "create environment")

	member := &mworkspace.WorkspaceUser{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      f.userID,
		Role:        mworkspace.RoleOwner,
	}
	err = services.WorkspaceUserService.CreateWorkspaceUser(f.ctx, member)
	require.NoError(t, err, "create workspace user")

	return workspaceID
}

// createCredential is a helper that creates a credential directly in the database
func (f *credentialFixture) createCredential(t *testing.T, wsID idwrap.IDWrap, name string, kind credentialv1.CredentialKind) idwrap.IDWrap {
	t.Helper()

	credID := idwrap.NewNow()
	req := connect.NewRequest(&credentialv1.CredentialInsertRequest{
		Items: []*credentialv1.CredentialInsert{{
			CredentialId: credID.Bytes(),
			WorkspaceId:  wsID.Bytes(),
			Name:         name,
			Kind:         kind,
		}},
	})
	_, err := f.handler.CredentialInsert(f.ctx, req)
	require.NoError(t, err, "create credential")
	return credID
}

// --- Credential CRUD Tests ---

func TestCredentialInsert_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := idwrap.NewNow()

	req := connect.NewRequest(&credentialv1.CredentialInsertRequest{
		Items: []*credentialv1.CredentialInsert{{
			CredentialId: credID.Bytes(),
			WorkspaceId:  wsID.Bytes(),
			Name:         "My OpenAI Key",
			Kind:         credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI,
		}},
	})

	_, err := f.handler.CredentialInsert(f.ctx, req)
	require.NoError(t, err)

	// Verify credential was created
	cred, err := f.cs.GetCredential(f.ctx, credID)
	require.NoError(t, err)
	require.Equal(t, "My OpenAI Key", cred.Name)
	require.Equal(t, mcredential.CREDENTIAL_KIND_OPENAI, cred.Kind)
}

func TestCredentialInsert_InvalidWorkspace(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	// Use a workspace ID that doesn't exist / user doesn't have access to
	fakeWsID := idwrap.NewNow()
	credID := idwrap.NewNow()

	req := connect.NewRequest(&credentialv1.CredentialInsertRequest{
		Items: []*credentialv1.CredentialInsert{{
			CredentialId: credID.Bytes(),
			WorkspaceId:  fakeWsID.Bytes(),
			Name:         "Should Fail",
			Kind:         credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI,
		}},
	})

	_, err := f.handler.CredentialInsert(f.ctx, req)
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestCredentialUpdate_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := idwrap.NewNow()

	// Insert credential first
	insertReq := connect.NewRequest(&credentialv1.CredentialInsertRequest{
		Items: []*credentialv1.CredentialInsert{{
			CredentialId: credID.Bytes(),
			WorkspaceId:  wsID.Bytes(),
			Name:         "Original Name",
			Kind:         credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI,
		}},
	})
	_, err := f.handler.CredentialInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Update the name
	newName := "Updated Name"
	updateReq := connect.NewRequest(&credentialv1.CredentialUpdateRequest{
		Items: []*credentialv1.CredentialUpdate{{
			CredentialId: credID.Bytes(),
			Name:         &newName,
		}},
	})
	_, err = f.handler.CredentialUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Verify credential was updated
	cred, err := f.cs.GetCredential(f.ctx, credID)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", cred.Name)
}

func TestCredentialUpdate_NotFound(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	fakeCredID := idwrap.NewNow()
	newName := "Updated Name"

	updateReq := connect.NewRequest(&credentialv1.CredentialUpdateRequest{
		Items: []*credentialv1.CredentialUpdate{{
			CredentialId: fakeCredID.Bytes(),
			Name:         &newName,
		}},
	})
	_, err := f.handler.CredentialUpdate(f.ctx, updateReq)
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestCredentialDelete_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := idwrap.NewNow()

	// Insert credential first
	insertReq := connect.NewRequest(&credentialv1.CredentialInsertRequest{
		Items: []*credentialv1.CredentialInsert{{
			CredentialId: credID.Bytes(),
			WorkspaceId:  wsID.Bytes(),
			Name:         "To Be Deleted",
			Kind:         credentialv1.CredentialKind_CREDENTIAL_KIND_GEMINI,
		}},
	})
	_, err := f.handler.CredentialInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Verify credential exists
	cred, err := f.cs.GetCredential(f.ctx, credID)
	require.NoError(t, err)
	require.NotNil(t, cred)

	// Delete credential
	deleteReq := connect.NewRequest(&credentialv1.CredentialDeleteRequest{
		Items: []*credentialv1.CredentialDelete{{
			CredentialId: credID.Bytes(),
		}},
	})
	_, err = f.handler.CredentialDelete(f.ctx, deleteReq)
	require.NoError(t, err)

	// Verify credential was deleted
	_, err = f.cs.GetCredential(f.ctx, credID)
	require.Error(t, err)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestCredentialDelete_AlreadyDeleted(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	fakeCredID := idwrap.NewNow()

	// Deleting non-existent credential should succeed (idempotent)
	deleteReq := connect.NewRequest(&credentialv1.CredentialDeleteRequest{
		Items: []*credentialv1.CredentialDelete{{
			CredentialId: fakeCredID.Bytes(),
		}},
	})
	_, err := f.handler.CredentialDelete(f.ctx, deleteReq)
	require.NoError(t, err) // Should not error for already-deleted items
}

func TestCredentialCollection_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")

	// Insert multiple credentials
	for i := range 3 {
		credID := idwrap.NewNow()
		insertReq := connect.NewRequest(&credentialv1.CredentialInsertRequest{
			Items: []*credentialv1.CredentialInsert{{
				CredentialId: credID.Bytes(),
				WorkspaceId:  wsID.Bytes(),
				Name:         fmt.Sprintf("Credential %d", i),
				Kind:         credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI,
			}},
		})
		_, err := f.handler.CredentialInsert(f.ctx, insertReq)
		require.NoError(t, err)
	}

	// Get collection
	resp, err := f.handler.CredentialCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 3)
}

// --- CredentialOpenAi CRUD Tests ---

func TestCredentialOpenAiInsert_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "OpenAI Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	// Insert OpenAI secret
	insertReq := connect.NewRequest(&credentialv1.CredentialOpenAiInsertRequest{
		Items: []*credentialv1.CredentialOpenAiInsert{{
			CredentialId: credID.Bytes(),
			Token:        "sk-test-token-12345",
		}},
	})
	_, err := f.handler.CredentialOpenAiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Verify collection returns the secret
	resp, err := f.handler.CredentialOpenAiCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1)
	require.Equal(t, credID.Bytes(), resp.Msg.Items[0].CredentialId)
	require.Equal(t, "sk-test-token-12345", resp.Msg.Items[0].Token)
}

func TestCredentialOpenAiInsert_WithBaseUrl(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "OpenAI Custom", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	baseUrl := "https://api.openai.example.com"
	insertReq := connect.NewRequest(&credentialv1.CredentialOpenAiInsertRequest{
		Items: []*credentialv1.CredentialOpenAiInsert{{
			CredentialId: credID.Bytes(),
			Token:        "sk-custom-token",
			BaseUrl:      &baseUrl,
		}},
	})
	_, err := f.handler.CredentialOpenAiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	resp, err := f.handler.CredentialOpenAiCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1)
	require.NotNil(t, resp.Msg.Items[0].BaseUrl)
	require.Equal(t, baseUrl, *resp.Msg.Items[0].BaseUrl)
}

func TestCredentialOpenAiUpdate_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "OpenAI Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	// Insert OpenAI secret
	insertReq := connect.NewRequest(&credentialv1.CredentialOpenAiInsertRequest{
		Items: []*credentialv1.CredentialOpenAiInsert{{
			CredentialId: credID.Bytes(),
			Token:        "old-token",
		}},
	})
	_, err := f.handler.CredentialOpenAiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Update token
	newToken := "new-updated-token"
	updateReq := connect.NewRequest(&credentialv1.CredentialOpenAiUpdateRequest{
		Items: []*credentialv1.CredentialOpenAiUpdate{{
			CredentialId: credID.Bytes(),
			Token:        &newToken,
		}},
	})
	_, err = f.handler.CredentialOpenAiUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	// Verify update
	resp, err := f.handler.CredentialOpenAiCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1)
	require.Equal(t, "new-updated-token", resp.Msg.Items[0].Token)
}

func TestCredentialOpenAiDelete_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "OpenAI Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	// Insert OpenAI secret
	insertReq := connect.NewRequest(&credentialv1.CredentialOpenAiInsertRequest{
		Items: []*credentialv1.CredentialOpenAiInsert{{
			CredentialId: credID.Bytes(),
			Token:        "token-to-delete",
		}},
	})
	_, err := f.handler.CredentialOpenAiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	// Delete (cascades to credential)
	deleteReq := connect.NewRequest(&credentialv1.CredentialOpenAiDeleteRequest{
		Items: []*credentialv1.CredentialOpenAiDelete{{
			CredentialId: credID.Bytes(),
		}},
	})
	_, err = f.handler.CredentialOpenAiDelete(f.ctx, deleteReq)
	require.NoError(t, err)

	// Verify parent credential was also deleted
	_, err = f.cs.GetCredential(f.ctx, credID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

// --- CredentialGemini CRUD Tests ---

func TestCredentialGeminiInsert_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Gemini Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_GEMINI)

	insertReq := connect.NewRequest(&credentialv1.CredentialGeminiInsertRequest{
		Items: []*credentialv1.CredentialGeminiInsert{{
			CredentialId: credID.Bytes(),
			ApiKey:       "gemini-api-key-12345",
		}},
	})
	_, err := f.handler.CredentialGeminiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	resp, err := f.handler.CredentialGeminiCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1)
	require.Equal(t, credID.Bytes(), resp.Msg.Items[0].CredentialId)
	require.Equal(t, "gemini-api-key-12345", resp.Msg.Items[0].ApiKey)
}

func TestCredentialGeminiUpdate_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Gemini Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_GEMINI)

	insertReq := connect.NewRequest(&credentialv1.CredentialGeminiInsertRequest{
		Items: []*credentialv1.CredentialGeminiInsert{{
			CredentialId: credID.Bytes(),
			ApiKey:       "old-api-key",
		}},
	})
	_, err := f.handler.CredentialGeminiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	newApiKey := "new-api-key"
	updateReq := connect.NewRequest(&credentialv1.CredentialGeminiUpdateRequest{
		Items: []*credentialv1.CredentialGeminiUpdate{{
			CredentialId: credID.Bytes(),
			ApiKey:       &newApiKey,
		}},
	})
	_, err = f.handler.CredentialGeminiUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	resp, err := f.handler.CredentialGeminiCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1)
	require.Equal(t, "new-api-key", resp.Msg.Items[0].ApiKey)
}

func TestCredentialGeminiDelete_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Gemini Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_GEMINI)

	insertReq := connect.NewRequest(&credentialv1.CredentialGeminiInsertRequest{
		Items: []*credentialv1.CredentialGeminiInsert{{
			CredentialId: credID.Bytes(),
			ApiKey:       "key-to-delete",
		}},
	})
	_, err := f.handler.CredentialGeminiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	deleteReq := connect.NewRequest(&credentialv1.CredentialGeminiDeleteRequest{
		Items: []*credentialv1.CredentialGeminiDelete{{
			CredentialId: credID.Bytes(),
		}},
	})
	_, err = f.handler.CredentialGeminiDelete(f.ctx, deleteReq)
	require.NoError(t, err)

	// Verify parent credential was also deleted
	_, err = f.cs.GetCredential(f.ctx, credID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

// --- CredentialAnthropic CRUD Tests ---

func TestCredentialAnthropicInsert_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Anthropic Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_ANTHROPIC)

	insertReq := connect.NewRequest(&credentialv1.CredentialAnthropicInsertRequest{
		Items: []*credentialv1.CredentialAnthropicInsert{{
			CredentialId: credID.Bytes(),
			ApiKey:       "anthropic-api-key-12345",
		}},
	})
	_, err := f.handler.CredentialAnthropicInsert(f.ctx, insertReq)
	require.NoError(t, err)

	resp, err := f.handler.CredentialAnthropicCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1)
	require.Equal(t, credID.Bytes(), resp.Msg.Items[0].CredentialId)
	require.Equal(t, "anthropic-api-key-12345", resp.Msg.Items[0].ApiKey)
}

func TestCredentialAnthropicUpdate_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Anthropic Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_ANTHROPIC)

	insertReq := connect.NewRequest(&credentialv1.CredentialAnthropicInsertRequest{
		Items: []*credentialv1.CredentialAnthropicInsert{{
			CredentialId: credID.Bytes(),
			ApiKey:       "old-anthropic-key",
		}},
	})
	_, err := f.handler.CredentialAnthropicInsert(f.ctx, insertReq)
	require.NoError(t, err)

	newApiKey := "new-anthropic-key"
	updateReq := connect.NewRequest(&credentialv1.CredentialAnthropicUpdateRequest{
		Items: []*credentialv1.CredentialAnthropicUpdate{{
			CredentialId: credID.Bytes(),
			ApiKey:       &newApiKey,
		}},
	})
	_, err = f.handler.CredentialAnthropicUpdate(f.ctx, updateReq)
	require.NoError(t, err)

	resp, err := f.handler.CredentialAnthropicCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1)
	require.Equal(t, "new-anthropic-key", resp.Msg.Items[0].ApiKey)
}

func TestCredentialAnthropicDelete_Success(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Anthropic Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_ANTHROPIC)

	insertReq := connect.NewRequest(&credentialv1.CredentialAnthropicInsertRequest{
		Items: []*credentialv1.CredentialAnthropicInsert{{
			CredentialId: credID.Bytes(),
			ApiKey:       "key-to-delete",
		}},
	})
	_, err := f.handler.CredentialAnthropicInsert(f.ctx, insertReq)
	require.NoError(t, err)

	deleteReq := connect.NewRequest(&credentialv1.CredentialAnthropicDeleteRequest{
		Items: []*credentialv1.CredentialAnthropicDelete{{
			CredentialId: credID.Bytes(),
		}},
	})
	_, err = f.handler.CredentialAnthropicDelete(f.ctx, deleteReq)
	require.NoError(t, err)

	// Verify parent credential was also deleted
	_, err = f.cs.GetCredential(f.ctx, credID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

// --- Sync Tests ---

func TestCredentialCollection_ReturnsCorrectData(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")

	// Create credentials
	var createdIDs []idwrap.IDWrap
	for i := range 2 {
		credID := idwrap.NewNow()
		insertReq := connect.NewRequest(&credentialv1.CredentialInsertRequest{
			Items: []*credentialv1.CredentialInsert{{
				CredentialId: credID.Bytes(),
				WorkspaceId:  wsID.Bytes(),
				Name:         fmt.Sprintf("Cred %d", i),
				Kind:         credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI,
			}},
		})
		_, err := f.handler.CredentialInsert(f.ctx, insertReq)
		require.NoError(t, err)
		createdIDs = append(createdIDs, credID)
	}

	// Use CredentialCollection to verify data (sync initial collection uses same data source)
	resp, err := f.handler.CredentialCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 2, "expected 2 credentials")

	// Verify both credentials are returned
	foundIDs := make(map[string]bool)
	for _, item := range resp.Msg.Items {
		id, _ := idwrap.NewFromBytes(item.CredentialId)
		foundIDs[id.String()] = true
	}
	for _, id := range createdIDs {
		require.True(t, foundIDs[id.String()], "credential %s not found in collection", id.String())
	}
}

func TestCredentialSyncFiltersUnauthorizedWorkspaces(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	// Create authorized workspace
	wsID := f.createWorkspace(t, "authorized-workspace")
	credID := f.createCredential(t, wsID, "Authorized Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	// Create unauthorized workspace (no membership)
	services := f.base.GetBaseServices()
	unauthorizedWsID := idwrap.NewNow()
	err := services.WorkspaceService.Create(context.Background(), &mworkspace.Workspace{
		ID:      unauthorizedWsID,
		Name:    "unauthorized-workspace",
		Updated: dbtime.DBNow(),
	})
	require.NoError(t, err, "create unauthorized workspace")

	// Insert credential in unauthorized workspace directly (bypassing permission checks)
	unauthorizedCredID := idwrap.NewNow()
	err = f.cs.CreateCredential(context.Background(), &mcredential.Credential{
		ID:          unauthorizedCredID,
		WorkspaceID: unauthorizedWsID,
		Name:        "Unauthorized Cred",
		Kind:        mcredential.CREDENTIAL_KIND_OPENAI,
	})
	require.NoError(t, err, "create unauthorized credential")

	// Get collection - should only return authorized credential
	resp, err := f.handler.CredentialCollection(f.ctx, connect.NewRequest(&emptypb.Empty{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Items, 1, "should only see authorized credential")

	returnedID, _ := idwrap.NewFromBytes(resp.Msg.Items[0].CredentialId)
	require.Equal(t, credID, returnedID, "should only see authorized credential")
}

// --- Concurrent Tests ---

func TestCredentialDeleteConcurrent(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")

	// Create multiple credentials
	var credIDs []idwrap.IDWrap
	for i := 0; i < 10; i++ {
		credID := f.createCredential(t, wsID, fmt.Sprintf("Cred %d", i), credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)
		credIDs = append(credIDs, credID)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(credIDs))

	for _, credID := range credIDs {
		wg.Add(1)
		go func(id idwrap.IDWrap) {
			defer wg.Done()
			req := connect.NewRequest(&credentialv1.CredentialDeleteRequest{
				Items: []*credentialv1.CredentialDelete{{CredentialId: id.Bytes()}},
			})
			_, err := f.handler.CredentialDelete(f.ctx, req)
			if err != nil {
				errCh <- err
			}
		}(credID)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(errCh)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: concurrent deletes timed out")
	}

	for err := range errCh {
		require.NoError(t, err, "concurrent delete failed")
	}
}

func TestCredentialUpdateConcurrent(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")

	// Create multiple credentials
	var credIDs []idwrap.IDWrap
	for i := 0; i < 10; i++ {
		credID := f.createCredential(t, wsID, fmt.Sprintf("Cred %d", i), credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)
		credIDs = append(credIDs, credID)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(credIDs))

	for i, credID := range credIDs {
		wg.Add(1)
		go func(id idwrap.IDWrap, idx int) {
			defer wg.Done()
			newName := fmt.Sprintf("updated-%d", idx)
			req := connect.NewRequest(&credentialv1.CredentialUpdateRequest{
				Items: []*credentialv1.CredentialUpdate{{
					CredentialId: id.Bytes(),
					Name:         &newName,
				}},
			})
			_, err := f.handler.CredentialUpdate(f.ctx, req)
			if err != nil {
				errCh <- err
			}
		}(credID, i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(errCh)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: concurrent updates timed out")
	}

	for err := range errCh {
		require.NoError(t, err, "concurrent update failed")
	}
}

func TestCredentialOpenAiUpdateConcurrent(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")

	// Create multiple credentials with OpenAI secrets
	var credIDs []idwrap.IDWrap
	for i := 0; i < 10; i++ {
		credID := f.createCredential(t, wsID, fmt.Sprintf("OpenAI Cred %d", i), credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

		// Insert OpenAI secret
		insertReq := connect.NewRequest(&credentialv1.CredentialOpenAiInsertRequest{
			Items: []*credentialv1.CredentialOpenAiInsert{{
				CredentialId: credID.Bytes(),
				Token:        fmt.Sprintf("token-%d", i),
			}},
		})
		_, err := f.handler.CredentialOpenAiInsert(f.ctx, insertReq)
		require.NoError(t, err)

		credIDs = append(credIDs, credID)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(credIDs))

	for i, credID := range credIDs {
		wg.Add(1)
		go func(id idwrap.IDWrap, idx int) {
			defer wg.Done()
			newToken := fmt.Sprintf("updated-token-%d", idx)
			req := connect.NewRequest(&credentialv1.CredentialOpenAiUpdateRequest{
				Items: []*credentialv1.CredentialOpenAiUpdate{{
					CredentialId: id.Bytes(),
					Token:        &newToken,
				}},
			})
			_, err := f.handler.CredentialOpenAiUpdate(f.ctx, req)
			if err != nil {
				errCh <- err
			}
		}(credID, i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
		close(errCh)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: concurrent OpenAI updates timed out")
	}

	for err := range errCh {
		require.NoError(t, err, "concurrent OpenAI update failed")
	}
}

// --- Stream Sync Tests ---

func collectCredentialSyncItems(t *testing.T, ch <-chan *credentialv1.CredentialSyncResponse, count int) []*credentialv1.CredentialSync {
	t.Helper()
	var items []*credentialv1.CredentialSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before receiving %d items (got %d)", count, len(items))
			}
			items = append(items, resp.Items...)
		case <-timeout:
			t.Fatalf("timeout waiting for %d items (got %d)", count, len(items))
		}
	}
	return items
}

func collectCredentialOpenAiSyncItems(t *testing.T, ch <-chan *credentialv1.CredentialOpenAiSyncResponse, count int) []*credentialv1.CredentialOpenAiSync {
	t.Helper()
	var items []*credentialv1.CredentialOpenAiSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before receiving %d items (got %d)", count, len(items))
			}
			items = append(items, resp.Items...)
		case <-timeout:
			t.Fatalf("timeout waiting for %d items (got %d)", count, len(items))
		}
	}
	return items
}

func TestCredentialSyncStreamsUpdates(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Test Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *credentialv1.CredentialSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamCredentialSync(ctx, f.userID, func(resp *credentialv1.CredentialSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Verify NO snapshot arrives (snapshots removed in favor of *Collection RPCs)
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active, no snapshot sent
	}

	// Test live UPDATE event
	newName := "updated cred"
	req := connect.NewRequest(&credentialv1.CredentialUpdateRequest{
		Items: []*credentialv1.CredentialUpdate{{
			CredentialId: credID.Bytes(),
			Name:         &newName,
		}},
	})
	_, err := f.handler.CredentialUpdate(f.ctx, req)
	require.NoError(t, err, "CredentialUpdate")

	updateItems := collectCredentialSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	require.NotNil(t, updateVal, "update response missing value union")
	require.Equal(t, credentialv1.CredentialSync_ValueUnion_KIND_UPSERT, updateVal.GetKind())
	require.Equal(t, newName, updateVal.GetUpsert().GetName())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestCredentialOpenAiSyncStreamsUpdates(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "OpenAI Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	// Insert OpenAI secret
	insertReq := connect.NewRequest(&credentialv1.CredentialOpenAiInsertRequest{
		Items: []*credentialv1.CredentialOpenAiInsert{{
			CredentialId: credID.Bytes(),
			Token:        "initial-token",
		}},
	})
	_, err := f.handler.CredentialOpenAiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *credentialv1.CredentialOpenAiSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamCredentialOpenAiSync(ctx, f.userID, func(resp *credentialv1.CredentialOpenAiSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Verify NO snapshot arrives
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active, no snapshot sent
	}

	// Test live UPDATE event
	newToken := "updated-token"
	updateReq := connect.NewRequest(&credentialv1.CredentialOpenAiUpdateRequest{
		Items: []*credentialv1.CredentialOpenAiUpdate{{
			CredentialId: credID.Bytes(),
			Token:        &newToken,
		}},
	})
	_, err = f.handler.CredentialOpenAiUpdate(f.ctx, updateReq)
	require.NoError(t, err, "CredentialOpenAiUpdate")

	updateItems := collectCredentialOpenAiSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	require.NotNil(t, updateVal, "update response missing value union")
	require.Equal(t, credentialv1.CredentialOpenAiSync_ValueUnion_KIND_UPSERT, updateVal.GetKind())
	require.Equal(t, newToken, updateVal.GetUpsert().GetToken())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func collectCredentialGeminiSyncItems(t *testing.T, ch <-chan *credentialv1.CredentialGeminiSyncResponse, count int) []*credentialv1.CredentialGeminiSync {
	t.Helper()
	var items []*credentialv1.CredentialGeminiSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before receiving %d items (got %d)", count, len(items))
			}
			items = append(items, resp.Items...)
		case <-timeout:
			t.Fatalf("timeout waiting for %d items (got %d)", count, len(items))
		}
	}
	return items
}

func collectCredentialAnthropicSyncItems(t *testing.T, ch <-chan *credentialv1.CredentialAnthropicSyncResponse, count int) []*credentialv1.CredentialAnthropicSync {
	t.Helper()
	var items []*credentialv1.CredentialAnthropicSync
	timeout := time.After(2 * time.Second)
	for len(items) < count {
		select {
		case resp, ok := <-ch:
			if !ok {
				t.Fatalf("channel closed before receiving %d items (got %d)", count, len(items))
			}
			items = append(items, resp.Items...)
		case <-timeout:
			t.Fatalf("timeout waiting for %d items (got %d)", count, len(items))
		}
	}
	return items
}

func TestCredentialGeminiSyncStreamsUpdates(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Gemini Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_GEMINI)

	// Insert Gemini secret
	insertReq := connect.NewRequest(&credentialv1.CredentialGeminiInsertRequest{
		Items: []*credentialv1.CredentialGeminiInsert{{
			CredentialId: credID.Bytes(),
			ApiKey:       "initial-api-key",
		}},
	})
	_, err := f.handler.CredentialGeminiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *credentialv1.CredentialGeminiSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamCredentialGeminiSync(ctx, f.userID, func(resp *credentialv1.CredentialGeminiSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Verify NO snapshot arrives
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active, no snapshot sent
	}

	// Test live UPDATE event
	newApiKey := "updated-api-key"
	updateReq := connect.NewRequest(&credentialv1.CredentialGeminiUpdateRequest{
		Items: []*credentialv1.CredentialGeminiUpdate{{
			CredentialId: credID.Bytes(),
			ApiKey:       &newApiKey,
		}},
	})
	_, err = f.handler.CredentialGeminiUpdate(f.ctx, updateReq)
	require.NoError(t, err, "CredentialGeminiUpdate")

	updateItems := collectCredentialGeminiSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	require.NotNil(t, updateVal, "update response missing value union")
	require.Equal(t, credentialv1.CredentialGeminiSync_ValueUnion_KIND_UPSERT, updateVal.GetKind())
	require.Equal(t, newApiKey, updateVal.GetUpsert().GetApiKey())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestCredentialAnthropicSyncStreamsUpdates(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Anthropic Cred", credentialv1.CredentialKind_CREDENTIAL_KIND_ANTHROPIC)

	// Insert Anthropic secret
	insertReq := connect.NewRequest(&credentialv1.CredentialAnthropicInsertRequest{
		Items: []*credentialv1.CredentialAnthropicInsert{{
			CredentialId: credID.Bytes(),
			ApiKey:       "initial-api-key",
		}},
	})
	_, err := f.handler.CredentialAnthropicInsert(f.ctx, insertReq)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *credentialv1.CredentialAnthropicSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamCredentialAnthropicSync(ctx, f.userID, func(resp *credentialv1.CredentialAnthropicSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Verify NO snapshot arrives
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active, no snapshot sent
	}

	// Test live UPDATE event
	newApiKey := "updated-api-key"
	updateReq := connect.NewRequest(&credentialv1.CredentialAnthropicUpdateRequest{
		Items: []*credentialv1.CredentialAnthropicUpdate{{
			CredentialId: credID.Bytes(),
			ApiKey:       &newApiKey,
		}},
	})
	_, err = f.handler.CredentialAnthropicUpdate(f.ctx, updateReq)
	require.NoError(t, err, "CredentialAnthropicUpdate")

	updateItems := collectCredentialAnthropicSyncItems(t, msgCh, 1)
	updateVal := updateItems[0].GetValue()
	require.NotNil(t, updateVal, "update response missing value union")
	require.Equal(t, credentialv1.CredentialAnthropicSync_ValueUnion_KIND_UPSERT, updateVal.GetKind())
	require.Equal(t, newApiKey, updateVal.GetUpsert().GetApiKey())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestCredentialSyncStreamsDeleteEvents(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "Cred To Delete", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *credentialv1.CredentialSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamCredentialSync(ctx, f.userID, func(resp *credentialv1.CredentialSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for stream to be active
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active
	}

	// Delete the credential
	deleteReq := connect.NewRequest(&credentialv1.CredentialDeleteRequest{
		Items: []*credentialv1.CredentialDelete{{
			CredentialId: credID.Bytes(),
		}},
	})
	_, err := f.handler.CredentialDelete(f.ctx, deleteReq)
	require.NoError(t, err, "CredentialDelete")

	// Verify DELETE event received
	deleteItems := collectCredentialSyncItems(t, msgCh, 1)
	deleteVal := deleteItems[0].GetValue()
	require.NotNil(t, deleteVal, "delete response missing value union")
	require.Equal(t, credentialv1.CredentialSync_ValueUnion_KIND_DELETE, deleteVal.GetKind())
	require.Equal(t, credID.Bytes(), deleteVal.GetDelete().GetCredentialId())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}

func TestCredentialOpenAiSyncStreamsDeleteEvents(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	wsID := f.createWorkspace(t, "test-workspace")
	credID := f.createCredential(t, wsID, "OpenAI To Delete", credentialv1.CredentialKind_CREDENTIAL_KIND_OPEN_AI)

	// Insert OpenAI secret
	insertReq := connect.NewRequest(&credentialv1.CredentialOpenAiInsertRequest{
		Items: []*credentialv1.CredentialOpenAiInsert{{
			CredentialId: credID.Bytes(),
			Token:        "token-to-delete",
		}},
	})
	_, err := f.handler.CredentialOpenAiInsert(f.ctx, insertReq)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	msgCh := make(chan *credentialv1.CredentialOpenAiSyncResponse, 10)
	errCh := make(chan error, 1)

	go func() {
		err := f.handler.streamCredentialOpenAiSync(ctx, f.userID, func(resp *credentialv1.CredentialOpenAiSyncResponse) error {
			msgCh <- resp
			return nil
		})
		errCh <- err
		close(msgCh)
	}()

	// Wait for stream to be active
	select {
	case <-msgCh:
		require.FailNow(t, "received unexpected snapshot item")
	case <-time.After(100 * time.Millisecond):
		// Good - stream active
	}

	// Delete the OpenAI credential (cascades to parent)
	deleteReq := connect.NewRequest(&credentialv1.CredentialOpenAiDeleteRequest{
		Items: []*credentialv1.CredentialOpenAiDelete{{
			CredentialId: credID.Bytes(),
		}},
	})
	_, err = f.handler.CredentialOpenAiDelete(f.ctx, deleteReq)
	require.NoError(t, err, "CredentialOpenAiDelete")

	// Verify DELETE event received
	deleteItems := collectCredentialOpenAiSyncItems(t, msgCh, 1)
	deleteVal := deleteItems[0].GetValue()
	require.NotNil(t, deleteVal, "delete response missing value union")
	require.Equal(t, credentialv1.CredentialOpenAiSync_ValueUnion_KIND_DELETE, deleteVal.GetKind())
	require.Equal(t, credID.Bytes(), deleteVal.GetDelete().GetCredentialId())

	cancel()
	err = <-errCh
	if err != nil {
		require.ErrorIs(t, err, context.Canceled)
	}
}
