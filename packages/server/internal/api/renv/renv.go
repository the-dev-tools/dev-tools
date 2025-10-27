package renv

import (
	"context"
	"database/sql"
	"errors"
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
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type EnvRPC struct {
	DB *sql.DB

	es senv.EnvService
	vs svar.VarService
	us suser.UserService
}

func New(db *sql.DB, es senv.EnvService, vs svar.VarService, us suser.UserService) EnvRPC {
	return EnvRPC{
		DB: db,
		es: es,
		vs: vs,
		us: us,
	}
}

func CreateService(srv EnvRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := environmentv1connect.NewEnvironmentServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// EnvironmentCollection returns all environments the user has access to
func (e *EnvRPC) EnvironmentCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.EnvironmentCollectionResponse], error) {
	// For now, return empty collection as this is a proof of concept
	// In a full implementation, we would:
	// 1. Get user ID from context
	// 2. Get user's workspaces using sworkspace.GetWorkspacesByUserIDOrdered
	// 3. Get all environments from those workspaces
	// 4. Convert to TanStack DB format with proper ordering

	return connect.NewResponse(&apiv1.EnvironmentCollectionResponse{
		Items: []*apiv1.Environment{},
	}), nil
}

// EnvironmentCreate creates a new environment
func (e *EnvRPC) EnvironmentCreate(ctx context.Context, req *connect.Request[apiv1.EnvironmentCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	// Process first environment for now (TanStack DB supports batch creation)
	envCreate := req.Msg.Items[0]
	workspaceID, err := idwrap.NewFromBytes(envCreate.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	envReq := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal, // Default to normal environment
		Description: envCreate.Description,
		Name:        envCreate.Name,
		Updated:     time.Now(),
	}

	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, e.us, envReq.WorkspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = e.es.Create(ctx, envReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentUpdate updates an existing environment
func (e *EnvRPC) EnvironmentUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	// Process first environment for now
	envUpdate := req.Msg.Items[0]
	envID, err := idwrap.NewFromBytes(envUpdate.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions
	rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, envID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Get existing environment
	env, err := e.es.Get(ctx, envID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Update fields if provided
	if envUpdate.Name != nil {
		env.Name = *envUpdate.Name
	}
	if envUpdate.Description != nil {
		env.Description = *envUpdate.Description
	}

	err = e.es.Update(ctx, env)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentDelete deletes an environment
func (e *EnvRPC) EnvironmentDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	// Process first environment for now
	envDelete := req.Msg.Items[0]
	envID, err := idwrap.NewFromBytes(envDelete.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, envID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = e.es.Delete(ctx, envID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
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
