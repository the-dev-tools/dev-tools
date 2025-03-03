package rflow

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"sort"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rtag"
	"the-dev-tools/backend/internal/api/rworkspace"
	"the-dev-tools/backend/pkg/dbtime"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/nfor"
	"the-dev-tools/backend/pkg/flow/node/nforeach"
	"the-dev-tools/backend/pkg/flow/node/nif"
	"the-dev-tools/backend/pkg/flow/node/nnoop"
	"the-dev-tools/backend/pkg/flow/node/nrequest"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/backend/pkg/httpclient"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/logconsole"
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/model/mnnode"
	"the-dev-tools/backend/pkg/model/mnnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnnode/mnforeach"
	"the-dev-tools/backend/pkg/model/mnnode/mnif"
	"the-dev-tools/backend/pkg/model/mnnode/mnnoop"
	"the-dev-tools/backend/pkg/model/mnnode/mnrequest"
	"the-dev-tools/backend/pkg/permcheck"
	"the-dev-tools/backend/pkg/service/sbodyform"
	"the-dev-tools/backend/pkg/service/sbodyraw"
	"the-dev-tools/backend/pkg/service/sbodyurl"
	"the-dev-tools/backend/pkg/service/sedge"
	"the-dev-tools/backend/pkg/service/sexampleheader"
	"the-dev-tools/backend/pkg/service/sexamplequery"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/service/sflowtag"
	"the-dev-tools/backend/pkg/service/sitemapi"
	"the-dev-tools/backend/pkg/service/sitemapiexample"
	"the-dev-tools/backend/pkg/service/snode"
	"the-dev-tools/backend/pkg/service/snodefor"
	"the-dev-tools/backend/pkg/service/snodeforeach"
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snodenoop"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/stag"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/backend/pkg/translate/tflow"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
	"the-dev-tools/spec/dist/buf/go/flow/v1/flowv1connect"
	"time"

	"connectrpc.com/connect"
)

type FlowServiceRPC struct {
	DB *sql.DB
	ws sworkspace.WorkspaceService
	us suser.UserService
	ts stag.TagService

	// flow
	fs  sflow.FlowService
	fts sflowtag.FlowTagService
	fes sedge.EdgeService

	// request
	as sitemapi.ItemApiService
	es sitemapiexample.ItemApiExampleService
	qs sexamplequery.ExampleQueryService
	hs sexampleheader.HeaderService

	// body
	brs  sbodyraw.BodyRawService
	bfs  sbodyform.BodyFormService
	bues sbodyurl.BodyURLEncodedService

	// sub nodes
	ns   snode.NodeService
	rns  snoderequest.NodeRequestService
	fns  snodefor.NodeForService
	fens snodeforeach.NodeForEachService
	sns  snodenoop.NodeNoopService
	ins  snodeif.NodeIfService

	logChanMap logconsole.LogChanMap
}

func New(db *sql.DB, ws sworkspace.WorkspaceService,
	us suser.UserService, ts stag.TagService,
	fs sflow.FlowService, fts sflowtag.FlowTagService,
	fes sedge.EdgeService, as sitemapi.ItemApiService, es sitemapiexample.ItemApiExampleService, qs sexamplequery.ExampleQueryService, hs sexampleheader.HeaderService,
	brs sbodyraw.BodyRawService, bfs sbodyform.BodyFormService, bues sbodyurl.BodyURLEncodedService,
	ns snode.NodeService, rns snoderequest.NodeRequestService, flns snodefor.NodeForService, fens snodeforeach.NodeForEachService,
	sns snodenoop.NodeNoopService, ins snodeif.NodeIfService,
	logChanMap logconsole.LogChanMap,
) FlowServiceRPC {
	return FlowServiceRPC{
		DB: db,
		ws: ws,
		us: us,
		ts: ts,

		// flow
		fs:  fs,
		fes: fes,
		fts: fts,

		// body
		brs:  brs,
		bfs:  bfs,
		bues: bues,

		// request
		as: as,
		es: es,
		qs: qs,
		hs: hs,

		// sub nodes
		ns:   ns,
		rns:  rns,
		fns:  flns,
		fens: fens,
		sns:  sns,
		ins:  ins,

		logChanMap: logChanMap,
	}
}

func CreateService(srv FlowServiceRPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := flowv1connect.NewFlowServiceHandler(&srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (c *FlowServiceRPC) FlowList(ctx context.Context, req *connect.Request[flowv1.FlowListRequest]) (*connect.Response[flowv1.FlowListResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var tagIDPtr *idwrap.IDWrap
	if len(req.Msg.TagId) > 0 {
		tagID, err := idwrap.NewFromBytes(req.Msg.TagId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		tagIDPtr = &tagID
	}

	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, workspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	var rpcFlows []*flowv1.FlowListItem

	if tagIDPtr == nil {
		flow, err := c.fs.GetFlowsByWorkspaceID(ctx, workspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		rpcFlows = append(rpcFlows, tgeneric.MassConvert(flow, tflow.SeralizeModelToRPCItem)...)

	} else {
		rpcErr := permcheck.CheckPerm(rtag.CheckOwnerTag(ctx, c.ts, c.us, *tagIDPtr))
		if rpcErr != nil {
			return nil, rpcErr
		}
		tagFlows, err := c.fts.GetFlowTagsByTagID(ctx, *tagIDPtr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// TODO: make this one query
		for _, tagFlow := range tagFlows {
			latestFlow, err := c.fs.GetFlow(ctx, tagFlow.FlowID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			rpcFlow := tflow.SeralizeModelToRPCItem(latestFlow)
			rpcFlows = append(rpcFlows, rpcFlow)
		}
	}

	rpcResp := &flowv1.FlowListResponse{
		WorkspaceId: req.Msg.WorkspaceId,
		TagId:       req.Msg.TagId,
		Items:       rpcFlows,
	}
	return connect.NewResponse(rpcResp), nil
}

func (c *FlowServiceRPC) FlowGet(ctx context.Context, req *connect.Request[flowv1.FlowGetRequest]) (*connect.Response[flowv1.FlowGetResponse], error) {
	if len(req.Msg.FlowId) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	rpcFlow := tflow.SeralizeModelToRPC(flow)
	rpcResp := &flowv1.FlowGetResponse{
		FlowId: rpcFlow.FlowId,
		Name:   rpcFlow.Name,
	}
	return connect.NewResponse(rpcResp), nil
}

func (c *FlowServiceRPC) FlowCreate(ctx context.Context, req *connect.Request[flowv1.FlowCreateRequest]) (*connect.Response[flowv1.FlowCreateResponse], error) {
	workspaceID, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(rworkspace.CheckOwnerWorkspace(ctx, c.us, workspaceID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	ws, err := c.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	name := req.Msg.Name

	flowID := idwrap.NewNow()

	flow := mflow.Flow{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        name,
	}

	nodeNoopID := idwrap.NewNow()

	tx, err := c.DB.Begin()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	defer tx.Rollback()

	txFlow, err := sflow.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txNode, err := snode.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	txNoopNode, err := snodenoop.NewTX(ctx, tx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = txFlow.CreateFlow(ctx, flow)
	if err != nil {
		return nil, err
	}

	err = txNode.CreateNode(ctx, mnnode.MNode{
		ID:        nodeNoopID,
		FlowID:    flowID,
		Name:      "Default Start Node",
		NodeKind:  mnnode.NODE_KIND_NO_OP,
		PositionX: float64(0),
		PositionY: float64(0),
	})
	if err != nil {
		return nil, err
	}
	err = txNoopNode.CreateNodeNoop(ctx, mnnoop.NoopNode{
		FlowNodeID: nodeNoopID,
		Type:       mnnoop.NODE_NO_OP_KIND_START,
	})
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ws.FlowCount++
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowv1.FlowCreateResponse{
		FlowId: flowID.Bytes(),
	}), nil
}

func (c *FlowServiceRPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[flowv1.FlowUpdateResponse], error) {
	msg := req.Msg
	if msg.FlowId == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}
	if msg.Name == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	flowID, err := idwrap.NewFromBytes(msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}

	flow.Name = *msg.Name

	err = c.fs.UpdateFlow(ctx, flow)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowv1.FlowUpdateResponse{}), nil
}

func (c *FlowServiceRPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[flowv1.FlowDeleteResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}

	ws, err := c.ws.Get(ctx, flow.WorkspaceID)
	if err != nil {
		return nil, err
	}

	ws.FlowCount--
	ws.Updated = dbtime.DBNow()
	err = c.ws.Update(ctx, ws)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	err = c.fs.DeleteFlow(ctx, flowID)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&flowv1.FlowDeleteResponse{}), nil
}

func (c *FlowServiceRPC) FlowRun(ctx context.Context, req *connect.Request[flowv1.FlowRunRequest], stream *connect.ServerStream[flowv1.FlowRunResponse]) error {
	return c.FlowRunAdHoc(ctx, req, stream)
}

func (c *FlowServiceRPC) FlowRunAdHoc(ctx context.Context, req *connect.Request[flowv1.FlowRunRequest], stream api.ServerStreamAdHoc[flowv1.FlowRunResponse]) error {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return rpcErr
	}

	flow, err := c.fs.GetFlow(ctx, flowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	latestFlowID := flow.ID

	nodes, err := c.ns.GetNodesByFlowID(ctx, latestFlowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get nodes"))
	}

	edges, err := c.fes.GetEdgesByFlowID(ctx, latestFlowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get edges"))
	}
	edgeMap := edge.NewEdgesMap(edges)

	var requestNodes []mnrequest.MNRequest
	var forNodes []mnfor.MNFor
	var forEachNodes []mnforeach.MNForEach
	var ifNodes []mnif.MNIF
	var noopNodes []mnnoop.NoopNode
	var startNodeID idwrap.IDWrap

	for _, node := range nodes {
		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			rn, err := c.rns.GetNodeRequest(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node request: %w", err))
			}
			requestNodes = append(requestNodes, *rn)
		case mnnode.NODE_KIND_FOR:
			fn, err := c.fns.GetNodeFor(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node for: %w", err))
			}
			forNodes = append(forNodes, *fn)
		case mnnode.NODE_KIND_FOR_EACH:
			fen, err := c.fens.GetNodeForEach(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node for each: %w", err))
			}
			forEachNodes = append(forEachNodes, *fen)
		case mnnode.NODE_KIND_NO_OP:
			sn, err := c.sns.GetNodeNoop(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node start: %w", err))
			}
			noopNodes = append(noopNodes, *sn)
		case mnnode.NODE_KIND_CONDITION:
			in, err := c.ins.GetNodeIf(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("get node if"))
			}
			ifNodes = append(ifNodes, *in)
		default:
			return connect.NewError(connect.CodeInternal, errors.New("not supported node"))
		}
	}

	var foundStartNode bool
	for _, node := range noopNodes {
		if node.Type == mnnoop.NODE_NO_OP_KIND_START {
			if foundStartNode {
				return connect.NewError(connect.CodeInternal, errors.New("multiple start nodes"))
			}
			foundStartNode = true
			startNodeID = node.FlowNodeID
		}
	}
	if !foundStartNode {
		return connect.NewError(connect.CodeInternal, errors.New("no start node"))
	}

	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, 0)
	for _, forNode := range forNodes {
		flowNodeMap[forNode.FlowNodeID] = nfor.New(forNode.FlowNodeID, forNode.IterCount, time.Second)
	}

	for _, requestNode := range requestNodes {
		if requestNode.EndpointID == nil || requestNode.ExampleID == nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("endpoint or example not found for %s", requestNode.FlowNodeID))
		}
		endpoint, err := c.as.GetItemApi(ctx, *requestNode.EndpointID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		example, err := c.es.GetApiExample(ctx, *requestNode.ExampleID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if example.ItemApiID != endpoint.ID {
			return connect.NewError(connect.CodeInternal, errors.New("example and endpoint not match"))
		}
		headers, err := c.hs.GetHeaderByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.New("get headers"))
		}
		queries, err := c.qs.GetExampleQueriesByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.New("get queries"))
		}

		exampleUlid := example.ID
		bodyBytes := &bytes.Buffer{}
		switch example.BodyType {
		case mitemapiexample.BodyTypeNone:
		case mitemapiexample.BodyTypeRaw:
			bodyData, err := c.brs.GetBodyRawByExampleID(ctx, exampleUlid)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			bodyBytes.Write(bodyData.Data)
		case mitemapiexample.BodyTypeForm:
			forms, err := c.bfs.GetBodyFormsByExampleID(ctx, exampleUlid)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			writer := multipart.NewWriter(bodyBytes)

			for _, v := range forms {
				err = writer.WriteField(v.BodyKey, v.Value)
				if err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
			}

		case mitemapiexample.BodyTypeUrlencoded:
			urls, err := c.bues.GetBodyURLEncodedByExampleID(ctx, exampleUlid)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
			urlVal := url.Values{}
			for _, url := range urls {
				urlVal.Add(url.BodyKey, url.Value)
			}

		}
		body := bodyBytes.Bytes()

		httpClient := httpclient.New()

		flowNodeMap[requestNode.FlowNodeID] = nrequest.New(requestNode.FlowNodeID, *endpoint, *example, queries, headers, body, httpClient)
	}

	for _, ifNode := range ifNodes {
		comp := ifNode.Condition.Comparisons
		flowNodeMap[ifNode.FlowNodeID] = nif.New(ifNode.FlowNodeID, comp.Kind, comp.Path, comp.Value)
	}

	for _, noopNode := range noopNodes {
		flowNodeMap[noopNode.FlowNodeID] = nnoop.New(noopNode.FlowNodeID)
	}

	for _, forEachNode := range forEachNodes {
		// TODO: add names
		// TODO: make timeout configurable
		flowNodeMap[forEachNode.FlowNodeID] = nforeach.New(forEachNode.FlowNodeID, "", forEachNode.IterPath, time.Second,
			forEachNode.Condition, forEachNode.ErrorHandling)
	}

	// TODO: get timeout from flow config
	runnerInst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), latestFlowID, startNodeID, flowNodeMap, edgeMap, time.Second*10)

	flowNodeStatusChan := make(chan runner.FlowNodeStatus, 1000)
	flowStatusChan := make(chan runner.FlowStatus, 10)
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	updateNodeChan := make(chan mnnode.MNode, 1000)
	go func() {
		nodeStatusFunc := func(flowNodeStatus runner.FlowNodeStatus) {
			id := flowNodeStatus.NodeID
			idStr := id.String()
			stateStr := mnnode.StringNodeState(flowNodeStatus.State)

			localErr := c.logChanMap.SendMsgToUserWithContext(ctx, idwrap.NewNow(), fmt.Sprintf("Node %s: %s", idStr, stateStr))
			if localErr != nil {
				done <- localErr
				return
			}
			resp := &flowv1.FlowRunResponse{
				NodeId: flowNodeStatus.NodeID.Bytes(),
				State:  nodev1.NodeState(flowNodeStatus.State),
			}
			nodeStateData := mnnode.MNode{
				ID:        id,
				State:     flowNodeStatus.State,
				StateData: flowNodeStatus.OutputData,
			}
			updateNodeChan <- nodeStateData
			localErr = stream.Send(resp)
			if localErr != nil {
				done <- localErr
				fmt.Println("Error in sending response")
				return
			}
		}

		defer close(done)
		for {
			select {
			case <-subCtx.Done():
				close(flowNodeStatusChan)
				close(flowStatusChan)
				done <- errors.New("context done")
				return
			case flowNodeStatus := <-flowNodeStatusChan:
				nodeStatusFunc(flowNodeStatus)
			case flowStatus := <-flowStatusChan:
				if len(flowNodeStatusChan) > 0 {
					for flowNodeStatus := range flowNodeStatusChan {
						nodeStatusFunc(flowNodeStatus)
					}
				}
				fmt.Println("FlowStatus: ", runner.FlowStatusString(flowStatus))
				if runner.IsFlowStatusDone(flowStatus) {
					done <- nil
					return
				}
			}
		}
	}()

	flowRunErr := runnerInst.Run(ctx, flowNodeStatusChan, flowStatusChan)

	// wait for the flow to finish
	flowErr := <-done

	if flowRunErr != nil {
		return connect.NewError(connect.CodeInternal, flowRunErr)
	}
	close(updateNodeChan)

	flow.VersionParentID = &flow.ID
	res, err := c.PrepareCopyFlow(ctx, flow.WorkspaceID, flow)
	if err != nil {
		fmt.Println("Error in PrepareCopyFlow", err)
		return connect.NewError(connect.CodeInternal, err)
	}

	tx, err := c.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	txNode, err := snode.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create node service: %w", err)
	}

	for node := range updateNodeChan {
		err = txNode.UpdateNodeState(ctx, node)
		if err != nil {
			return fmt.Errorf("update node: %w", err)
		}
	}

	if flowErr != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	err = c.CopyFlow(ctx, tx, res)
	if err != nil {
		fmt.Println("Error in CopyFlow")
		return connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

type CopyFlowResult struct {
	Flow  mflow.Flow
	Nodes []mnnode.MNode
	Edges []edge.Edge

	// Specific node types
	RequestNodes []mnrequest.MNRequest
	ForNodes     []mnfor.MNFor
	ForEachNodes []mnforeach.MNForEach
	IfNodes      []mnif.MNIF
	NoopNodes    []mnnoop.NoopNode
}

func (c *FlowServiceRPC) PrepareCopyFlow(ctx context.Context, workspaceID idwrap.IDWrap, originalFlow mflow.Flow) (CopyFlowResult, error) {
	result := CopyFlowResult{}

	newFlowID := idwrap.NewNow()
	oldFlowID := originalFlow.ID
	originalFlow.ID = newFlowID
	result.Flow = originalFlow

	nodes, err := c.ns.GetNodesByFlowID(ctx, oldFlowID)
	if err != nil {
		return result, fmt.Errorf("get nodes: %w", err)
	}

	edges, err := c.fes.GetEdgesByFlowID(ctx, oldFlowID)
	if err != nil {
		return result, fmt.Errorf("get edges: %w", err)
	}

	oldToNewIDMap := make(map[idwrap.IDWrap]idwrap.IDWrap)

	for _, node := range nodes {
		newNodeID := idwrap.NewNow()
		oldToNewIDMap[node.ID] = newNodeID

		newNode := node
		newNode.ID = newNodeID
		newNode.FlowID = newFlowID
		result.Nodes = append(result.Nodes, newNode)

		// Get and copy specific node data based on type
		switch node.NodeKind {
		case mnnode.NODE_KIND_REQUEST:
			rn, err := c.rns.GetNodeRequest(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get request node: %w", err)
			}
			newRN := *rn
			newRN.FlowNodeID = newNodeID
			result.RequestNodes = append(result.RequestNodes, newRN)

		case mnnode.NODE_KIND_FOR:
			fn, err := c.fns.GetNodeFor(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get for node: %w", err)
			}
			newFN := *fn
			newFN.FlowNodeID = newNodeID
			result.ForNodes = append(result.ForNodes, newFN)

		case mnnode.NODE_KIND_FOR_EACH:
			fen, err := c.fens.GetNodeForEach(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get foreach node: %w", err)
			}
			newFEN := *fen
			newFEN.FlowNodeID = newNodeID
			result.ForEachNodes = append(result.ForEachNodes, newFEN)

		case mnnode.NODE_KIND_CONDITION:
			ifn, err := c.ins.GetNodeIf(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get if node: %w", err)
			}
			newIFN := *ifn
			newIFN.FlowNodeID = newNodeID
			result.IfNodes = append(result.IfNodes, newIFN)

		case mnnode.NODE_KIND_NO_OP:
			nn, err := c.sns.GetNodeNoop(ctx, node.ID)
			if err != nil {
				return result, fmt.Errorf("get noop node: %w", err)
			}
			newNN := *nn
			newNN.FlowNodeID = newNodeID
			result.NoopNodes = append(result.NoopNodes, newNN)
		}
	}

	// Copy edges with new node IDs
	for _, edge := range edges {
		newEdge := edge
		newEdge.ID = idwrap.NewNow()
		newEdge.FlowID = newFlowID
		newEdge.SourceID = oldToNewIDMap[edge.SourceID]
		newEdge.TargetID = oldToNewIDMap[edge.TargetID]
		result.Edges = append(result.Edges, newEdge)
	}

	return result, nil
}

func (c *FlowServiceRPC) CopyFlow(ctx context.Context, tx *sql.Tx, copyData CopyFlowResult) error {
	// Create flow
	txFlow, err := sflow.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create flow service: %w", err)
	}
	err = txFlow.CreateFlow(ctx, copyData.Flow)
	if err != nil {
		return fmt.Errorf("create flow: %w", err)
	}

	// Create nodes
	txNode, err := snode.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create node service: %w", err)
	}
	for _, node := range copyData.Nodes {
		err = txNode.CreateNode(ctx, node)
		if err != nil {
			return fmt.Errorf("create node: %w", err)
		}
	}

	// Create specific node data
	txRequestNode, err := snoderequest.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create request node service: %w", err)
	}
	for _, rn := range copyData.RequestNodes {
		err = txRequestNode.CreateNodeRequest(ctx, rn)
		if err != nil {
			return fmt.Errorf("create request node: %w", err)
		}
	}

	// Similar for other node types...
	txForNode, err := snodefor.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create for node service: %w", err)
	}
	for _, fn := range copyData.ForNodes {
		err = txForNode.CreateNodeFor(ctx, fn)
		if err != nil {
			return fmt.Errorf("create for node: %w", err)
		}
	}

	txForEachNode, err := snodeforeach.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create foreach node service: %w", err)
	}
	for _, fen := range copyData.ForEachNodes {
		err = txForEachNode.CreateNodeForEach(ctx, fen)
		if err != nil {
			return fmt.Errorf("create foreach node: %w", err)
		}
	}

	txIfNode, err := snodeif.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create if node service: %w", err)
	}
	for _, ifn := range copyData.IfNodes {
		err = txIfNode.CreateNodeIf(ctx, ifn)
		if err != nil {
			return fmt.Errorf("create if node: %w", err)
		}
	}

	txNoopNode, err := snodenoop.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create noop node service: %w", err)
	}
	for _, nn := range copyData.NoopNodes {
		err = txNoopNode.CreateNodeNoop(ctx, nn)
		if err != nil {
			return fmt.Errorf("create noop node: %w", err)
		}
	}

	// Create edges
	txEdge, err := sedge.NewTX(ctx, tx)
	if err != nil {
		return fmt.Errorf("create edge service: %w", err)
	}
	for _, edge := range copyData.Edges {
		err = txEdge.CreateEdge(ctx, edge)
		if err != nil {
			return fmt.Errorf("create edge: %w", err)
		}
	}

	return nil
}

func (c *FlowServiceRPC) FlowVersions(ctx context.Context, req *connect.Request[flowv1.FlowVersionsRequest]) (*connect.Response[flowv1.FlowVersionsResponse], error) {
	flowID, err := idwrap.NewFromBytes(req.Msg.FlowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flowID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	flows, err := c.fs.GetFlowsByVersionParentID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	translateFunc := func(flow mflow.Flow) *flowv1.FlowVersionsItem {
		return &flowv1.FlowVersionsItem{
			FlowId: flow.ID.Bytes(),
			Name:   flow.Name,
		}
	}
	translatedFlows := tgeneric.MassConvert(flows, translateFunc)
	resp := &flowv1.FlowVersionsResponse{
		Items: translatedFlows,
	}

	sort.Slice(translatedFlows, func(i, j int) bool {
		return bytes.Compare(translatedFlows[i].FlowId, translatedFlows[j].FlowId) > 0
	})

	return connect.NewResponse(resp), nil
}

func CheckOwnerFlow(ctx context.Context, fs sflow.FlowService, us suser.UserService, flowID idwrap.IDWrap) (bool, error) {
	// TODO: add sql query to make it faster
	flow, err := fs.GetFlow(ctx, flowID)
	if err != nil {
		return false, err
	}
	return rworkspace.CheckOwnerWorkspace(ctx, us, flow.WorkspaceID)
}
