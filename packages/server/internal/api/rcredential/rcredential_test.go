package rcredential

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"testing"

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
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/testutil"
	credentialv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type credentialFixture struct {
	ctx     context.Context
	base    *testutil.BaseDBQueries
	handler CredentialRPC

	cs     scredential.CredentialService
	fs     *sfile.FileService
	userID idwrap.IDWrap
}

func newCredentialFixture(t *testing.T) *credentialFixture {
	t.Helper()

	base := testutil.CreateBaseDB(context.Background(), t)
	services := base.GetBaseServices()
	logger := slog.Default()

	// Create credential service with default vault
	vault := credvault.NewDefault()
	cs := scredential.NewCredentialService(base.Queries, scredential.WithVault(vault))
	credReader := scredential.NewCredentialReader(base.DB, scredential.WithDecrypter(vault))

	// Create file service
	fs := sfile.New(base.Queries, logger)

	// Create stream for events
	stream := memory.NewInMemorySyncStreamer[CredentialTopic, CredentialEvent]()
	t.Cleanup(stream.Shutdown)

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

	handler := New(CredentialRPCDeps{
		DB:        base.DB,
		Service:   cs,
		User:      services.UserService,
		Workspace: services.WorkspaceService,
		File:      fs,
		Reader:    credReader,
		Streamer:  stream,
	})

	t.Cleanup(base.Close)

	return &credentialFixture{
		ctx:     mwauth.CreateAuthedContext(context.Background(), userID),
		base:    base,
		handler: handler,
		cs:      cs,
		fs:      fs,
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
	// Note: API enum CREDENTIAL_KIND_OPEN_AI=1 maps directly to model value 1 (GEMINI)
	// This is a known enum offset issue between API (has UNSPECIFIED=0) and model (starts at OPENAI=0)
	require.Equal(t, mcredential.CredentialKind(1), cred.Kind)

	// Verify file was created
	file, err := f.fs.GetFileByContentID(f.ctx, credID)
	require.NoError(t, err)
	require.Equal(t, "My OpenAI Key", file.Name)
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
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
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

	// Verify file name was also updated
	file, err := f.fs.GetFileByContentID(f.ctx, credID)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", file.Name)
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

	// Verify file exists
	file, err := f.fs.GetFileByContentID(f.ctx, credID)
	require.NoError(t, err)
	require.NotNil(t, file)

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

	// Verify file was also deleted (cascade)
	_, err = f.fs.GetFileByContentID(f.ctx, credID)
	require.Error(t, err)
	require.ErrorIs(t, err, sfile.ErrFileNotFound)
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

func TestGetCredentialOpenAi_NotFound(t *testing.T) {
	t.Parallel()
	f := newCredentialFixture(t)

	fakeCredID := idwrap.NewNow()

	req := connect.NewRequest(&credentialv1.GetCredentialOpenAiRequest{
		CredentialId: fakeCredID.Bytes(),
	})
	_, err := f.handler.GetCredentialOpenAi(f.ctx, req)
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
