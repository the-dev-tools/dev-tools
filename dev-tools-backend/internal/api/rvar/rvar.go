package rvar

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	variablev1 "dev-tools-services/gen/variable/v1"
	"dev-tools-services/gen/variable/v1/variablev1connect"
	"errors"

	"connectrpc.com/connect"
)

type VarRPC struct {
	DB *sql.DB
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	var options []connect.HandlerOption

	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	service := &VarRPC{
		DB: db,
		// root
	}

	path, handler := variablev1connect.NewVariableServiceHandler(service, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (v *VarRPC) CreateVariable(ctx context.Context, req *connect.Request[variablev1.CreateVariableRequest]) (*connect.Response[variablev1.CreateVariableResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

// GetVariable calls variable.v1.VariableService.GetVariable.
func (v *VarRPC) GetVariable(ctx context.Context, req *connect.Request[variablev1.GetVariableRequest]) (*connect.Response[variablev1.GetVariableResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

// GetVariables calls variable.v1.VariableService.GetVariables.
func (v *VarRPC) GetVariables(ctx context.Context, req *connect.Request[variablev1.GetVariablesRequest]) (*connect.Response[variablev1.GetVariablesResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

// UpdateVariable calls variable.v1.VariableService.UpdateVariable.
func (c *VarRPC) UpdateVariable(ctx context.Context, req *connect.Request[variablev1.UpdateVariableRequest]) (*connect.Response[variablev1.UpdateVariableResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

// DeleteVariable calls variable.v1.VariableService.DeleteVariable.
func (c *VarRPC) DeleteVariable(ctx context.Context, req *connect.Request[variablev1.DeleteVariableRequest]) (*connect.Response[variablev1.DeleteVariableResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}
