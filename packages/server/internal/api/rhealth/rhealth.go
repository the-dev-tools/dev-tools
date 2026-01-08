//nolint:revive // exported
package rhealth

import (
	"context"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/health/v1/healthv1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type HealthServiceRPC struct{}

func New() *HealthServiceRPC {
	return &HealthServiceRPC{}
}

func CreateService(srv *HealthServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := healthv1connect.NewHealthServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *HealthServiceRPC) HealthCheck(ctx context.Context, _ *connect.Request[emptypb.Empty]) (*connect.Response[emptypb.Empty], error) {
	return connect.NewResponse(&emptypb.Empty{}), nil
}
