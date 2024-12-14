package rnode

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rflow"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/suser"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	"the-dev-tools/spec/dist/buf/go/flow/node/v1/nodev1connect"

	"connectrpc.com/connect"
)

type NodeServiceRPC struct {
	DB *sql.DB

	// parent
	fs sflow.FlowService
	us suser.UserService

	// sub
	nis snodeif.NodeIfService
	nrs snoderequest.NodeRequestService
	nlf snodefor.NodeForService
	ns  snode.NodeService
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService, fs sflow.FlowService, nis snodeif.NodeIfService, nrs snoderequest.NodeRequestService, nlf snodefor.NodeForService, ns snode.NodeService) *NodeServiceRPC {
	return &NodeServiceRPC{
		DB: db,

		us: us,
		fs: fs,

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
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	nodes, err := c.ns.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var NodeList []*nodev1.NodeListItem

	for _, currentNode := range nodes {
		switch currentNode.NodeKind {
		case mnode.NODE_KIND_REQUEST:
			nodeReq, err := c.nrs.GetNodeRequest(ctx, currentNode.ID)
			if err != nil {
				return nil, err
			}
			nodeListItem := &nodev1.NodeListItem{
				Kind: nodev1.NodeKind_NODE_KIND_FOR,
				Request: &nodev1.NodeRequest{
					NodeId: currentNode.ID.Bytes(),
					Position: &nodev1.Position{
						X: float32(currentNode.PositionX),
						Y: float32(currentNode.PositionY),
					},
					ExampleID: nodeReq.ExampleID.Bytes(),
				},
			}
			NodeList = append(NodeList, nodeListItem)
		case mnode.NODE_KIND_FOR:
			nodeFor, err := c.nlf.GetNodeFor(ctx, currentNode.ID)
			if err != nil {
				return nil, err
			}
			nodeListItem := &nodev1.NodeListItem{
				Kind: nodev1.NodeKind_NODE_KIND_FOR,
				For: &nodev1.NodeFor{
					NodeId: currentNode.ID.Bytes(),
					Position: &nodev1.Position{
						X: float32(currentNode.PositionX),
						Y: float32(currentNode.PositionY),
					},
					Iteration: int32(nodeFor.IterCount),
				},
			}
			NodeList = append(NodeList, nodeListItem)
		case mnode.NODE_KIND_START:
			// TODO: implement
		case mnode.NODE_KIND_CONDITION:
			// TODO: implement

		}
	}

	resp := &nodev1.NodeListResponse{
		Items: NodeList,
	}
	return connect.NewResponse(resp), nil
}

func (c *NodeServiceRPC) NodeGet(ctx context.Context, req *connect.Request[nodev1.NodeGetRequest]) (*connect.Response[nodev1.NodeGetResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (c *NodeServiceRPC) NodeCreate(ctx context.Context, req *connect.Request[nodev1.NodeCreateRequest]) (*connect.Response[nodev1.NodeCreateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (c *NodeServiceRPC) NodeUpdate(ctx context.Context, req *connect.Request[nodev1.NodeUpdateRequest]) (*connect.Response[nodev1.NodeUpdateResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func (c *NodeServiceRPC) NodeDelete(ctx context.Context, req *connect.Request[nodev1.NodeDeleteRequest]) (*connect.Response[nodev1.NodeDeleteResponse], error) {
	nodeID, err := idwrap.NewFromBytes(req.Msg.NodeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerNode(ctx, c.fs, c.us, c.ns, nodeID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.ns.DeleteNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&nodev1.NodeDeleteResponse{}), nil
}

func (c *NodeServiceRPC) NodeRun(ctx context.Context, req *connect.Request[nodev1.NodeRunRequest]) (*connect.Response[nodev1.NodeRunResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func CheckOwnerNode(ctx context.Context, fs sflow.FlowService, us suser.UserService, ns snode.NodeService, nodeID idwrap.IDWrap) (bool, error) {
	node, err := ns.GetNode(ctx, nodeID)
	if err != nil {
		return false, err
	}

	return rflow.CheckOwnerFlow(ctx, fs, us, node.FlowID)
}
