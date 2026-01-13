package rcredential

import (
	"context"
	"database/sql"
	"errors"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/converter"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	credentialv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/credential/v1/credentialv1connect"
)

type CredentialRPC struct {
	DB *sql.DB

	cs scredential.CredentialService
	us suser.UserService
	ws sworkspace.WorkspaceService

	credReader *scredential.CredentialReader

	credStream eventstream.SyncStreamer[CredentialTopic, CredentialEvent]
}

type CredentialTopic struct {
	WorkspaceID idwrap.IDWrap
}

type CredentialEvent struct {
	Type       string
	Credential *credentialv1.Credential
}

type CredentialRPCDeps struct {
	DB        *sql.DB
	Service   scredential.CredentialService
	User      suser.UserService
	Workspace sworkspace.WorkspaceService
	Reader    *scredential.CredentialReader
	Streamer  eventstream.SyncStreamer[CredentialTopic, CredentialEvent]
}

func New(deps CredentialRPCDeps) CredentialRPC {
	return CredentialRPC{
		DB:         deps.DB,
		cs:         deps.Service,
		us:         deps.User,
		ws:         deps.Workspace,
		credReader: deps.Reader,
		credStream: deps.Streamer,
	}
}

func CreateService(srv CredentialRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := credentialv1connect.NewCredentialServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (s *CredentialRPC) CredentialCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[credentialv1.CredentialCollectionResponse], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*credentialv1.Credential
	for _, ws := range workspaces {
		creds, err := s.credReader.ListCredentials(ctx, ws.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range creds {
			items = append(items, converter.ToAPICredential(c))
		}
	}

	return connect.NewResponse(&credentialv1.CredentialCollectionResponse{Items: items}), nil
}

func (s *CredentialRPC) GetCredentialOpenAi(
	ctx context.Context,
	req *connect.Request[credentialv1.GetCredentialOpenAiRequest],
) (*connect.Response[credentialv1.GetCredentialOpenAiResponse], error) {
	credID, err := idwrap.NewFromBytes(req.Msg.CredentialId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	cred, err := s.credReader.GetCredential(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, cred.WorkspaceID)
	if err != nil || !belongs {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
	}

	openai, err := s.credReader.GetCredentialOpenAI(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiOpenAI := converter.ToAPICredentialOpenAI(*openai)
	return connect.NewResponse(&credentialv1.GetCredentialOpenAiResponse{
		CredentialId: apiOpenAI.CredentialId,
		Token:        apiOpenAI.Token,
		BaseUrl:      apiOpenAI.BaseUrl,
	}), nil
}

func (s *CredentialRPC) GetCredentialGemini(
	ctx context.Context,
	req *connect.Request[credentialv1.GetCredentialGeminiRequest],
) (*connect.Response[credentialv1.GetCredentialGeminiResponse], error) {
	credID, err := idwrap.NewFromBytes(req.Msg.CredentialId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	cred, err := s.credReader.GetCredential(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, cred.WorkspaceID)
	if err != nil || !belongs {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
	}

	gemini, err := s.credReader.GetCredentialGemini(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiGemini := converter.ToAPICredentialGemini(*gemini)
	return connect.NewResponse(&credentialv1.GetCredentialGeminiResponse{
		CredentialId: apiGemini.CredentialId,
		ApiKey:       apiGemini.ApiKey,
		BaseUrl:      apiGemini.BaseUrl,
	}), nil
}

func (s *CredentialRPC) GetCredentialAnthropic(
	ctx context.Context,
	req *connect.Request[credentialv1.GetCredentialAnthropicRequest],
) (*connect.Response[credentialv1.GetCredentialAnthropicResponse], error) {
	credID, err := idwrap.NewFromBytes(req.Msg.CredentialId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	cred, err := s.credReader.GetCredential(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, cred.WorkspaceID)
	if err != nil || !belongs {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
	}

	anthropic, err := s.credReader.GetCredentialAnthropic(ctx, credID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	apiAnthropic := converter.ToAPICredentialAnthropic(*anthropic)
	return connect.NewResponse(&credentialv1.GetCredentialAnthropicResponse{
		CredentialId: apiAnthropic.CredentialId,
		ApiKey:       apiAnthropic.ApiKey,
		BaseUrl:      apiAnthropic.BaseUrl,
	}), nil
}

func (s *CredentialRPC) CredentialInsert(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		workspaceID, err := idwrap.NewFromBytes(item.GetWorkspaceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Check user has access to workspace
		belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, workspaceID)
		if err != nil || !belongs {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("access denied"))
		}

		// Create base credential only
		// Provider-specific data (tokens/API keys) must be set via separate calls
		cred := &mcredential.Credential{
			ID:          credID,
			WorkspaceID: workspaceID,
			Name:        item.GetName(),
			Kind:        mcredential.CredentialKind(int8(item.GetKind())), //nolint:gosec // G115: Kind is a small enum (0-2)
		}
		if err := s.cs.CreateCredential(ctx, cred); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish event
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: workspaceID}, CredentialEvent{
				Type:       "insert",
				Credential: converter.ToAPICredential(*cred),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialUpdate(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing credential to check ownership
		existing, err := s.credReader.GetCredential(ctx, credID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, existing.WorkspaceID)
		if err != nil || !belongs {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
		}

		// Update base credential only
		// Provider-specific data (tokens/API keys) must be updated via separate calls
		if item.Name != nil {
			existing.Name = *item.Name
		}
		if err := s.cs.UpdateCredential(ctx, existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish event
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: existing.WorkspaceID}, CredentialEvent{
				Type:       "update",
				Credential: converter.ToAPICredential(*existing),
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialDelete(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	for _, item := range req.Msg.GetItems() {
		credID, err := idwrap.NewFromBytes(item.GetCredentialId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		// Get existing credential
		existing, err := s.credReader.GetCredential(ctx, credID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue // Already deleted
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		belongs, err := s.us.CheckUserBelongsToWorkspace(ctx, userID, existing.WorkspaceID)
		if err != nil || !belongs {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("credential not found"))
		}

		// Delete credential (provider-specific cascade handled by DB)
		if err := s.cs.DeleteCredential(ctx, credID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish event
		if s.credStream != nil {
			s.credStream.Publish(CredentialTopic{WorkspaceID: existing.WorkspaceID}, CredentialEvent{
				Type:       "delete",
				Credential: &credentialv1.Credential{CredentialId: credID.Bytes()},
			})
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *CredentialRPC) CredentialSync(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[credentialv1.CredentialSyncResponse],
) error {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Build initial collection
	var items []*credentialv1.Credential
	for _, ws := range workspaces {
		creds, err := s.credReader.ListCredentials(ctx, ws.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		for _, c := range creds {
			items = append(items, converter.ToAPICredential(c))
		}
	}

	// Convert to sync items format
	syncItems := make([]*credentialv1.CredentialSync, 0, len(items))
	for _, item := range items {
		syncItems = append(syncItems, &credentialv1.CredentialSync{
			Value: &credentialv1.CredentialSync_ValueUnion{
				Kind: credentialv1.CredentialSync_ValueUnion_KIND_UPSERT,
				Upsert: &credentialv1.CredentialSyncUpsert{
					CredentialId: item.CredentialId,
					Name:         item.Name,
					Kind:         item.Kind,
				},
			},
		})
	}

	// Send initial collection as upsert items
	if err := stream.Send(&credentialv1.CredentialSyncResponse{
		Items: syncItems,
	}); err != nil {
		return err
	}

	// Wait for context cancellation (real-time streaming not yet implemented)
	<-ctx.Done()
	return nil
}
