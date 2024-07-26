package flow

import (
	"context"
	"devtools-backend/internal/api"
	flowv1 "devtools-services/gen/flow/v1"
	"devtools-services/gen/flow/v1/flowv1connect"
	"errors"
	"os"

	"connectrpc.com/connect"
	"github.com/bufbuild/httplb"
)

type FlowServer struct{}

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

func CreateService(httpClient *httplb.Client) (*api.Service, error) {
	upstream := os.Getenv("MASTER_NODE_ENDPOINT")
	if upstream == "" {
		return nil, errors.New("MASTER_NODE_IP env var is required")
	}

	server := &FlowServer{}
	path, handler := flowv1connect.NewFlowServiceHandler(server)
	return &api.Service{Path: path, Handler: handler}, nil
}
