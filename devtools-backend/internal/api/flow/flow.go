package flow

import (
	"context"
	"database/sql"
	"devtools-backend/internal/api"
	"devtools-backend/pkg/stoken"
	flowv1 "devtools-services/gen/flow/v1"
	"devtools-services/gen/flow/v1/flowv1connect"
	"errors"

	"connectrpc.com/connect"
)

// TODO: Move to a common package.
const tokenHeaderKey = "token"

func (c FlowServer) NewAuthInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			tokenTemp := req.Header().Get(tokenHeaderKey)
			if tokenTemp == "" {
				// Check token in handlers.
				return nil, connect.NewError(
					connect.CodeUnauthenticated,
					errors.New("no token provided"),
				)
			}

			stoken.ValidateJWT(tokenTemp, c.secret)

			return next(ctx, req)
		})
	}
	return connect.UnaryInterceptorFunc(interceptor)
}

type FlowServer struct {
	db     *sql.DB
	secret []byte
}

func (c FlowServer) Create(ctx context.Context, req *connect.Request[flowv1.FlowServiceCreateRequest]) (*connect.Response[flowv1.FlowServiceCreateResponse], error) {
	return nil, nil
}

func (c FlowServer) Save(ctx context.Context, req *connect.Request[flowv1.FlowServiceSaveRequest]) (*connect.Response[flowv1.FlowServiceSaveResponse], error) {
	return nil, nil
}

func (c FlowServer) Load(ctx context.Context, req *connect.Request[flowv1.FlowServiceLoadRequest]) (*connect.Response[flowv1.FlowServiceLoadResponse], error) {
	return nil, nil
}

func (c FlowServer) Delete(ctx context.Context, req *connect.Request[flowv1.FlowServiceDeleteRequest]) (*connect.Response[flowv1.FlowServiceDeleteResponse], error) {
	return nil, nil
}

func (c FlowServer) AddPostmanCollection(ctx context.Context, req *connect.Request[flowv1.FlowServiceAddPostmanCollectionRequest]) (*connect.Response[flowv1.FlowServiceAddPostmanCollectionResponse], error) {
	return nil, nil
}

func CreateService(secret []byte) (*api.Service, error) {
	server := &FlowServer{
		secret: secret,
	}
	path, handler := flowv1connect.NewFlowServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
