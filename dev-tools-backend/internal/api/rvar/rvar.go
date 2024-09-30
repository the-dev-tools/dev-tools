package rvar

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/internal/api/renv"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/service/senv"
	"dev-tools-backend/pkg/service/suser"
	"dev-tools-backend/pkg/service/svar"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/tvar"
	variablev1 "dev-tools-services/gen/variable/v1"
	"dev-tools-services/gen/variable/v1/variablev1connect"

	"connectrpc.com/connect"
)

type VarRPC struct {
	DB *sql.DB

	us suser.UserService

	es senv.EnvService
	vs svar.VarService
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	var options []connect.HandlerOption

	us, err := suser.New(ctx, db)
	if err != nil {
		return nil, err
	}

	es, err := senv.New(ctx, db)
	if err != nil {
		return nil, err
	}

	vs, err := svar.New(ctx, db)
	if err != nil {
		return nil, err
	}

	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	service := &VarRPC{
		DB: db,

		us: *us,

		es: es,

		vs: vs,
	}

	path, handler := variablev1connect.NewVariableServiceHandler(service, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (v *VarRPC) CreateVariable(ctx context.Context, req *connect.Request[variablev1.CreateVariableRequest]) (*connect.Response[variablev1.CreateVariableResponse], error) {
	envID, err := idwrap.NewWithParse(req.Msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := renv.CheckOwnerEnv(ctx, v.us, v.es, envID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	varReq := tvar.DeserializeRPCToModelWithID(idwrap.NewNow(), req.Msg.GetVariable())
	err = v.vs.Create(ctx, varReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	varReq.EnvID = envID

	return connect.NewResponse(&variablev1.CreateVariableResponse{Id: varReq.ID.String()}), nil
}

func (v *VarRPC) GetVariable(ctx context.Context, req *connect.Request[variablev1.GetVariableRequest]) (*connect.Response[variablev1.GetVariableResponse], error) {
	id, err := idwrap.NewWithParse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := CheckOwnerVar(ctx, v.us, v.vs, v.es, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}
	varible, err := v.vs.Get(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.GetVariableResponse{Variable: tvar.SerializeModelToRPC(*varible)}), nil
}

func (v *VarRPC) GetVariables(ctx context.Context, req *connect.Request[variablev1.GetVariablesRequest]) (*connect.Response[variablev1.GetVariablesResponse], error) {
	envID, err := idwrap.NewWithParse(req.Msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := renv.CheckOwnerEnv(ctx, v.us, v.es, envID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}
	variables, err := v.vs.GetVariableByEnvID(ctx, envID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcVars := tgeneric.MassConvert(variables, tvar.SerializeModelToRPC)
	return connect.NewResponse(&variablev1.GetVariablesResponse{Variables: rpcVars}), nil
}

func (c *VarRPC) UpdateVariable(ctx context.Context, req *connect.Request[variablev1.UpdateVariableRequest]) (*connect.Response[variablev1.UpdateVariableResponse], error) {
	varReq, err := tvar.DeserializeRPCToModel(req.Msg.GetVariable())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := CheckOwnerVar(ctx, c.us, c.vs, c.es, varReq.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	err = c.vs.Update(ctx, &varReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.UpdateVariableResponse{}), nil
}

func (c *VarRPC) DeleteVariable(ctx context.Context, req *connect.Request[variablev1.DeleteVariableRequest]) (*connect.Response[variablev1.DeleteVariableResponse], error) {
	id, err := idwrap.NewWithParse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	ok, err := CheckOwnerVar(ctx, c.us, c.vs, c.es, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !ok {
		return nil, connect.NewError(connect.CodePermissionDenied, nil)
	}

	err = c.vs.Delete(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.DeleteVariableResponse{}), nil
}

func CheckOwnerVar(ctx context.Context, us suser.UserService, vs svar.VarService, es senv.EnvService, varID idwrap.IDWrap) (bool, error) {
	variable, err := vs.Get(ctx, varID)
	if err != nil {
		return false, err
	}
	return renv.CheckOwnerEnv(ctx, us, es, variable.EnvID)
}
