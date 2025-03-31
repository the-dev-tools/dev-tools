package rnode

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rflow"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mexamplerespheader"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/reference"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/translate/tcondition"
	"the-dev-tools/server/pkg/translate/tgeneric"
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
	ns    snode.NodeService
	nis   snodeif.NodeIfService
	nrs   snoderequest.NodeRequestService
	nfls  snodefor.NodeForService
	nlfes snodeforeach.NodeForEachService
	nss   snodenoop.NodeNoopService
	njss  snodejs.NodeJSService

	// api
	ias  sitemapi.ItemApiService
	iaes sitemapiexample.ItemApiExampleService
	eqs  sexamplequery.ExampleQueryService
	ehs  sexampleheader.HeaderService

	// endpoint body
	brs  sbodyraw.BodyRawService
	bfs  sbodyform.BodyFormService
	bues sbodyurl.BodyURLEncodedService
}

func NewNodeServiceRPC(db *sql.DB, us suser.UserService,
	fs sflow.FlowService, nis snodeif.NodeIfService, nrs snoderequest.NodeRequestService,
	nlfs snodefor.NodeForService, nlfes snodeforeach.NodeForEachService, ns snode.NodeService,
	nss snodenoop.NodeNoopService, njss snodejs.NodeJSService,
	ias sitemapi.ItemApiService, ieas sitemapiexample.ItemApiExampleService,
	eqs sexamplequery.ExampleQueryService, ehs sexampleheader.HeaderService,
	brs sbodyraw.BodyRawService, bfs sbodyform.BodyFormService, bues sbodyurl.BodyURLEncodedService,
) *NodeServiceRPC {
	return &NodeServiceRPC{
		DB: db,

		us: us,
		fs: fs,

		ns:    ns,
		nis:   nis,
		nrs:   nrs,
		nfls:  nlfs,
		nlfes: nlfes,
		nss:   nss,
		njss:  njss,

		ias:  ias,
		iaes: ieas,
		eqs:  eqs,
		ehs:  ehs,

		brs:  brs,
		bfs:  bfs,
		bues: bues,
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
		return nil, connect.NewError(connect.CodeInternal, errors.New("any node found"))
	}

	NodeList := make([]*nodev1.NodeListItem, len(nodes))
	for i, node := range nodes {
		rpcNode, err := GetNodeSub(ctx, node, c.ns, c.nis, c.nrs, c.nfls, c.nlfes, c.nss, c.njss)
		if err != nil {
			return nil, err
		}
		convertedItem := &nodev1.NodeListItem{
			Name:      node.Name,
			NodeId:    node.ID.Bytes(),
			State:     rpcNode.State,
			Position:  rpcNode.Position,
			Kind:      rpcNode.Kind,
			NoOp:      rpcNode.NoOp,
			Request:   rpcNode.Request,
			For:       rpcNode.For,
			ForEach:   rpcNode.ForEach,
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
					return nil, connect.NewError(connect.CodeInternal, errors.New("example not found"))
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
	rpcNode, err := GetNodeSub(ctx, *node, c.ns, c.nis, c.nrs, c.nfls, c.nlfes, c.nss, c.njss)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("sub node not found"))
	}
	resp := nodev1.NodeGetResponse{
		Name:      node.Name,
		NodeId:    node.ID.Bytes(),
		State:     rpcNode.State,
		Position:  rpcNode.Position,
		Kind:      rpcNode.Kind,
		NoOp:      rpcNode.NoOp,
		Request:   rpcNode.Request,
		For:       rpcNode.For,
		ForEach:   rpcNode.ForEach,
		Condition: rpcNode.Condition,
		Js:        rpcNode.Js,
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

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	NodeID := idwrap.NewNow()

	RpcNodeCreated := &nodev1.Node{
		NodeId:    NodeID.Bytes(),
		Name:      req.Msg.Name,
		Position:  req.Msg.Position,
		Kind:      req.Msg.Kind,
		NoOp:      req.Msg.NoOp,
		Request:   req.Msg.Request,
		For:       req.Msg.For,
		ForEach:   req.Msg.ForEach,
		Condition: req.Msg.Condition,
		Js:        req.Msg.Js,
	}

	nodeData, err := ConvertRPCNodeToModelWithoutID(ctx, RpcNodeCreated, flow.ID, NodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node: %w", err))
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer devtoolsdb.TxnRollback(tx)
	nsTX, err := snode.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	err = nsTX.CreateNode(ctx, *nodeData.Base)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// INFO: this is using reflection to check the type of subNode
	// in future, this should be refactored to use a more explicit way to check the type
	switch subNodeType := nodeData.SubNode.(type) {
	case *mnrequest.MNRequest:
		subNodeType.FlowNodeID = nodeData.Base.ID
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
	case *mnforeach.MNForEach:
		nlfeTX, err := snodeforeach.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = nlfeTX.CreateNodeForEach(ctx, *subNodeType)
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
	case *mnjs.MNJS:
		njTX, err := snodejs.NewTX(ctx, tx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		err = njTX.CreateNodeJS(ctx, *subNodeType)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown subNode type: %T", nodeData.SubNode))
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

	RpcNodeUpdate := &nodev1.Node{
		NodeId:    nodeID.Bytes(),
		Position:  req.Msg.Position,
		NoOp:      req.Msg.NoOp,
		Request:   req.Msg.Request,
		For:       req.Msg.For,
		ForEach:   req.Msg.ForEach,
		Condition: req.Msg.Condition,
		Js:        req.Msg.Js,
	}

	node, err := c.ns.GetNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if req.Msg.Position != nil {
		node.PositionX = float64(req.Msg.Position.X)
		node.PositionY = float64(req.Msg.Position.Y)
	}

	if req.Msg.Name != nil {
		node.Name = *req.Msg.Name
	}

	RpcNodeUpdate.Kind = nodev1.NodeKind(node.NodeKind)

	switch RpcNodeUpdate.Kind {
	case nodev1.NodeKind_NODE_KIND_REQUEST:
		if RpcNodeUpdate.Request != nil {
			var anyUpdate bool
			requestNode, err := c.nrs.GetNodeRequest(ctx, nodeID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if len(RpcNodeUpdate.Request.ExampleId) != 0 {
				exmplePtr, err := idwrap.NewFromBytes(RpcNodeUpdate.Request.ExampleId)
				if err != nil {
					return nil, err
				}
				requestNode.ExampleID = &exmplePtr
				anyUpdate = true
			}

			if len(RpcNodeUpdate.Request.EndpointId) != 0 {
				endpointPtr, err := idwrap.NewFromBytes(RpcNodeUpdate.Request.EndpointId)
				if err != nil {
					return nil, err
				}
				requestNode.EndpointID = &endpointPtr
				anyUpdate = true
			}

			if len(RpcNodeUpdate.Request.DeltaExampleId) != 0 {
				deltaExamplePtr, err := idwrap.NewFromBytes(RpcNodeUpdate.Request.DeltaExampleId)
				if err != nil {
					return nil, err
				}
				requestNode.DeltaExampleID = &deltaExamplePtr
				anyUpdate = true
			}

			if anyUpdate {
				err = c.nrs.UpdateNodeRequest(ctx, *requestNode)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	case nodev1.NodeKind_NODE_KIND_FOR:
		if RpcNodeUpdate.For != nil {
			var anyUpdate bool
			forNode, err := c.nfls.GetNodeFor(ctx, nodeID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if RpcNodeUpdate.For.Condition != nil {
				condition, err := tcondition.DeserializeConditionRPCToModel(RpcNodeUpdate.For.Condition)
				if err != nil {
					return nil, err
				}
				anyUpdate = true
				forNode.Condition = *condition
			}
			if RpcNodeUpdate.For.ErrorHandling != nodev1.ErrorHandling(mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED) {
				errorHandling := mnfor.ErrorHandling(RpcNodeUpdate.For.ErrorHandling)
				RpcNodeUpdate.For.ErrorHandling = nodev1.ErrorHandling(errorHandling)
				anyUpdate = true
			}
			if RpcNodeUpdate.For.Iterations != 0 {
				forNode.IterCount = int64(RpcNodeUpdate.For.Iterations)
			}
			if anyUpdate {
				err = c.nfls.UpdateNodeFor(ctx, *forNode)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	case nodev1.NodeKind_NODE_KIND_FOR_EACH:
		if RpcNodeUpdate.ForEach != nil {
			forEachNode, err := c.nlfes.GetNodeForEach(ctx, nodeID)
			if err != nil {
				return nil, err
			}

			var anyUpdate bool
			if RpcNodeUpdate.ForEach.Condition != nil {
				condition, err := tcondition.DeserializeConditionRPCToModel(RpcNodeUpdate.ForEach.Condition)
				if err != nil {
					return nil, err
				}
				forEachNode.Condition = *condition
				anyUpdate = true
			}

			if RpcNodeUpdate.ForEach.ErrorHandling != nodev1.ErrorHandling(mnfor.ErrorHandling_ERROR_HANDLING_UNSPECIFIED) {
				errorHandling := mnfor.ErrorHandling(RpcNodeUpdate.ForEach.ErrorHandling)
				if errorHandling != mnfor.ErrorHandling_ERROR_HANDLING_IGNORE {
					RpcNodeUpdate.ForEach.ErrorHandling = nodev1.ErrorHandling(errorHandling)
					anyUpdate = true
				}
			}

			if RpcNodeUpdate.ForEach.Path != nil {
				refs := tgeneric.MassConvert(RpcNodeUpdate.ForEach.Path, reference.ConvertRpcKeyToPkgKey)
				iterpath, err := reference.ConvertRefernceKeyArrayToStringPath(refs)
				if err != nil {
					return nil, err
				}
				forEachNode.IterPath = iterpath
				anyUpdate = true
			}

			if anyUpdate {
				err = c.nlfes.UpdateNodeForEach(ctx, *forEachNode)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	case nodev1.NodeKind_NODE_KIND_NO_OP:
	case nodev1.NodeKind_NODE_KIND_CONDITION:
		if RpcNodeUpdate.Condition != nil {
			nodeIf, err := c.nis.GetNodeIf(ctx, nodeID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if RpcNodeUpdate.Condition.Condition != nil {
				condition, err := tcondition.DeserializeConditionRPCToModel(RpcNodeUpdate.Condition.Condition)
				if err != nil {
					return nil, err
				}
				nodeIf.Condition = *condition
			}

			err = c.nis.UpdateNodeIf(ctx, *nodeIf)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	case nodev1.NodeKind_NODE_KIND_JS:
		if RpcNodeUpdate.Js != nil {
			nodeJS, err := c.njss.GetNodeJS(ctx, nodeID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if RpcNodeUpdate.Js.Code != "" {
				nodeJS.Code = []byte(RpcNodeUpdate.Js.Code)
			}

			err = c.njss.UpdateNodeJS(ctx, nodeJS)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	default:
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("unknown node kind: %s", RpcNodeUpdate.Kind))
	}

	err = c.ns.UpdateNode(ctx, *node)
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

func (c *NodeServiceRPC) NodeRun(ctx context.Context, req *connect.Request[nodev1.NodeRunRequest], stream *connect.ServerStream[nodev1.NodeRunResponse]) error {
	nodeID, err := idwrap.NewFromBytes(req.Msg.NodeId)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	_, err = idwrap.NewFromBytes(req.Msg.EnvironmentId)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerNode(ctx, c.fs, c.us, c.ns, nodeID))
	if rpcErr != nil {
		return rpcErr
	}

	node, err := c.ns.GetNode(ctx, nodeID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	switch node.NodeKind {
	case mnnode.NODE_KIND_REQUEST:
		nodeReq, err := c.nrs.GetNodeRequest(ctx, node.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		if nodeReq.EndpointID == nil || nodeReq.ExampleID == nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("endpoint or example not found for %s", nodeReq.FlowNodeID))
		}

		endpointID := *nodeReq.EndpointID
		exampleID := *nodeReq.ExampleID

		itemApi, err := c.ias.GetItemApi(ctx, endpointID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		example, err := c.iaes.GetApiExample(ctx, exampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		queries, err := c.eqs.GetExampleQueriesByExampleID(ctx, exampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		headers, err := c.ehs.GetHeaderByExampleID(ctx, exampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		rawBody, err := c.brs.GetBodyRawByExampleID(ctx, exampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		formBody, err := c.bfs.GetBodyFormsByExampleID(ctx, exampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		urlBody, err := c.bues.GetBodyURLEncodedByExampleID(ctx, exampleID)
		if err != nil {
			return err
		}
		requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, 1)

		// TODO: add proper new paramters
		exampleResp := mexampleresp.ExampleResp{}
		exampleRespHeader := []mexamplerespheader.ExampleRespHeader{}
		asserts := []massert.Assert{}

		// TODO: add name
		nrequest.New(nodeReq.FlowNodeID, "", *itemApi, *example, queries, headers, *rawBody, formBody, urlBody,
			exampleResp, exampleRespHeader, asserts, httpclient.New(), requestNodeRespChan)

	case mnnode.NODE_KIND_FOR:
	default:
		return connect.NewError(connect.CodeUnimplemented, nil)
	}

	return connect.NewError(connect.CodeUnimplemented, nil)
}

func CheckOwnerNode(ctx context.Context, fs sflow.FlowService, us suser.UserService, ns snode.NodeService, nodeID idwrap.IDWrap) (bool, error) {
	node, err := ns.GetNode(ctx, nodeID)
	if err != nil {
		return false, err
	}

	return rflow.CheckOwnerFlow(ctx, fs, us, node.FlowID)
}

func GetNodeSub(ctx context.Context, currentNode mnnode.MNode, ns snode.NodeService, nis snodeif.NodeIfService, nrs snoderequest.NodeRequestService,
	nlfs snodefor.NodeForService, nlfes snodeforeach.NodeForEachService, nss snodenoop.NodeNoopService, njss snodejs.NodeJSService,
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
			Name:     currentNode.Name,
			Request: &nodev1.NodeRequest{
				CollectionId:   rpcExampleID,
				ExampleId:      rpcExampleID,
				EndpointId:     rpcEndpointID,
				DeltaExampleId: rpcDeltaExampleID,
			},
		}
		rpcNode = nodeList
	case mnnode.NODE_KIND_FOR:
		nodeFor, err := nlfs.GetNodeFor(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}
		rpcCond, err := tcondition.SeralizeConditionModelToRPC(nodeFor.Condition)
		if err != nil {
			return nil, err
		}

		nodeList := &nodev1.Node{
			NodeId:   currentNode.ID.Bytes(),
			Kind:     nodev1.NodeKind_NODE_KIND_FOR,
			Position: Position,
			Name:     currentNode.Name,
			For: &nodev1.NodeFor{
				ErrorHandling: nodev1.ErrorHandling(nodeFor.ErrorHandling),
				Iterations:    int32(nodeFor.IterCount),
				Condition:     rpcCond,
			},
		}
		rpcNode = nodeList
	case mnnode.NODE_KIND_FOR_EACH:
		nodeForEach, err := nlfes.GetNodeForEach(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}

		rpcCond, err := tcondition.SeralizeConditionModelToRPC(nodeForEach.Condition)
		if err != nil {
			return nil, err
		}

		refPaths, err := reference.ConvertStringPathToReferenceKeyArray(nodeForEach.IterPath)
		if err != nil {
			return nil, err
		}
		rpcRefs := tgeneric.MassConvert(refPaths, reference.ConvertPkgKeyToRpc)

		nodeList := &nodev1.Node{
			NodeId:   currentNode.ID.Bytes(),
			Kind:     nodev1.NodeKind_NODE_KIND_FOR_EACH,
			Position: Position,
			Name:     currentNode.Name,
			ForEach: &nodev1.NodeForEach{
				ErrorHandling: nodev1.ErrorHandling(nodeForEach.ErrorHandling),
				Condition:     rpcCond,
				Path:          rpcRefs,
			},
		}
		rpcNode = nodeList
	case mnnode.NODE_KIND_NO_OP:
		nodeNoop, err := nss.GetNodeNoop(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}
		NoOpKind := nodev1.NodeNoOpKind(nodeNoop.Type)

		rpcNode = &nodev1.Node{
			NodeId:   nodeNoop.FlowNodeID.Bytes(),
			Kind:     nodev1.NodeKind_NODE_KIND_NO_OP,
			Name:     currentNode.Name,
			Position: Position,
			NoOp:     &NoOpKind,
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
			Position: Position,
			Kind:     nodev1.NodeKind_NODE_KIND_CONDITION,
			Name:     currentNode.Name,
			Condition: &nodev1.NodeCondition{
				Condition: rpcCondition,
			},
		}
	case mnnode.NODE_KIND_JS:

		nodeJS, err := njss.GetNodeJS(ctx, currentNode.ID)
		if err != nil {
			return nil, err
		}

		if nodeJS.CodeCompressType != compress.CompressTypeNone {
			nodeJS.Code, err = compress.Decompress(nodeJS.Code, nodeJS.CodeCompressType)
			if err != nil {
				return nil, err
			}
		}

		rpcNode = &nodev1.Node{
			NodeId:   nodeJS.FlowNodeID.Bytes(),
			Position: Position,
			Kind:     nodev1.NodeKind_NODE_KIND_JS,
			Name:     currentNode.Name,
			Js: &nodev1.NodeJS{
				Code: string(nodeJS.Code),
			},
		}
	}

	rpcNode.State = nodev1.NodeState(currentNode.State)

	return rpcNode, nil
}

func ConvertRPCNodeToModelWithID(ctx context.Context, rpcNode *nodev1.Node, flowID idwrap.IDWrap) (*NodeData, error) {
	id, err := idwrap.NewFromBytes(rpcNode.NodeId)
	if err != nil {
		return nil, err
	}
	return ConvertRPCNodeToModelWithoutID(ctx, rpcNode, flowID, id)
}

type NodeData struct {
	Base    *mnnode.MNode
	SubNode any
}

func ConvertRPCNodeToModelWithoutID(ctx context.Context, rpcNode *nodev1.Node, flowID idwrap.IDWrap, nodeID idwrap.IDWrap) (*NodeData, error) {
	var subNode any

	if rpcNode.Position == nil {
		rpcNode.Position = &nodev1.Position{}
	}

	baseNode := &mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      rpcNode.Name,
		NodeKind:  mnnode.NodeKind(rpcNode.Kind),
		PositionX: float64(rpcNode.Position.X),
		PositionY: float64(rpcNode.Position.Y),
	}

	switch rpcNode.Kind {
	case nodev1.NodeKind_NODE_KIND_REQUEST:
		var endpointIDPtr, exampleIDPtr, deltaExampleIDPtr *idwrap.IDWrap
		if rpcNode.Request != nil {
			if rpcNode.Request.EndpointId != nil {
				endpointID, err := idwrap.NewFromBytes(rpcNode.Request.EndpointId)
				if err != nil {
					return nil, err
				}
				endpointIDPtr = &endpointID
			}
			if rpcNode.Request.ExampleId != nil {
				exampleID, err := idwrap.NewFromBytes(rpcNode.Request.ExampleId)
				if err != nil {
					return nil, err
				}
				exampleIDPtr = &exampleID
			}
			if rpcNode.Request.DeltaExampleId != nil {
				deltaExampleID, err := idwrap.NewFromBytes(rpcNode.Request.DeltaExampleId)
				if err != nil {
					return nil, err
				}
				deltaExampleIDPtr = &deltaExampleID
			}
		}

		reqNode := &mnrequest.MNRequest{
			FlowNodeID:     nodeID,
			EndpointID:     endpointIDPtr,
			ExampleID:      exampleIDPtr,
			DeltaExampleID: deltaExampleIDPtr,
		}

		subNode = reqNode
	case nodev1.NodeKind_NODE_KIND_FOR:
		var condition *mcondition.Condition
		var err error

		forNode := rpcNode.For

		if forNode.Condition == nil {
			condition = mcondition.Default()
		} else if forNode.Condition.Comparison == nil {
			condition = mcondition.Default()
		} else {
			condition, err = tcondition.DeserializeConditionRPCToModel(forNode.Condition)
			if err != nil {
				return nil, err
			}
		}

		if condition == nil {
			return nil, fmt.Errorf("condition is nil")
		}

		forNodeConverted := &mnfor.MNFor{
			FlowNodeID:    nodeID,
			IterCount:     int64(forNode.Iterations),
			Condition:     *condition,
			ErrorHandling: mnfor.ErrorHandling(forNode.ErrorHandling),
		}
		subNode = forNodeConverted
	case nodev1.NodeKind_NODE_KIND_FOR_EACH:
		var condition *mcondition.Condition
		var err error
		var iterpath string

		forEach := rpcNode.ForEach
		if forEach.Path != nil {
			refs := tgeneric.MassConvert(rpcNode.ForEach.Path, reference.ConvertRpcKeyToPkgKey)
			iterpath, err = reference.ConvertRefernceKeyArrayToStringPath(refs)
			if err != nil {
				return nil, err
			}
		}

		if forEach.Condition == nil {
			condition = mcondition.Default()
		} else if forEach.Condition.Comparison == nil {
			condition = mcondition.Default()
		} else {
			condition, err = tcondition.DeserializeConditionRPCToModel(forEach.Condition)
			if err != nil {
				return nil, err
			}
		}

		if condition == nil {
			return nil, fmt.Errorf("condition is nil")
		}

		forNode := &mnforeach.MNForEach{
			FlowNodeID:    nodeID,
			IterPath:      iterpath,
			Condition:     *condition,
			ErrorHandling: mnfor.ErrorHandling(forEach.ErrorHandling),
		}
		subNode = forNode
	case nodev1.NodeKind_NODE_KIND_NO_OP:
		a := mnnoop.NoopTypes(*rpcNode.NoOp)
		noopNode := &mnnoop.NoopNode{
			FlowNodeID: nodeID,
			Type:       a,
		}
		subNode = noopNode
	case nodev1.NodeKind_NODE_KIND_CONDITION:

		var condition *mcondition.Condition
		var err error

		conditionNode := rpcNode.Condition

		if conditionNode.Condition == nil {
			condition = mcondition.Default()
		} else if conditionNode.Condition.Comparison == nil {
			condition = mcondition.Default()
		} else {
			condition, err = tcondition.DeserializeConditionRPCToModel(conditionNode.Condition)
			if err != nil {
				return nil, err
			}
		}

		ifNode := &mnif.MNIF{
			FlowNodeID: nodeID,
			Condition:  *condition,
		}
		subNode = ifNode
	case nodev1.NodeKind_NODE_KIND_JS:
		subNode = &mnjs.MNJS{
			FlowNodeID: nodeID,
			Code:       []byte(rpcNode.Js.Code),
		}
	default:
		return nil, fmt.Errorf("unknown node kind: %v", rpcNode.Kind)
	}

	return &NodeData{Base: baseNode, SubNode: subNode}, nil
}
