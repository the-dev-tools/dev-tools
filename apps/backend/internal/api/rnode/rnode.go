package rnode

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rflow"
	"the-dev-tools/backend/pkg/flow/node/nrequest"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mnode"
	"the-dev-tools/backend/pkg/model/mnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnode/mnif"
	"the-dev-tools/backend/pkg/model/mnode/mnrequest"
	"the-dev-tools/backend/pkg/model/mnode/mnstart"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/snodestart"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/nodes/pkg/httpclient"
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
	ns   snode.NodeService
	nis  snodeif.NodeIfService
	nrs  snoderequest.NodeRequestService
	nfls snodefor.NodeForService
	nss  snodestart.NodeStartService

	// api
	ias  sitemapi.ItemApiService
	iaes sitemapiexample.ItemApiExampleService
	eqs  sexamplequery.ExampleQueryService
	ehs  sexampleheader.HeaderService
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService, fs sflow.FlowService, nis snodeif.NodeIfService, nrs snoderequest.NodeRequestService,
	nlf snodefor.NodeForService, ns snode.NodeService, nss snodestart.NodeStartService,
) *NodeServiceRPC {
	return &NodeServiceRPC{
		DB: db,

		us: us,
		fs: fs,

		ns:   ns,
		nis:  nis,
		nrs:  nrs,
		nfls: nlf,
		nss:  nss,
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

	NodeList := make([]*nodev1.NodeListItem, len(nodes))
	for i, node := range nodes {
		rpcNode, err := GetNodeSub(ctx, node, c.ns, c.nis, c.nrs, c.nfls, c.nss)
		if err != nil {
			return nil, err
		}
		convertedItem := &nodev1.NodeListItem{
			NodeId:    rpcNode.NodeId,
			Position:  rpcNode.Position,
			Kind:      rpcNode.Kind,
			Start:     rpcNode.Start,
			Request:   rpcNode.Request,
			For:       rpcNode.For,
			Condition: rpcNode.Condition,
		}
		NodeList[i] = convertedItem
	}

	resp := &nodev1.NodeListResponse{
		Items: NodeList,
	}
	return connect.NewResponse(resp), nil
}

func (c *NodeServiceRPC) NodeGet(ctx context.Context, req *connect.Request[nodev1.NodeGetRequest]) (*connect.Response[nodev1.NodeGetResponse], error) {
	nodeID, err := idwrap.NewFromBytes(req.Msg.NodeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerNode(ctx, c.fs, c.us, c.ns, nodeID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	node, err := c.ns.GetNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcNode, err := GetNodeSub(ctx, *node, c.ns, c.nis, c.nrs, c.nfls, c.nss)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := nodev1.NodeGetResponse{
		NodeId:    rpcNode.NodeId,
		Position:  rpcNode.Position,
		Kind:      rpcNode.Kind,
		Start:     rpcNode.Start,
		Request:   rpcNode.Request,
		For:       rpcNode.For,
		Condition: rpcNode.Condition,
	}

	return connect.NewResponse(&resp), nil
}

func (c *NodeServiceRPC) NodeCreate(ctx context.Context, req *connect.Request[nodev1.NodeCreateRequest]) (*connect.Response[nodev1.NodeCreateResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	RpcNodeCreated := &nodev1.Node{
		NodeId:    idwrap.NewNow().Bytes(),
		Position:  req.Msg.Position,
		Kind:      req.Msg.Kind,
		Start:     req.Msg.Start,
		Request:   req.Msg.Request,
		For:       req.Msg.For,
		Condition: req.Msg.Condition,
	}

	node, subNode, err := ConvertRPCNodeToModel(ctx, RpcNodeCreated, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	nsTX, err := snode.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = nsTX.CreateNode(ctx, *node)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// INFO: this is using reflection to check the type of subNode
	// in future, this should be refactored to use a more explicit way to check the type
	switch subNodeType := subNode.(type) {
	case mnrequest.MNRequest:
		nrsTX, err := snoderequest.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nrsTX.CreateNodeRequest(ctx, subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case mnfor.MNFor:
		nlfTX, err := snodefor.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nlfTX.CreateNodeFor(ctx, subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case mnif.MNIF:
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown subNode type: %T", subNode))
	}

	return connect.NewResponse(&nodev1.NodeCreateResponse{NodeId: RpcNodeCreated.NodeId}), nil
}

func (c *NodeServiceRPC) NodeUpdate(ctx context.Context, req *connect.Request[nodev1.NodeUpdateRequest]) (*connect.Response[nodev1.NodeUpdateResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	nodeID, err := idwrap.NewFromBytes(req.Msg.NodeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerNode(ctx, c.fs, c.us, c.ns, nodeID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	node, err := c.ns.GetNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if node.FlowID != flowID {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("node %s does not belong to flow %s", nodeID, flowID))
	}

	RpcNodeCreated := &nodev1.Node{
		NodeId:    nodeID.Bytes(),
		Position:  req.Msg.Position,
		Kind:      req.Msg.Kind,
		Start:     req.Msg.Start,
		Request:   req.Msg.Request,
		For:       req.Msg.For,
		Condition: req.Msg.Condition,
	}

	node, subNode, err := ConvertRPCNodeToModel(ctx, RpcNodeCreated, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	nsTX, err := snode.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = nsTX.UpdateNode(ctx, *node)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// INFO: this is using reflection to check the type of subNode
	// in future, this should be refactored to use a more explicit way to check the type
	switch subNodeType := subNode.(type) {
	case mnrequest.MNRequest:
		nrsTX, err := snoderequest.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nrsTX.UpdateNodeRequest(ctx, subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case mnfor.MNFor:
		nlfTX, err := snodefor.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nlfTX.UpdateNodeFor(ctx, subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case mnif.MNIF:
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown subNode type: %T", subNode))
	}
	return connect.NewResponse(&nodev1.NodeUpdateResponse{}), nil
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
	nodeID, err := idwrap.NewFromBytes(req.Msg.NodeId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	_, err = idwrap.NewFromBytes(req.Msg.EnvironmentId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerNode(ctx, c.fs, c.us, c.ns, nodeID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	node, err := c.ns.GetNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	switch node.NodeKind {
	case mnode.NODE_KIND_REQUEST:
		nodeReq, err := c.nrs.GetNodeRequest(ctx, node.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		itemApi, err := c.ias.GetItemApi(ctx, nodeReq.EndpointID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		example, err := c.iaes.GetApiExample(ctx, nodeReq.ExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		queries, err := c.eqs.GetExampleQueriesByExampleID(ctx, nodeReq.ExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		headers, err := c.ehs.GetHeaderByExampleID(ctx, nodeReq.ExampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		nrequest.New(nodeReq.FlowNodeID, *itemApi, *example, queries, headers, []byte{}, httpclient.New())

	case mnode.NODE_KIND_FOR:
	default:
		return nil, connect.NewError(connect.CodeUnimplemented, nil)
	}

	return nil, connect.NewError(connect.CodeUnimplemented, nil)
}

func CheckOwnerNode(ctx context.Context, fs sflow.FlowService, us suser.UserService, ns snode.NodeService, nodeID idwrap.IDWrap) (bool, error) {
	node, err := ns.GetNode(ctx, nodeID)
	if err != nil {
		return false, err
	}

	return rflow.CheckOwnerFlow(ctx, fs, us, node.FlowID)
}

func GetNodeSub(ctx context.Context, currentNode mnode.MNode, ns snode.NodeService, nis snodeif.NodeIfService, nrs snoderequest.NodeRequestService,
	nlf snodefor.NodeForService, nss snodestart.NodeStartService,
) (*nodev1.Node, error) {
	var rpcNode *nodev1.Node

	switch currentNode.NodeKind {
	case mnode.NODE_KIND_REQUEST:
		nodeReq, err := nrs.GetNodeRequest(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}
		nodeList := &nodev1.Node{
			Kind: nodev1.NodeKind_NODE_KIND_FOR,
			Request: &nodev1.NodeRequest{
				NodeId: currentNode.ID.Bytes(),
				Position: &nodev1.Position{
					X: float32(currentNode.PositionX),
					Y: float32(currentNode.PositionY),
				},
				ExampleId: nodeReq.ExampleID.Bytes(),
			},
		}
		rpcNode = nodeList
	case mnode.NODE_KIND_FOR:
		nodeFor, err := nlf.GetNodeFor(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}
		// TODO: ask which pos should be filled
		nodeList := &nodev1.Node{
			Kind: nodev1.NodeKind_NODE_KIND_FOR,
			Position: &nodev1.Position{
				X: float32(currentNode.PositionX),
				Y: float32(currentNode.PositionY),
			},
			For: &nodev1.NodeFor{
				NodeId: currentNode.ID.Bytes(),
				Position: &nodev1.Position{
					X: float32(currentNode.PositionX),
					Y: float32(currentNode.PositionY),
				},
				Iteration: int32(nodeFor.IterCount),
			},
		}
		rpcNode = nodeList
	case mnode.NODE_KIND_START:
		// TODO: can be remove later no need to fetch just id
		nodeStart, err := nss.GetNodeStart(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}
		rpcNode = &nodev1.Node{
			Kind: nodev1.NodeKind_NODE_KIND_START,
			Position: &nodev1.Position{
				X: float32(currentNode.PositionX),
				Y: float32(currentNode.PositionY),
			},
			Start: &nodev1.NodeStart{
				Position: &nodev1.Position{
					X: float32(currentNode.PositionX),
					Y: float32(currentNode.PositionY),
				},
				NodeId: nodeStart.FlowNodeID.Bytes(),
			},
		}

	case mnode.NODE_KIND_CONDITION:
		// TODO: implement
	}

	return rpcNode, nil
}

func ConvertRPCNodeToModel(ctx context.Context, rpcNode *nodev1.Node, flowID idwrap.IDWrap) (*mnode.MNode, interface{}, error) {
	var node *mnode.MNode
	var subNode interface{}
	id, err := idwrap.NewFromBytes(rpcNode.NodeId)
	if err != nil {
		return nil, nil, err
	}

	node = &mnode.MNode{
		ID:        id,
		FlowID:    flowID,
		NodeKind:  mnode.NodeKind(rpcNode.Kind),
		PositionX: float64(rpcNode.Position.X),
		PositionY: float64(rpcNode.Position.Y),
	}

	switch rpcNode.Kind {
	case nodev1.NodeKind_NODE_KIND_REQUEST:
		endpointID, err := idwrap.NewFromBytes(rpcNode.Request.EndpointId)
		if err != nil {
			return nil, nil, err
		}

		exampleID, err := idwrap.NewFromBytes(rpcNode.Request.ExampleId)
		if err != nil {
			return nil, nil, err
		}

		reqNode := &mnrequest.MNRequest{
			FlowNodeID: id,
			EndpointID: endpointID,
			ExampleID:  exampleID,
		}
		subNode = reqNode
	case nodev1.NodeKind_NODE_KIND_FOR:
		forNode := &mnfor.MNFor{
			FlowNodeID: id,
			IterCount:  int64(rpcNode.For.Iteration),
		}
		subNode = forNode
	case nodev1.NodeKind_NODE_KIND_START:
		startNode := &mnstart.StartNode{
			FlowNodeID: id,
		}
		subNode = startNode
	case nodev1.NodeKind_NODE_KIND_CONDITION:
		// TODO: implement
	}

	return node, subNode, nil
}
