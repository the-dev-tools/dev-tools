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
) (*connect.Response[credentialv1.CredentialOpenAi], error) {
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

	return connect.NewResponse(converter.ToAPICredentialOpenAI(*openai)), nil
}

func (s *CredentialRPC) GetCredentialGemini(
	ctx context.Context,
	req *connect.Request[credentialv1.GetCredentialGeminiRequest],
) (*connect.Response[credentialv1.CredentialGemini], error) {
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

	return connect.NewResponse(converter.ToAPICredentialGemini(*gemini)), nil
}

func (s *CredentialRPC) GetCredentialAnthropic(
	ctx context.Context,
	req *connect.Request[credentialv1.GetCredentialAnthropicRequest],
) (*connect.Response[credentialv1.CredentialAnthropic], error) {
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

	return connect.NewResponse(converter.ToAPICredentialAnthropic(*anthropic)), nil
}

func (s *CredentialRPC) CredentialInsert(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *CredentialRPC) CredentialUpdate(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *CredentialRPC) CredentialDelete(
	ctx context.Context,
	req *connect.Request[credentialv1.CredentialDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (s *CredentialRPC) CredentialSync(
	ctx context.Context,
	req *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[credentialv1.CredentialSyncResponse],
) error {
	return connect.NewError(connect.CodeUnimplemented, nil)
}