package renv

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/rworkspace"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/menv"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/senv"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/svar"
	"dev-tools-backend/pkg/translate/tenv"
	"dev-tools-backend/pkg/translate/tgeneric"
	environmentv1 "dev-tools-services/gen/environment/v1"
	"dev-tools-services/gen/environment/v1/environmentv1connect"

	"connectrpc.com/connect"
)

type EnvRPC struct {
	DB *sql.DB

	es senv.EnvService
	vs svar.VarService
	us suser.UserService
}

func New(db *sql.DB, es senv.EnvService, vs svar.VarService, us suser.UserService) *EnvRPC {
	return &EnvRPC{
		DB: db,
		es: es,
		vs: vs,
		us: us,
	}
}

func CreateService(ctx context.Context, srv EnvRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := environmentv1connect.NewEnvironmentServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (e *EnvRPC) CreateEnvironment(ctx context.Context, req *connect.Request[environmentv1.CreateEnvironmentRequest]) (*connect.Response[environmentv1.CreateEnvironmentResponse], error) {
	workspaceID, err := idwrap.NewWithParse(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	envReq := tenv.DeseralizeRPCToModelWithID(idwrap.NewNow(), req.Msg.GetEnvironment())
	envReq.WorkspaceID = workspaceID
	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, e.us, envReq.WorkspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = e.es.Create(ctx, envReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&environmentv1.CreateEnvironmentResponse{Id: envReq.ID.String()}), nil
}

func (e *EnvRPC) GetEnvironment(ctx context.Context, req *connect.Request[environmentv1.GetEnvironmentRequest]) (*connect.Response[environmentv1.GetEnvironmentResponse], error) {
	id, err := idwrap.NewWithParse(req.Msg.Id)
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
	return connect.NewResponse(&environmentv1.GetEnvironmentResponse{Environment: resp}), nil
}

func (e *EnvRPC) GetEnvironments(ctx context.Context, req *connect.Request[environmentv1.GetEnvironmentsRequest]) (*connect.Response[environmentv1.GetEnvironmentsResponse], error) {
	workspaceID, err := idwrap.NewWithParse(req.Msg.WorkspaceId)
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

	resp := tgeneric.MassConvert(envs, tenv.SeralizeModelToRPC)
	return connect.NewResponse(&environmentv1.GetEnvironmentsResponse{Environments: resp}), nil
}

func (e *EnvRPC) UpdateEnvironment(ctx context.Context, req *connect.Request[environmentv1.UpdateEnvironmentRequest]) (*connect.Response[environmentv1.UpdateEnvironmentResponse], error) {
	envReq, err := tenv.DeserializeRPCToModel(req.Msg.GetEnvironment())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, envReq.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = e.es.Update(ctx, &envReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&environmentv1.UpdateEnvironmentResponse{}), nil
}

func (e *EnvRPC) DeleteEnvironment(ctx context.Context, req *connect.Request[environmentv1.DeleteEnvironmentRequest]) (*connect.Response[environmentv1.DeleteEnvironmentResponse], error) {
	id, err := idwrap.NewWithParse(req.Msg.Id)
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
	return connect.NewResponse(&environmentv1.DeleteEnvironmentResponse{}), nil
}

func (e *EnvRPC) GetAllVariables(ctx context.Context, req *connect.Request[environmentv1.GetAllVariablesRequest]) (*connect.Response[environmentv1.GetAllVariablesResponse], error) {
	workspaceID, err := idwrap.NewWithParse(req.Msg.WorkspaceId)
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
	varMap := make(map[string][]menv.Env)
	for _, env := range envs {
		vars, err := e.vs.GetVariableByEnvID(ctx, env.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, v := range vars {
			mapVars, ok := varMap[v.VarKey]
			if !ok {
				varMap[v.VarKey] = []menv.Env{env}
			} else {
				varMap[v.VarKey] = append(mapVars, env)
			}
		}
	}
	var envsRPC []*environmentv1.VariableWithEnvironments
	for key, envs := range varMap {
		envsRPC = append(envsRPC, tenv.SeralizeModelToGroupRPC(key, envs))
	}
	return connect.NewResponse(&environmentv1.GetAllVariablesResponse{Items: envsRPC}), nil
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
