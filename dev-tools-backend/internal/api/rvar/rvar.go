package rvar

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/renv"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/permcheck"
	"dev-tools-backend/pkg/service/senv"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/svar"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/tvar"
	variablev1 "dev-tools-spec/dist/buf/go/variable/v1"
	"dev-tools-spec/dist/buf/go/variable/v1/variablev1connect"
	"sort"

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
	envID, err := idwrap.NewFromBytes(req.Msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(renv.CheckOwnerEnv(ctx, v.us, v.es, envID))
	if rpcErr != nil {
		return nil, rpcErr
	}
	variables, err := v.vs.GetVariableByEnvID(ctx, envID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	sort.Slice(variables, func(i, j int) bool {
		return variables[i].ID.Compare(variables[j].ID) < 0
	})
	rpcVars := tgeneric.MassConvert(variables, tvar.SerializeModelToRPCItem)
	return connect.NewResponse(&variablev1.VariableListResponse{Items: rpcVars}), nil
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
	varConverted := &variablev1.Variable{
		VariableId:  req.Msg.GetVariableId(),
		Name:        req.Msg.Name,
		Value:       req.Msg.Value,
		Enabled:     req.Msg.Enabled,
		Description: req.Msg.Description,
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
