package renv

import (
	"context"
	"database/sql"
	"time"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	apiv1 "the-dev-tools/spec/dist/buf/go/api/environment/v1"
	"the-dev-tools/spec/dist/buf/go/api/environment/v1/environmentv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type EnvRPC struct {
	DB *sql.DB

	es senv.EnvService
	vs svar.VarService
	us suser.UserService

	handler EnvironmentHandler
}

func New(db *sql.DB, es senv.EnvService, vs svar.VarService, us suser.UserService) *EnvRPC {
	srv := &EnvRPC{
		DB: db,
		es: es,
		vs: vs,
		us: us,
	}

	srv.handler = NewEnvironmentHandler(srv, srv.environmentPrincipal)
	return srv
}

func CreateService(srv *EnvRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := environmentv1connect.NewEnvironmentServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// EnvironmentCollection returns all environments the user has access to
func (e *EnvRPC) EnvironmentCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.EnvironmentCollectionResponse], error) {
	return e.handler.EnvironmentCollection(ctx, req)
}

// EnvironmentCreate creates a new environment
func (e *EnvRPC) EnvironmentCreate(ctx context.Context, req *connect.Request[apiv1.EnvironmentCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	return e.handler.EnvironmentCreate(ctx, req)
}

// EnvironmentUpdate updates an existing environment
func (e *EnvRPC) EnvironmentUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	return e.handler.EnvironmentUpdate(ctx, req)
}

// EnvironmentDelete deletes an environment
func (e *EnvRPC) EnvironmentDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	return e.handler.EnvironmentDelete(ctx, req)
}

func (e *EnvRPC) environmentPrincipal(ctx context.Context) (EnvironmentPrincipal, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return EnvironmentPrincipal{}, nil
	}

	return EnvironmentPrincipal{UserID: userID}, nil
}

func (e *EnvRPC) OnEnvironmentCollection(ctx context.Context, _ EnvironmentPrincipal) (*apiv1.EnvironmentCollectionResponse, error) {
	// For now, return empty collection as this is a proof of concept
	// In a full implementation, we would:
	// 1. Get user ID from context
	// 2. Get user's workspaces using sworkspace.GetWorkspacesByUserIDOrdered
	// 3. Get all environments from those workspaces
	// 4. Convert to TanStack DB format with proper ordering

	return &apiv1.EnvironmentCollectionResponse{
		Items: []*apiv1.Environment{},
	}, nil
}

func (e *EnvRPC) OnEnvironmentCreate(ctx context.Context, _ EnvironmentPrincipal, items []EnvironmentCreateInput) error {
	for _, input := range items {
		envReq := menv.Env{
			ID:          idwrap.NewNow(),
			WorkspaceID: input.WorkspaceID,
			Type:        menv.EnvNormal,
			Description: input.Description,
			Name:        input.Name,
			Updated:     time.Now(),
		}

		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, e.us, envReq.WorkspaceID))
		if rpcErr != nil {
			return rpcErr
		}

		if err := e.es.Create(ctx, envReq); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	return nil
}

func (e *EnvRPC) OnEnvironmentUpdate(ctx context.Context, _ EnvironmentPrincipal, items []EnvironmentUpdateInput) error {
	for _, input := range items {
		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, input.EnvironmentID))
		if rpcErr != nil {
			return rpcErr
		}

		env, err := e.es.Get(ctx, input.EnvironmentID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if input.Name != nil {
			env.Name = *input.Name
		}
		if input.Description != nil {
			env.Description = *input.Description
		}

		if err := e.es.Update(ctx, env); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	return nil
}

func (e *EnvRPC) OnEnvironmentDelete(ctx context.Context, _ EnvironmentPrincipal, items []EnvironmentDeleteInput) error {
	for _, input := range items {
		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, input.EnvironmentID))
		if rpcErr != nil {
			return rpcErr
		}

		if err := e.es.Delete(ctx, input.EnvironmentID); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
	}

	return nil
}

// EnvironmentSync handles real-time synchronization for environments
func (e *EnvRPC) EnvironmentSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.EnvironmentSyncResponse]) error {
	// For now, this is a placeholder for real-time sync
	// In a full implementation, this would:
	// 1. Subscribe to database changes for environments the user has access to
	// 2. Stream changes to the client
	// 3. Handle connection cleanup

	return nil
}

// EnvironmentVariableCollection returns all environment variables for environments the user has access to
func (e *EnvRPC) EnvironmentVariableCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.EnvironmentVariableCollectionResponse], error) {
	// Placeholder implementation
	return connect.NewResponse(&apiv1.EnvironmentVariableCollectionResponse{
		Items: []*apiv1.EnvironmentVariable{},
	}), nil
}

// EnvironmentVariableCreate creates new environment variables
func (e *EnvRPC) EnvironmentVariableCreate(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	// Placeholder implementation
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableUpdate updates existing environment variables
func (e *EnvRPC) EnvironmentVariableUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	// Placeholder implementation
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableDelete deletes environment variables
func (e *EnvRPC) EnvironmentVariableDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	// Placeholder implementation
	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableSync handles real-time synchronization for environment variables
func (e *EnvRPC) EnvironmentVariableSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.EnvironmentVariableSyncResponse]) error {
	// Placeholder implementation
	return nil
}

// Helper function to check environment ownership
func CheckOwnerEnv(ctx context.Context, su suser.UserService, es senv.EnvService, envid idwrap.IDWrap) (bool, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return false, err
	}
	env, err := es.Get(ctx, envid)
	if err != nil {
		return false, err
	}
	return su.CheckUserBelongsToWorkspace(ctx, userID, env.WorkspaceID)
}
