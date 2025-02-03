package rnode

import (
	"context"
	"database/sql"
	"fmt"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rflow"
	"the-dev-tools/backend/pkg/flow/node/nrequest"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mcondition"
	"the-dev-tools/backend/pkg/model/mnnode"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnnode/mnif"
	"the-dev-tools/backend/pkg/model/mnnode/mnnoop"
	"the-dev-tools/backend/pkg/model/mnnode/mnrequest"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snodenoop"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/translate/tcondition"
	"the-dev-tools/nodes/pkg/httpclient"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	"the-dev-tools/spec/dist/buf/go/flow/node/v1/nodev1connect"
	referencev1 "the-dev-tools/spec/dist/buf/go/reference/v1"

	"connectrpc.com/connect"
	"github.com/pkg/errors"
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
	nss  snodenoop.NodeNoopService

	// api
	ias  sitemapi.ItemApiService
	iaes sitemapiexample.ItemApiExampleService
	eqs  sexamplequery.ExampleQueryService
	ehs  sexampleheader.HeaderService
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService, fs sflow.FlowService, nis snodeif.NodeIfService, nrs snoderequest.NodeRequestService,
	nlf snodefor.NodeForService, ns snode.NodeService, nss snodenoop.NodeNoopService,
	ias sitemapi.ItemApiService, ieas sitemapiexample.ItemApiExampleService,
	eqs sexamplequery.ExampleQueryService, ehs sexampleheader.HeaderService,
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

		ias:  ias,
		iaes: ieas,
		eqs:  eqs,
		ehs:  ehs,
	}
}

func CreateService(srv *NodeServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := nodev1connect.NewNodeServiceHandler(srv, options...)
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
			NoOp:      rpcNode.NoOp,
			Request:   rpcNode.Request,
			For:       rpcNode.For,
			Condition: rpcNode.Condition,
		}
		if rpcNode.Kind == nodev1.NodeKind_NODE_KIND_REQUEST {
			if rpcNode.Request.ExampleId != nil {
				example, err := idwrap.NewFromBytes(rpcNode.Request.ExampleId)
				if err != nil {
					return nil, connect.NewError(connect.CodeInvalidArgument, err)
				}
				ex, err := c.iaes.GetApiExample(ctx, example)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
				rpcNode.Request.CollectionId = ex.CollectionID.Bytes()
			}
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
		return nil, connect.NewError(connect.CodeInternal, errors.New("root node not found"))
	}
	rpcNode, err := GetNodeSub(ctx, *node, c.ns, c.nis, c.nrs, c.nfls, c.nss)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("sub node not found"))
	}
	resp := nodev1.NodeGetResponse{
		NodeId:    rpcNode.NodeId,
		Position:  rpcNode.Position,
		Kind:      rpcNode.Kind,
		NoOp:      rpcNode.NoOp,
		Request:   rpcNode.Request,
		For:       rpcNode.For,
		Condition: rpcNode.Condition,
	}
	if rpcNode.Kind == nodev1.NodeKind_NODE_KIND_REQUEST {
		if rpcNode.Request.ExampleId != nil {
			example, err := idwrap.NewFromBytes(rpcNode.Request.ExampleId)
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			ex, err := c.iaes.GetApiExample(ctx, example)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			resp.Request.CollectionId = ex.CollectionID.Bytes()
		}
	}

	return connect.NewResponse(&resp), nil
}

func (c *NodeServiceRPC) NodeCreate(ctx context.Context, req *connect.Request[nodev1.NodeCreateRequest]) (*connect.Response[nodev1.NodeCreateResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	rpcErr := permcheck.CheckPerm(rflow.CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, fmt.Errorf("invalid flow owner: %w", rpcErr)
	}

	NodeID := idwrap.NewNow()

	RpcNodeCreated := &nodev1.Node{
		NodeId:    NodeID.Bytes(),
		Position:  req.Msg.Position,
		Kind:      req.Msg.Kind,
		NoOp:      req.Msg.NoOp,
		Request:   req.Msg.Request,
		For:       req.Msg.For,
		Condition: req.Msg.Condition,
	}

	node, subNode, err := ConvertRPCNodeToModelWithoutID(ctx, RpcNodeCreated, flowID, NodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node: %w", err))
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
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
	case *mnrequest.MNRequest:
		subNodeType.FlowNodeID = node.ID
		nrsTX, err := snoderequest.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nrsTX.CreateNodeRequest(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case *mnfor.MNFor:
		nlfTX, err := snodefor.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nlfTX.CreateNodeFor(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case *mnif.MNIF:
		niTX, err := snodeif.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = niTX.CreateNodeIf(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case *mnnoop.NoopNode:
		noopTX, err := snodenoop.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = noopTX.CreateNodeNoop(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown subNode type: %T", subNode))
	}
	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&nodev1.NodeCreateResponse{NodeId: RpcNodeCreated.NodeId}), nil
}

func (c *NodeServiceRPC) NodeUpdate(ctx context.Context, req *connect.Request[nodev1.NodeUpdateRequest]) (*connect.Response[nodev1.NodeUpdateResponse], error) {
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

	if req.Msg.Position == nil {
		req.Msg.Position = &nodev1.Position{
			X: float32(node.PositionX),
			Y: float32(node.PositionY),
		}
	}

	RpcNodeCreated := &nodev1.Node{
		NodeId:    nodeID.Bytes(),
		Kind:      nodev1.NodeKind(node.NodeKind),
		Position:  req.Msg.Position,
		NoOp:      req.Msg.NoOp,
		Request:   req.Msg.Request,
		For:       req.Msg.For,
		Condition: req.Msg.Condition,
	}

	node, subNode, err := ConvertRPCNodeToModelWithID(ctx, RpcNodeCreated, node.FlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()
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
	case *mnrequest.MNRequest:
		nrsTX, err := snoderequest.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nrsTX.UpdateNodeRequest(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case *mnfor.MNFor:
		nlfTX, err := snodefor.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nlfTX.UpdateNodeFor(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case *mnif.MNIF:
		nisTX, err := snodeif.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nisTX.UpdateNodeIf(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case *mnnoop.NoopNode:
		nssTX, err := snodenoop.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nssTX.UpdateNodeNoop(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown subNode type: %T, %V", subNodeType, subNode))
	}
	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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
	case mnnode.NODE_KIND_REQUEST:
		nodeReq, err := c.nrs.GetNodeRequest(ctx, node.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if nodeReq.EndpointID == nil || nodeReq.ExampleID == nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("endpoint or example not found for %s", nodeReq.FlowNodeID))
		}

		endpointID := *nodeReq.EndpointID
		exampleID := *nodeReq.ExampleID

		itemApi, err := c.ias.GetItemApi(ctx, endpointID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		example, err := c.iaes.GetApiExample(ctx, exampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		queries, err := c.eqs.GetExampleQueriesByExampleID(ctx, exampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		headers, err := c.ehs.GetHeaderByExampleID(ctx, exampleID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		nrequest.New(nodeReq.FlowNodeID, *itemApi, *example, queries, headers, []byte{}, httpclient.New())

	case mnnode.NODE_KIND_FOR:
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

func GetNodeSub(ctx context.Context, currentNode mnnode.MNode, ns snode.NodeService, nis snodeif.NodeIfService, nrs snoderequest.NodeRequestService,
	nlf snodefor.NodeForService, nss snodenoop.NodeNoopService,
) (*nodev1.Node, error) {
	var rpcNode *nodev1.Node

	Position := &nodev1.Position{
		X: float32(currentNode.PositionX),
		Y: float32(currentNode.PositionY),
	}

	switch currentNode.NodeKind {
	case mnnode.NODE_KIND_REQUEST:
		nodeReq, err := nrs.GetNodeRequest(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}
		var rpcExampleID, rpcEndpointID, rpcDeltaExampleID []byte
		if nodeReq.ExampleID != nil {
			rpcExampleID = nodeReq.ExampleID.Bytes()
		}
		if nodeReq.EndpointID != nil {
			rpcEndpointID = nodeReq.EndpointID.Bytes()
		}
		if nodeReq.DeltaExampleID != nil {
			rpcDeltaExampleID = nodeReq.DeltaExampleID.Bytes()
		}

		nodeList := &nodev1.Node{
			NodeId:   currentNode.ID.Bytes(),
			Kind:     nodev1.NodeKind_NODE_KIND_REQUEST,
			Position: Position,
			Request: &nodev1.NodeRequest{
				CollectionId:   rpcExampleID,
				ExampleId:      rpcExampleID,
				EndpointId:     rpcEndpointID,
				DeltaExampleId: rpcDeltaExampleID,
			},
		}
		rpcNode = nodeList
	case mnnode.NODE_KIND_FOR:
		nodeFor, err := nlf.GetNodeFor(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}
		// TODO: ask which pos should be filled
		nodeList := &nodev1.Node{
			NodeId:   currentNode.ID.Bytes(),
			Kind:     nodev1.NodeKind_NODE_KIND_FOR,
			Position: Position,
			For: &nodev1.NodeFor{
				ErrorHandling: nodev1.ErrorHandling_ERROR_HANDLING_BREAK,
				Iterations:    int32(nodeFor.IterCount),
			},
		}
		rpcNode = nodeList
	case mnnode.NODE_KIND_NO_OP:
		// TODO: can be remove later no need to fetch just id
		nodeStart, err := nss.GetNodeNoop(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}
		a := nodev1.NodeNoOpKind(nodeStart.Type)

		rpcNode = &nodev1.Node{
			NodeId:   nodeStart.FlowNodeID.Bytes(),
			Kind:     nodev1.NodeKind_NODE_KIND_NO_OP,
			Position: Position,
			NoOp:     &a,
		}

	case mnnode.NODE_KIND_CONDITION:
		nodeCondition, err := nis.GetNodeIf(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}

		rpcCondition, err := tcondition.SeralizeConditionModelToRPC(nodeCondition.Condition)
		if err != nil {
			return nil, err
		}

		rpcNode = &nodev1.Node{
			NodeId:   nodeCondition.FlowNodeID.Bytes(),
			Kind:     nodev1.NodeKind_NODE_KIND_CONDITION,
			Position: Position,
			Condition: &nodev1.NodeCondition{
				Condition: rpcCondition,
			},
		}
	}

	return rpcNode, nil
}

func ConvertRPCNodeToModelWithID(ctx context.Context, rpcNode *nodev1.Node, flowID idwrap.IDWrap) (*mnnode.MNode, interface{}, error) {
	id, err := idwrap.NewFromBytes(rpcNode.NodeId)
	if err != nil {
		return nil, nil, err
	}
	return ConvertRPCNodeToModelWithoutID(ctx, rpcNode, flowID, id)
}

func ConvertRPCNodeToModelWithoutID(ctx context.Context, rpcNode *nodev1.Node, flowID idwrap.IDWrap, nodeID idwrap.IDWrap) (*mnnode.MNode, interface{}, error) {
	var node *mnnode.MNode
	var subNode interface{}

	if rpcNode.Position == nil {
		rpcNode.Position = &nodev1.Position{}
	}

	node = &mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		NodeKind:  mnnode.NodeKind(rpcNode.Kind),
		PositionX: float64(rpcNode.Position.X),
		PositionY: float64(rpcNode.Position.Y),
	}

	switch rpcNode.Kind {
	case nodev1.NodeKind_NODE_KIND_REQUEST:
		var endpointIDPtr, exampleIDPtr, deltaExampleIDPtr *idwrap.IDWrap
		if rpcNode.Request.EndpointId != nil {
			endpointID, err := idwrap.NewFromBytes(rpcNode.Request.EndpointId)
			if err != nil {
				return nil, nil, err
			}
			endpointIDPtr = &endpointID
		}
		if rpcNode.Request.ExampleId != nil {
			exampleID, err := idwrap.NewFromBytes(rpcNode.Request.ExampleId)
			if err != nil {
				return nil, nil, err
			}
			exampleIDPtr = &exampleID
		}
		if rpcNode.Request.DeltaExampleId != nil {
			deltaExampleID, err := idwrap.NewFromBytes(rpcNode.Request.DeltaExampleId)
			if err != nil {
				return nil, nil, err
			}
			deltaExampleIDPtr = &deltaExampleID
		}

		reqNode := &mnrequest.MNRequest{
			FlowNodeID:     nodeID,
			EndpointID:     endpointIDPtr,
			ExampleID:      exampleIDPtr,
			DeltaExampleID: deltaExampleIDPtr,
		}

		subNode = reqNode
	case nodev1.NodeKind_NODE_KIND_FOR:
		forNode := &mnfor.MNFor{
			FlowNodeID: nodeID,
			IterCount:  int64(rpcNode.For.Iterations),
		}
		subNode = forNode
	case nodev1.NodeKind_NODE_KIND_NO_OP:
		a := mnnoop.NoopTypes(*rpcNode.NoOp)
		noopNode := &mnnoop.NoopNode{
			FlowNodeID: nodeID,
			Type:       a,
			Name:       "NoOp",
		}
		subNode = noopNode
	case nodev1.NodeKind_NODE_KIND_CONDITION:

		var condition *mcondition.Condition
		var path string

		if rpcNode.Condition == nil {
			condition = mcondition.Default()
		} else if rpcNode.Condition.Condition == nil {
			condition = mcondition.Default()
		} else if rpcNode.Condition.Condition.Comparison == nil {
			condition = mcondition.Default()
		} else {
			comp := rpcNode.Condition.Condition.Comparison
			for i, v := range comp.Path {
				switch v.Kind {
				case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP:
					if v.Group == nil {
						return nil, nil, fmt.Errorf("group is nil")
					}
					if i != 0 {
						path += "."
					}
					path += *v.Group
				case referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY:
					if v.Key == nil {
						return nil, nil, fmt.Errorf("key is nil")
					}
					if i != 0 {
						path += "."
					}
					path += *v.Key
				default:
					// TODO: Add other types of reference keys here
					return nil, nil, fmt.Errorf("unknown reference key kind: %v", v.Kind)
				}
			}
			condition = &mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Value: comp.Value,
					Path:  path,
					Kind:  mcondition.ComparisonKind(comp.Kind),
				},
			}
		}

		if condition == nil {
			return nil, nil, fmt.Errorf("condition is nil")
		}

		ifNode := &mnif.MNIF{
			FlowNodeID: nodeID,
			Name:       "Condition",
			Condition:  *condition,
		}
		subNode = ifNode
	default:
		return nil, nil, fmt.Errorf("unknown node kind: %v", rpcNode.Kind)
	}

	return node, subNode, nil
}
