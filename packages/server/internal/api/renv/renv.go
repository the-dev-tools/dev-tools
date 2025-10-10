package renv

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/translate/tenv"
	"the-dev-tools/server/pkg/translate/tgeneric"
	environmentv1 "the-dev-tools/spec/dist/buf/go/environment/v1"
	"the-dev-tools/spec/dist/buf/go/environment/v1/environmentv1connect"
	resourcesv1 "the-dev-tools/spec/dist/buf/go/resource/v1"
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

func (e *EnvRPC) EnvironmentCreate(ctx context.Context, req *connect.Request[environmentv1.EnvironmentCreateRequest]) (*connect.Response[environmentv1.EnvironmentCreateResponse], error) {
	ReqEnv := req.Msg
	workspaceID, err := idwrap.NewFromBytes(ReqEnv.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	envReq := menv.Env{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Type:        menv.EnvNormal,
		Description: ReqEnv.Description,
		Name:        ReqEnv.Name,
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
	envResp := &environmentv1.EnvironmentCreateResponse{
		EnvironmentId: envReq.ID.Bytes(),
	}

	return connect.NewResponse(envResp), nil
}

func (e *EnvRPC) EnvironmentList(ctx context.Context, req *connect.Request[environmentv1.EnvironmentListRequest]) (*connect.Response[environmentv1.EnvironmentListResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, e.us, workspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	envs, err := e.es.GetByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := tgeneric.MassConvert(envs, tenv.SeralizeModelToRPCItem)
	resp := &environmentv1.EnvironmentListResponse{
		WorkspaceId: req.Msg.WorkspaceId,
		Items:       items,
	}

	return connect.NewResponse(resp), nil
}

func (e *EnvRPC) EnvironmentGet(ctx context.Context, req *connect.Request[environmentv1.EnvironmentGetRequest]) (*connect.Response[environmentv1.EnvironmentGetResponse], error) {
	id, err := idwrap.NewFromBytes(req.Msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, id))
	if rpcErr != nil {
		return nil, rpcErr
	}
	ok, err := CheckOwnerEnv(ctx, e.us, e.es, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}
	env, err := e.es.Get(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := tenv.SeralizeModelToRPC(*env)
	respRaw := &environmentv1.EnvironmentGetResponse{
		EnvironmentId: resp.EnvironmentId,
		Name:          resp.Name,
		Description:   resp.Description,
		Updated:       resp.Updated,
		IsGlobal:      resp.IsGlobal,
	}
	return connect.NewResponse(respRaw), nil
}

func (e *EnvRPC) EnvironmentUpdate(ctx context.Context, req *connect.Request[environmentv1.EnvironmentUpdateRequest]) (*connect.Response[environmentv1.EnvironmentUpdateResponse], error) {
	msg := req.Msg
	envID, err := idwrap.NewFromBytes(msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, envID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	if msg.EnvironmentId == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("environment id is required"))
	}
	env, err := e.es.Get(ctx, envID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Name != nil {
		env.Name = *msg.Name
	}
	if msg.Description != nil {
		env.Description = *msg.Description
	}
	err = e.es.Update(ctx, env)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&environmentv1.EnvironmentUpdateResponse{}), nil
}

func (e *EnvRPC) EnvironmentDelete(ctx context.Context, req *connect.Request[environmentv1.EnvironmentDeleteRequest]) (*connect.Response[environmentv1.EnvironmentDeleteResponse], error) {
	id, err := idwrap.NewFromBytes(req.Msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, id))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = e.es.Delete(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&environmentv1.EnvironmentDeleteResponse{}), nil
}

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

func (c *EnvRPC) EnvironmentMove(ctx context.Context, req *connect.Request[environmentv1.EnvironmentMoveRequest]) (*connect.Response[emptypb.Empty], error) {
	// Validate environment ID
	environmentID, err := idwrap.NewFromBytes(req.Msg.GetEnvironmentId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions for the environment being moved
	rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, c.us, c.es, environmentID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Validate workspace ID if provided (for additional permission checking)
	if len(req.Msg.GetWorkspaceId()) > 0 {
		workspaceID, err := idwrap.NewFromBytes(req.Msg.GetWorkspaceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		// Verify environment belongs to the specified workspace
		environmentWorkspaceID, err := c.es.GetWorkspaceID(ctx, environmentID)
		if err != nil {
			if err == senv.ErrNoEnvironmentFound {
				return nil, connect.NewError(connect.CodeNotFound, errors.New("environment not found"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if environmentWorkspaceID.Compare(workspaceID) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("environment does not belong to specified workspace"))
		}
	}

	// Validate target environment ID
	targetEnvironmentID, err := idwrap.NewFromBytes(req.Msg.GetTargetEnvironmentId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// Check permissions for target environment (must be in same workspace)
	rpcErr = permcheck.CheckPerm(CheckOwnerEnv(ctx, c.us, c.es, targetEnvironmentID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	// Validate position
	position := req.Msg.GetPosition()
	if position == resourcesv1.MovePosition_MOVE_POSITION_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("position must be specified"))
	}

	// Prevent moving environment relative to itself
	if environmentID.Compare(targetEnvironmentID) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("cannot move environment relative to itself"))
	}

	// Verify both environments are in the same workspace
	sourceWorkspaceID, err := c.es.GetWorkspaceID(ctx, environmentID)
	if err != nil {
		if err == senv.ErrNoEnvironmentFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("environment not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	targetWorkspaceID, err := c.es.GetWorkspaceID(ctx, targetEnvironmentID)
	if err != nil {
		if err == senv.ErrNoEnvironmentFound {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("target environment not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if sourceWorkspaceID.Compare(targetWorkspaceID) != 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("environments must be in the same workspace"))
	}

	// Add debug logging for move operations
	slog.DebugContext(ctx, "EnvironmentMove request",
		"environment_id", environmentID.String(),
		"target_environment_id", targetEnvironmentID.String(),
		"position", position.String(),
		"workspace_id", sourceWorkspaceID.String())

	// Execute the move operation
	switch position {
	case resourcesv1.MovePosition_MOVE_POSITION_AFTER:
		err = c.es.MoveEnvironmentAfter(ctx, environmentID, targetEnvironmentID)
	case resourcesv1.MovePosition_MOVE_POSITION_BEFORE:
		err = c.es.MoveEnvironmentBefore(ctx, environmentID, targetEnvironmentID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid position"))
	}

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}
