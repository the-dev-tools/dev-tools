package renv

import (
	"context"
	"database/sql"
	"dev-tools-backend/internal/api"
	"dev-tools-backend/internal/api/middleware/mwauth"
	"dev-tools-backend/internal/api/middleware/mwcompress"
	environmentv1 "dev-tools-services/gen/environment/v1"
	"dev-tools-services/gen/environment/v1/environmentv1connect"
	"errors"

	"connectrpc.com/connect"
)

type EnvRPC struct {
	DB *sql.DB
}

func CreateService(ctx context.Context, db *sql.DB, secret []byte) (*api.Service, error) {
	var options []connect.HandlerOption

	options = append(options, connect.WithCompression("zstd", mwcompress.NewDecompress, mwcompress.NewCompress))
	options = append(options, connect.WithCompression("gzip", nil, nil))
	options = append(options, connect.WithInterceptors(mwauth.NewAuthInterceptor(secret)))
	service := &EnvRPC{
		DB: db,
		// root
	}

	path, handler := environmentv1connect.NewEnvironmentServiceHandler(service, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// CreateEnvironment calls environment.environmentv1.EnvironmentService.CreateEnvironment.
func (e *EnvRPC) CreateEnvironment(ctx context.Context, req *connect.Request[environmentv1.CreateEnvironmentRequest]) (*connect.Response[environmentv1.CreateEnvironmentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

// GetEnvironment calls environment.environmentv1.EnvironmentService.GetEnvironment.
func (e *EnvRPC) GetEnvironment(ctx context.Context, req *connect.Request[environmentv1.GetEnvironmentRequest]) (*connect.Response[environmentv1.GetEnvironmentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

// GetEnvironments calls environment.environmentv1.EnvironmentService.GetEnvironments.
func (e *EnvRPC) GetEnvironments(ctx context.Context, req *connect.Request[environmentv1.GetEnvironmentsRequest]) (*connect.Response[environmentv1.GetEnvironmentsResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

// UpdateEnvironment calls environment.environmentv1.EnvironmentService.UpdateEnvironment.
func (e *EnvRPC) UpdateEnvironment(ctx context.Context, req *connect.Request[environmentv1.UpdateEnvironmentRequest]) (*connect.Response[environmentv1.UpdateEnvironmentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}

// DeleteEnvironment calls environment.environmentv1.EnvironmentService.DeleteEnvironment.
func (e *EnvRPC) DeleteEnvironment(ctx context.Context, req *connect.Request[environmentv1.DeleteEnvironmentRequest]) (*connect.Response[environmentv1.DeleteEnvironmentResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("unimplemented"))
}
