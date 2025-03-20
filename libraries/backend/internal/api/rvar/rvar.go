package rvar

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"sort"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/renv"
	"the-dev-tools/backend/internal/api/rworkspace"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mvar"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/senv"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/svar"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/backend/pkg/translate/tvar"
	variablev1 "the-dev-tools/spec/dist/buf/go/variable/v1"
	"the-dev-tools/spec/dist/buf/go/variable/v1/variablev1connect"

	"connectrpc.com/connect"
)

type VarRPC struct {
	DB *sql.DB

	us suser.UserService

	es senv.EnvService
	vs svar.VarService
}

func New(db *sql.DB, us suser.UserService, es senv.EnvService, vs svar.VarService) VarRPC {
	return VarRPC{
		DB: db,
		us: us,
		es: es,
		vs: vs,
	}
}

func CreateService(srv VarRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := variablev1connect.NewVariableServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (v *VarRPC) VariableList(ctx context.Context, req *connect.Request[variablev1.VariableListRequest]) (*connect.Response[variablev1.VariableListResponse], error) {
	envIDRaw, workspaceIDRaw := req.Msg.GetEnvironmentId(), req.Msg.GetWorkspaceId()
	if len(envIDRaw) != 0 {
		envID, err := idwrap.NewFromBytes(req.Msg.EnvironmentId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		rpcErr := permcheck.CheckPerm(renv.CheckOwnerEnv(ctx, v.us, v.es, envID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		vars, err := v.vs.GetVariableByEnvID(ctx, envID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		sort.Slice(vars, func(i, j int) bool {
			return vars[i].ID.Compare(vars[j].ID) < 0
		})

		rpcVars := tgeneric.MassConvert(vars, tvar.SerializeModelToRPCItem)
		return connect.NewResponse(&variablev1.VariableListResponse{Items: rpcVars}), nil

	} else if len(workspaceIDRaw) != 0 {
		workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, v.us, workspaceID))
		if rpcErr != nil {
			return nil, rpcErr
		}
		envs, err := v.es.GetByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		var vars []mvar.Var
		for _, env := range envs {
			envVars, err := v.vs.GetVariableByEnvID(ctx, env.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			vars = append(vars, envVars...)
		}

		sort.Slice(vars, func(i, j int) bool {
			return bytes.Compare(vars[i].EnvID.Bytes(), vars[j].EnvID.Bytes()) < 0
		})

		rpcVars := tgeneric.MassConvert(vars, tvar.SerializeModelToRPCItem)

		return connect.NewResponse(&variablev1.VariableListResponse{Items: rpcVars}), nil
	}
	return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("workspace id or env ID is required"))
}

func (v *VarRPC) VariableGet(ctx context.Context, req *connect.Request[variablev1.VariableGetRequest]) (*connect.Response[variablev1.VariableGetResponse], error) {
	id, err := idwrap.NewFromBytes(req.Msg.VariableId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, v.us, v.vs, v.es, id))
	if rpcErr != nil {
		return nil, rpcErr
	}
	varible, err := v.vs.Get(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcVar := tvar.SerializeModelToRPC(*varible)
	rpcRawResp := &variablev1.VariableGetResponse{
		VariableId:  rpcVar.VariableId,
		Name:        rpcVar.Name,
		Value:       rpcVar.Value,
		Enabled:     rpcVar.Enabled,
		Description: rpcVar.Description,
	}
	return connect.NewResponse(rpcRawResp), nil
}

func (v *VarRPC) VariableCreate(ctx context.Context, req *connect.Request[variablev1.VariableCreateRequest]) (*connect.Response[variablev1.VariableCreateResponse], error) {
	envID, err := idwrap.NewFromBytes(req.Msg.GetEnvironmentId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(renv.CheckOwnerEnv(ctx, v.us, v.es, envID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	rpcVar := variablev1.Variable{
		Name:        req.Msg.Name,
		Value:       req.Msg.Value,
		Enabled:     req.Msg.Enabled,
		Description: req.Msg.Description,
	}

	varReq := tvar.DeserializeRPCToModelWithID(idwrap.NewNow(), envID, &rpcVar)
	err = v.vs.Create(ctx, varReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	varReq.EnvID = envID

	return connect.NewResponse(&variablev1.VariableCreateResponse{VariableId: varReq.ID.Bytes()}), nil
}

func (c *VarRPC) VariableUpdate(ctx context.Context, req *connect.Request[variablev1.VariableUpdateRequest]) (*connect.Response[variablev1.VariableUpdateResponse], error) {
	msg := req.Msg

	var name string
	var value string
	var enabled bool
	var description string
	if msg.Name != nil {
		name = *msg.Name
	}
	if msg.Value != nil {
		value = *msg.Value
	}
	if msg.Enabled != nil {
		enabled = *msg.Enabled
	}
	if msg.Description != nil {
		description = *msg.Description
	}

	varConverted := &variablev1.Variable{
		VariableId:  msg.GetVariableId(),
		Name:        name,
		Value:       value,
		Enabled:     enabled,
		Description: description,
	}
	varReq, err := tvar.DeserializeRPCToModel(varConverted)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, c.us, c.vs, c.es, varReq.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.vs.Update(ctx, &varReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.VariableUpdateResponse{}), nil
}

func (c *VarRPC) VariableDelete(ctx context.Context, req *connect.Request[variablev1.VariableDeleteRequest]) (*connect.Response[variablev1.VariableDeleteResponse], error) {
	id, err := idwrap.NewFromBytes(req.Msg.GetVariableId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerVar(ctx, c.us, c.vs, c.es, id))
	if rpcErr != nil {
		return nil, rpcErr
	}
	err = c.vs.Delete(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.VariableDeleteResponse{}), nil
}

func CheckOwnerVar(ctx context.Context, us suser.UserService, vs svar.VarService, es senv.EnvService, varID idwrap.IDWrap) (bool, error) {
	variable, err := vs.Get(ctx, varID)
	if err != nil {
		return false, err
	}
	return renv.CheckOwnerEnv(ctx, us, es, variable.EnvID)
}
