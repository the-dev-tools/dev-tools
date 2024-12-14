package rnode

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snoderequest"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	"the-dev-tools/spec/dist/buf/go/flow/node/v1/nodev1connect"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

type NodeServiceRPC struct {
	DB  *sql.DB
	nis snodeif.NodeIfService
	nrs snoderequest.NodeRequestService
	nlf snodefor.NodeForService
	ns  snode.NodeService
}

func NewNodeServiceRPC(db *sql.DB, nis snodeif.NodeIfService, nrs snoderequest.NodeRequestService, nlf snodefor.NodeForService, ns snode.NodeService) *NodeServiceRPC {
	return &NodeServiceRPC{
		DB:  db,
		nis: nis,
		nrs: nrs,
		nlf: nlf,
		ns:  ns,
	}
}

func CreateService(srv NodeServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := nodev1connect.NewNodeServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *NodeServiceRPC) NodeList(ctx context.Context, req *connect.Request[nodev1.NodeListRequest]) (*connect.Response[nodev1.NodeListResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// NodeGet calls flow.node.v1.NodeService.NodeGet.
func (c *NodeServiceRPC) NodeGet(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[nodev1.NodeGetResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// NodeCreate calls flow.node.v1.NodeService.NodeCreate.
func (c *NodeServiceRPC) NodeCreate(ctx context.Context, req *connect.Request[nodev1.NodeCreateRequest]) (*connect.Response[nodev1.NodeCreateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// NodeUpdate calls flow.node.v1.NodeService.NodeUpdate.
func (c *NodeServiceRPC) NodeUpdate(ctx context.Context, req *connect.Request[nodev1.NodeUpdateRequest]) (*connect.Response[nodev1.NodeUpdateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// NodeDelete calls flow.node.v1.NodeService.NodeDelete.
func (c *NodeServiceRPC) NodeDelete(ctx context.Context, req *connect.Request[nodev1.NodeDeleteRequest]) (*connect.Response[nodev1.NodeDeleteResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

// NodeRun calls flow.node.v1.NodeService.NodeRun.
func (c *NodeServiceRPC) NodeRun(ctx context.Context, req *connect.Request[nodev1.NodeRunRequest]) (*connect.Response[nodev1.NodeRunResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}
