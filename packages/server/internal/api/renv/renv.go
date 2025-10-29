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
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/service/senv"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
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
	ws sworkspace.WorkspaceService
}

func New(db *sql.DB, es senv.EnvService, vs svar.VarService, us suser.UserService, ws sworkspace.WorkspaceService) EnvRPC {
	return EnvRPC{
		DB: db,
		es: es,
		vs: vs,
		us: us,
		ws: ws,
	}
}

func CreateService(srv EnvRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := environmentv1connect.NewEnvironmentServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func toAPIEnvironment(env menv.Env) *apiv1.Environment {
	return &apiv1.Environment{
		EnvironmentId: env.ID.Bytes(),
		WorkspaceId:   env.WorkspaceID.Bytes(),
		Name:          env.Name,
		Description:   env.Description,
		IsGlobal:      env.Type == menv.EnvGlobal,
		Order:         float32(env.Order),
	}
}

func toAPIEnvironmentVariable(v mvar.Var) *apiv1.EnvironmentVariable {
	return &apiv1.EnvironmentVariable{
		EnvironmentVariableId: v.ID.Bytes(),
		EnvironmentId:         v.EnvID.Bytes(),
		Key:                   v.VarKey,
		Enabled:               v.Enabled,
		Value:                 v.Value,
		Description:           v.Description,
		Order:                 float32(v.Order),
	}
}

func (e *EnvRPC) listUserEnvironments(ctx context.Context) ([]menv.Env, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, err
	}

	workspaces, err := e.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrNoWorkspaceFound) {
			return []menv.Env{}, nil
		}
		return nil, err
	}

	var environments []menv.Env
	for _, workspace := range workspaces {
		envs, err := e.es.ListEnvironments(ctx, workspace.ID)
		if err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				continue
			}
			return nil, err
		}
		environments = append(environments, envs...)
	}
	return environments, nil
}

// EnvironmentCollection returns all environments the user has access to
func (e *EnvRPC) EnvironmentCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.EnvironmentCollectionResponse], error) {
	environments, err := e.listUserEnvironments(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*apiv1.Environment, 0, len(environments))
	for _, env := range environments {
		items = append(items, toAPIEnvironment(env))
	}

	return connect.NewResponse(&apiv1.EnvironmentCollectionResponse{Items: items}), nil
}

// EnvironmentCreate creates a new environment
func (e *EnvRPC) EnvironmentCreate(ctx context.Context, req *connect.Request[apiv1.EnvironmentCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	envService := e.es.TX(tx)

	for _, envCreate := range req.Msg.Items {
		workspaceID, err := idwrap.NewFromBytes(envCreate.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, e.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		var envID idwrap.IDWrap
		if len(envCreate.EnvironmentId) > 0 {
			envID, err = idwrap.NewFromBytes(envCreate.EnvironmentId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		} else {
			envID = idwrap.NewNow()
		}

		envReq := menv.Env{
			ID:          envID,
			WorkspaceID: workspaceID,
			Type:        menv.EnvNormal,
			Description: envCreate.Description,
			Name:        envCreate.Name,
			Order:       float64(envCreate.Order),
		}

		if err := envService.CreateEnvironment(ctx, &envReq); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentUpdate updates an existing environment
func (e *EnvRPC) EnvironmentUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	envService := e.es.TX(tx)

	for _, envUpdate := range req.Msg.Items {
		envID, err := idwrap.NewFromBytes(envUpdate.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, envService, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		env, err := envService.GetEnvironment(ctx, envID)
		if err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if envUpdate.WorkspaceId != nil && len(envUpdate.WorkspaceId) > 0 {
			newWorkspaceID, err := idwrap.NewFromBytes(envUpdate.WorkspaceId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			if newWorkspaceID.Compare(env.WorkspaceID) != 0 {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("moving environments across workspaces is not supported"))
			}
		}

		if envUpdate.Name != nil {
			env.Name = *envUpdate.Name
		}
		if envUpdate.Description != nil {
			env.Description = *envUpdate.Description
		}
		if envUpdate.IsGlobal != nil {
			if *envUpdate.IsGlobal {
				env.Type = menv.EnvGlobal
			} else {
				env.Type = menv.EnvNormal
			}
		}
		if envUpdate.Order != nil {
			env.Order = float64(*envUpdate.Order)
		}

		if err := envService.UpdateEnvironment(ctx, env); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentDelete deletes an environment
func (e *EnvRPC) EnvironmentDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	envService := e.es.TX(tx)

	for _, envDelete := range req.Msg.Items {
		envID, err := idwrap.NewFromBytes(envDelete.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, envService, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		if err := envService.DeleteEnvironment(ctx, envID); err != nil {
			if errors.Is(err, senv.ErrNoEnvironmentFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentSync handles real-time synchronization for environments
func (e *EnvRPC) EnvironmentSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.EnvironmentSyncResponse]) error {
	// Not yet implemented for TanStack DB
	return nil
}

// EnvironmentVariableCollection returns all environment variables for environments the user has access to
func (e *EnvRPC) EnvironmentVariableCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[apiv1.EnvironmentVariableCollectionResponse], error) {
	environments, err := e.listUserEnvironments(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var items []*apiv1.EnvironmentVariable
	for _, env := range environments {
		vars, err := e.vs.GetVariableByEnvID(ctx, env.ID)
		if err != nil {
			if errors.Is(err, svar.ErrNoVarFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, v := range vars {
			items = append(items, toAPIEnvironmentVariable(v))
		}
	}

	return connect.NewResponse(&apiv1.EnvironmentVariableCollectionResponse{Items: items}), nil
}

// EnvironmentVariableCreate creates new environment variables
func (e *EnvRPC) EnvironmentVariableCreate(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment variable must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	varService := e.vs.TX(tx)

	for _, item := range req.Msg.Items {
		envID, err := idwrap.NewFromBytes(item.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		varID := idwrap.NewNow()
		if len(item.EnvironmentVariableId) > 0 {
			varID, err = idwrap.NewFromBytes(item.EnvironmentVariableId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
		}

		varReq := mvar.Var{
			ID:          varID,
			EnvID:       envID,
			VarKey:      item.Key,
			Value:       item.Value,
			Enabled:     item.Enabled,
			Description: item.Description,
			Order:       float64(item.Order),
		}

		if err := varService.Create(ctx, varReq); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableUpdate updates existing environment variables
func (e *EnvRPC) EnvironmentVariableUpdate(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment variable must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	varService := e.vs.TX(tx)

	for _, item := range req.Msg.Items {
		varID, err := idwrap.NewFromBytes(item.EnvironmentVariableId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, e.us, varService, e.es, varID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		variable, err := varService.Get(ctx, varID)
		if err != nil {
			if errors.Is(err, svar.ErrNoVarFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.EnvironmentId != nil && len(item.EnvironmentId) > 0 {
			newEnvID, err := idwrap.NewFromBytes(item.EnvironmentId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}

			rpcErr := permcheck.CheckPerm(CheckOwnerEnv(ctx, e.us, e.es, newEnvID))
			if rpcErr != nil {
				return nil, rpcErr
			}

			variable.EnvID = newEnvID
		}

		if item.Key != nil {
			variable.VarKey = *item.Key
		}
		if item.Value != nil {
			variable.Value = *item.Value
		}
		if item.Enabled != nil {
			variable.Enabled = *item.Enabled
		}
		if item.Description != nil {
			variable.Description = *item.Description
		}
		if item.Order != nil {
			variable.Order = float64(*item.Order)
		}

		if err := varService.Update(ctx, variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableDelete deletes environment variables
func (e *EnvRPC) EnvironmentVariableDelete(ctx context.Context, req *connect.Request[apiv1.EnvironmentVariableDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.Items) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one environment variable must be provided"))
	}

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	varService := e.vs.TX(tx)

	for _, item := range req.Msg.Items {
		varID, err := idwrap.NewFromBytes(item.EnvironmentVariableId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}

		rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, e.us, varService, e.es, varID))
		if rpcErr != nil {
			return nil, rpcErr
		}

		if err := varService.Delete(ctx, varID); err != nil {
			if errors.Is(err, svar.ErrNoVarFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

// EnvironmentVariableSync handles real-time synchronization for environment variables
func (e *EnvRPC) EnvironmentVariableSync(ctx context.Context, req *connect.Request[emptypb.Empty], stream *connect.ServerStream[apiv1.EnvironmentVariableSyncResponse]) error {
	// Not yet implemented for TanStack DB
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

// Helper function to check environment variable ownership
func CheckOwnerVar(ctx context.Context, su suser.UserService, vs svar.VarService, es senv.EnvService, varID idwrap.IDWrap) (bool, error) {
	variable, err := vs.Get(ctx, varID)
	if err != nil {
		return false, err
	}
	return CheckOwnerEnv(ctx, su, es, variable.EnvID)
}
