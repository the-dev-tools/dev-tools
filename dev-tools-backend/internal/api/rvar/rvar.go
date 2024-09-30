package rvar

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/service/svar"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-backend/pkg/translate/tvar"
	variablev1 "dev-tools-services/gen/variable/v1"
	"dev-tools-services/gen/variable/v1/variablev1connect"

	"connectrpc.com/connect"
)

type VarRPC struct {
	DB *sql.DB

	vs svar.VarService
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	var options []connect.HandlerOption

	vs, err := svar.New(ctx, db)
	if err != nil {
		return nil, err
	}

	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	service := &VarRPC{
		DB: db,

		vs: vs,
	}

	path, handler := variablev1connect.NewVariableServiceHandler(service, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// TODO: add perm checks
func (v *VarRPC) CreateVariable(ctx context.Context, req *connect.Request[variablev1.CreateVariableRequest]) (*connect.Response[variablev1.CreateVariableResponse], error) {
	envID, err := idwrap.NewWithParse(req.Msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	varReq := tvar.DeserializeRPCToModelWithID(idwrap.NewNow(), req.Msg.GetVariable())
	err = v.vs.Create(ctx, varReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	varReq.EnvID = envID

	return connect.NewResponse(&variablev1.CreateVariableResponse{Id: varReq.ID.String()}), nil
}

// TODO: add perm checks
func (v *VarRPC) GetVariable(ctx context.Context, req *connect.Request[variablev1.GetVariableRequest]) (*connect.Response[variablev1.GetVariableResponse], error) {
	id, err := idwrap.NewWithParse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	varible, err := v.vs.Get(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.GetVariableResponse{Variable: tvar.SerializeModelToRPC(*varible)}), nil
}

// TODO: add perm checks
func (v *VarRPC) GetVariables(ctx context.Context, req *connect.Request[variablev1.GetVariablesRequest]) (*connect.Response[variablev1.GetVariablesResponse], error) {
	envID, err := idwrap.NewWithParse(req.Msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	variables, err := v.vs.GetVariableByEnvID(ctx, envID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcVars := tgeneric.MassConvert(variables, tvar.SerializeModelToRPC)
	return connect.NewResponse(&variablev1.GetVariablesResponse{Variables: rpcVars}), nil
}

// TODO: add perm checks
func (c *VarRPC) UpdateVariable(ctx context.Context, req *connect.Request[variablev1.UpdateVariableRequest]) (*connect.Response[variablev1.UpdateVariableResponse], error) {
	varReq, err := tvar.DeserializeRPCToModel(req.Msg.GetVariable())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	err = c.vs.Update(ctx, &varReq)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.UpdateVariableResponse{}), nil
}

// TODO: add perm checks
func (c *VarRPC) DeleteVariable(ctx context.Context, req *connect.Request[variablev1.DeleteVariableRequest]) (*connect.Response[variablev1.DeleteVariableResponse], error) {
	id, err := idwrap.NewWithParse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	err = c.vs.Delete(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&variablev1.DeleteVariableResponse{}), nil
}
