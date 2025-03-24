package renv

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/middleware/mwauth"
	"the-dev-tools/backend/internal/api/rworkspace"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/menv"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/senv"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/svar"
	"the-dev-tools/backend/pkg/translate/tenv"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	environmentv1 "the-dev-tools/spec/dist/buf/go/environment/v1"
	"the-dev-tools/spec/dist/buf/go/environment/v1/environmentv1connect"
	"time"

	"connectrpc.com/connect"
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
