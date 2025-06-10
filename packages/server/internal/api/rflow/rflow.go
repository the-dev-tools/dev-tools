package rflow

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	devtoolsdb "the-dev-tools/db"
	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/ritemapiexample"
	"the-dev-tools/server/internal/api/rtag"
	"the-dev-tools/server/internal/api/rworkspace"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/flow/node"
	"the-dev-tools/server/pkg/flow/node/nfor"
	"the-dev-tools/server/pkg/flow/node/nforeach"
	"the-dev-tools/server/pkg/flow/node/nif"
	"the-dev-tools/server/pkg/flow/node/njs"
	"the-dev-tools/server/pkg/flow/node/nnoop"
	"the-dev-tools/server/pkg/flow/node/nrequest"
	"the-dev-tools/server/pkg/flow/runner"
	"the-dev-tools/server/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/server/pkg/http/request"
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logconsole"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/massertres"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/permcheck"
	"the-dev-tools/server/pkg/reference"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/sassertres"
	"the-dev-tools/server/pkg/service/sbodyform"
	"the-dev-tools/server/pkg/service/sbodyraw"
	"the-dev-tools/server/pkg/service/sbodyurl"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sexampleresp"
	"the-dev-tools/server/pkg/service/sexamplerespheader"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowtag"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/stag"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tflow"
	"the-dev-tools/server/pkg/translate/tflowversion"
	"the-dev-tools/server/pkg/translate/tgeneric"
	nodev1 "the-dev-tools/spec/dist/buf/go/flow/node/v1"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
	"the-dev-tools/spec/dist/buf/go/flow/v1/flowv1connect"
	"the-dev-tools/spec/dist/buf/go/nodejs_executor/v1/nodejs_executorv1connect"
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
	fvs sflowvariable.FlowVariableService

	// request
	ias sitemapi.ItemApiService
	es  sitemapiexample.ItemApiExampleService
	qs  sexamplequery.ExampleQueryService
	hs  sexampleheader.HeaderService

	// body
	brs  sbodyraw.BodyRawService
	bfs  sbodyform.BodyFormService
	bues sbodyurl.BodyURLEncodedService

	// response
	ers  sexampleresp.ExampleRespService
	erhs sexamplerespheader.ExampleRespHeaderService
	as   sassert.AssertService
	ars  sassertres.AssertResultService

	// sub nodes
	ns   snode.NodeService
	rns  snoderequest.NodeRequestService
	fns  snodefor.NodeForService
	fens snodeforeach.NodeForEachService
	sns  snodenoop.NodeNoopService
	ins  snodeif.NodeIfService
	jsns snodejs.NodeJSService

	logChanMap logconsole.LogChanMap
}

func New(db *sql.DB, ws sworkspace.WorkspaceService, us suser.UserService, ts stag.TagService,
	// flow
	fs sflow.FlowService, fts sflowtag.FlowTagService, fes sedge.EdgeService, fvs sflowvariable.FlowVariableService,
	// req
	ias sitemapi.ItemApiService, es sitemapiexample.ItemApiExampleService, qs sexamplequery.ExampleQueryService, hs sexampleheader.HeaderService,
	// body
	brs sbodyraw.BodyRawService, bfs sbodyform.BodyFormService, bues sbodyurl.BodyURLEncodedService,
	// resp
	ers sexampleresp.ExampleRespService, erhs sexamplerespheader.ExampleRespHeaderService, as sassert.AssertService, ars sassertres.AssertResultService,
	// sub nodes
	ns snode.NodeService, rns snoderequest.NodeRequestService, flns snodefor.NodeForService, fens snodeforeach.NodeForEachService,
	sns snodenoop.NodeNoopService, ins snodeif.NodeIfService, jsns snodejs.NodeJSService,
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
		fvs: fvs,

		// request
		ias: ias,
		es:  es,
		qs:  qs,
		hs:  hs,

		// body
		brs:  brs,
		bfs:  bfs,
		bues: bues,

		// resp
		ers:  ers,
		erhs: erhs,
		as:   as,
		ars:  ars,

		// sub nodes
		ns:   ns,
		rns:  rns,
		fns:  flns,
		fens: fens,
		sns:  sns,
		ins:  ins,
		jsns: jsns,

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
	defer devtoolsdb.TxnRollback(tx)

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

	err = c.fs.DeleteFlow(ctx, flowID)
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

	flowVars, err := c.fvs.GetFlowVariablesByFlowID(ctx, flowID)
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
	var jsNodes []mnjs.MNJS
	var startNodeID idwrap.IDWrap

	nodeNameMap := make(map[idwrap.IDWrap]string, len(nodes))

	for _, node := range nodes {
		nodeNameMap[node.ID] = node.Name

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
		case mnnode.NODE_KIND_JS:
			jsn, err := c.jsns.GetNodeJS(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node js: %w", err))
			}
			jsNodes = append(jsNodes, jsn)
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
		name := nodeNameMap[forNode.FlowNodeID]
		flowNodeMap[forNode.FlowNodeID] = nfor.New(forNode.FlowNodeID, name, forNode.IterCount, time.Second)
	}

	requestNodeRespChan := make(chan nrequest.NodeRequestSideResp, len(requestNodes))
	for _, requestNode := range requestNodes {

		// Base Request
		if requestNode.EndpointID == nil || requestNode.ExampleID == nil {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("endpoint or example not found for %s", requestNode.FlowNodeID))
		}
		endpoint, err := c.ias.GetItemApi(ctx, *requestNode.EndpointID)
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

		rawBody, err := c.brs.GetBodyRawByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		formBody, err := c.bfs.GetBodyFormsByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		urlBody, err := c.bues.GetBodyURLEncodedByExampleID(ctx, example.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}

		exampleResp, err := c.ers.GetExampleRespByExampleIDLatest(ctx, example.ID)
		if err != nil {
			if err == sexampleresp.ErrNoRespFound {
				exampleResp = &mexampleresp.ExampleResp{
					ID:        idwrap.NewNow(),
					ExampleID: example.ID,
				}
				err = c.ers.CreateExampleResp(ctx, *exampleResp)
				if err != nil {
					return connect.NewError(connect.CodeInternal, errors.New("create example resp"))
				}
			} else {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		exampleRespHeader, err := c.erhs.GetHeaderByRespID(ctx, exampleResp.ID)
		if err != nil {
			return connect.NewError(connect.CodeInternal, errors.New("get example resp header"))
		}

		asserts, err := c.as.GetAssertByExampleID(ctx, example.ID)
		if err != nil && err != sassert.ErrNoAssertFound {
			return connect.NewError(connect.CodeInternal, err)
		}

		// Delta Request
		if requestNode.DeltaExampleID != nil {
			deltaExample, err := c.es.GetApiExample(ctx, *requestNode.DeltaExampleID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			// Delta Endpoint - overwrite URL if delta endpoint is provided
			if requestNode.DeltaEndpointID != nil {
				deltaEndpoint, err := c.ias.GetItemApi(ctx, *requestNode.DeltaEndpointID)
				if err != nil {
					return connect.NewError(connect.CodeInternal, err)
				}
				endpoint.Url = deltaEndpoint.Url
				endpoint.Method = deltaEndpoint.Method
			}

			deltaHeaders, err := c.hs.GetHeaderByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			deltaQueries, err := c.qs.GetExampleQueriesByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}

			rawBodyDelta, err := c.brs.GetBodyRawByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("delta raw body not found"))
			}

			formBodyDelta, err := c.bfs.GetBodyFormsByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("delta form body not found"))
			}

			urlBodyDelta, err := c.bues.GetBodyURLEncodedByExampleID(ctx, deltaExample.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("delta url body not found"))
			}

			mergeExamplesInput := request.MergeExamplesInput{
				Base:  *example,
				Delta: *deltaExample,

				BaseQueries:  queries,
				DeltaQueries: deltaQueries,

				BaseHeaders:  headers,
				DeltaHeaders: deltaHeaders,

				BaseRawBody:  *rawBody,
				DeltaRawBody: *rawBodyDelta,

				BaseFormBody:  formBody,
				DeltaFormBody: formBodyDelta,

				BaseUrlEncodedBody:  urlBody,
				DeltaUrlEncodedBody: urlBodyDelta,
			}

			mergeExampleOutput := request.MergeExamples(mergeExamplesInput)
			example = &mergeExampleOutput.Merged

			headers = mergeExampleOutput.MergeHeaders
			queries = mergeExampleOutput.MergeQueries

			rawBody = &mergeExampleOutput.MergeRawBody
			formBody = mergeExampleOutput.MergeFormBody
			urlBody = mergeExampleOutput.MergeUrlEncodedBody
		}

		httpClient := httpclient.New()

		name := nodeNameMap[requestNode.FlowNodeID]

		flowNodeMap[requestNode.FlowNodeID] = nrequest.New(requestNode.FlowNodeID, name, *endpoint, *example, queries, headers, *rawBody, formBody, urlBody,
			*exampleResp, exampleRespHeader, asserts, httpClient, requestNodeRespChan)
	}

	for _, ifNode := range ifNodes {
		comp := ifNode.Condition
		name := nodeNameMap[ifNode.FlowNodeID]
		flowNodeMap[ifNode.FlowNodeID] = nif.New(ifNode.FlowNodeID, name, comp)
	}

	for _, noopNode := range noopNodes {
		name := nodeNameMap[noopNode.FlowNodeID]
		flowNodeMap[noopNode.FlowNodeID] = nnoop.New(noopNode.FlowNodeID, name)
	}

	for _, forEachNode := range forEachNodes {
		name := nodeNameMap[forEachNode.FlowNodeID]
		flowNodeMap[forEachNode.FlowNodeID] = nforeach.New(forEachNode.FlowNodeID, name, forEachNode.IterExpression, time.Second,
			forEachNode.Condition, forEachNode.ErrorHandling)
	}

	var clientPtr *nodejs_executorv1connect.NodeJSExecutorServiceClient
	for i, jsNode := range jsNodes {
		if i == 0 {
			client := nodejs_executorv1connect.NewNodeJSExecutorServiceClient(httpclient.New(), "http://localhost:9090")
			clientPtr = &client
		}

		if jsNode.CodeCompressType != compress.CompressTypeNone {
			jsNode.Code, err = compress.Decompress(jsNode.Code, jsNode.CodeCompressType)
			if err != nil {
				return connect.NewError(connect.CodeInternal, err)
			}
		}

		name := nodeNameMap[jsNode.FlowNodeID]

		flowNodeMap[jsNode.FlowNodeID] = njs.New(jsNode.FlowNodeID, name, string(jsNode.Code), *clientPtr)
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
			name := flowNodeStatus.Name
			idStr := id.String()
			stateStr := mnnode.StringNodeState(flowNodeStatus.State)
			if flowNodeStatus.State != mnnode.NODE_STATE_RUNNING {
				go func() {
					ref := reference.NewReferenceFromInterfaceWithKey(flowNodeStatus, name)
					refs := []reference.ReferenceTreeItem{ref}

					localErr := c.logChanMap.SendMsgToUserWithContext(ctx, idwrap.NewNow(), fmt.Sprintf("Node %s:%s: %s", name, idStr, stateStr), refs)
					if localErr != nil {
						done <- localErr
						return
					}
				}()
			}

			// var changes []*changev1.Change

			select {
			// TODO: move invalidation to a separate function so we can test it
			case requestNodeResp := <-requestNodeRespChan:

				err = c.HandleExampleChanges(ctx, requestNodeResp)
				if err != nil {
					log.Println("cannot update example on flow run", err)
				}

				/*
					HistoryChangesService := "collection.item.example.v1.ExampleService"
					HistroyChangesMethod := "ExampleGet"
					exampleGetChangeKind := changev1.ChangeKind_CHANGE_KIND_INVALIDATE

					exampleGetRequest, err := anypb.New(&examplev1.ExampleGetRequest{
						ExampleId: requestNodeResp.Example.ID.Bytes(),
					})
					if err != nil {
						log.Fatal("anypb examplev1.ExampleGetRequest cannot be created")
						return
					}

					changes = append(changes, &changev1.Change{
						Kind:    &exampleGetChangeKind,
						Data:    exampleGetRequest,
						Service: &HistoryChangesService,
						Method:  &HistroyChangesMethod,
					})

					HistroyChangesSubService := "collection.item.response.v1.ResponseService"
					HistroyChangesSubMethod := "ResponseGet"
					responseGetChangeKind := changev1.ChangeKind_CHANGE_KIND_INVALIDATE
					RespRequest, err := anypb.New(&responsev1.ResponseGetRequest{
						ResponseId: requestNodeResp.Resp.ExampleResp.ID.Bytes(),
					})
					if err != nil {
						log.Fatal("anypb responsev1.ResponseGetRequest cannot be created")
						return
					}

					changes = append(changes, &changev1.Change{
						Kind:    &responseGetChangeKind,
						Data:    RespRequest,
						Service: &HistroyChangesSubService,
						Method:  &HistroyChangesSubMethod,
					})
				*/

				example := &flowv1.FlowRunExampleResponse{
					ExampleId:  requestNodeResp.Example.ID.Bytes(),
					ResponseId: requestNodeResp.Resp.ExampleResp.ID.Bytes(),
				}

				resp := &flowv1.FlowRunResponse{
					Example: example,
				}

				localErr := stream.Send(resp)
				if localErr != nil {
					done <- localErr
					return
				}

			default:
			}

			nodeResp := &flowv1.FlowRunNodeResponse{
				NodeId: flowNodeStatus.NodeID.Bytes(),
				State:  nodev1.NodeState(flowNodeStatus.State),
			}

			resp := &flowv1.FlowRunResponse{
				Node: nodeResp,
			}

			data, localErr := json.Marshal(flowNodeStatus.OutputData)
			if localErr != nil {
				done <- err
				return
			}

			nodeStateData := mnnode.MNode{
				ID:        id,
				State:     flowNodeStatus.State,
				StateData: data,
			}
			updateNodeChan <- nodeStateData
			localErr = stream.Send(resp)
			if localErr != nil {
				done <- localErr
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
			case flowNodeStatus, ok := <-flowNodeStatusChan:
				if !ok {
					return
				}
				nodeStatusFunc(flowNodeStatus)
			case flowStatus, ok := <-flowStatusChan:
				if !ok {
					return
				}
				if len(flowNodeStatusChan) > 0 {
					for flowNodeStatus := range flowNodeStatusChan {
						nodeStatusFunc(flowNodeStatus)
					}
				}
				if runner.IsFlowStatusDone(flowStatus) {
					done <- nil
					return
				}
			}
		}
	}()

	// TODO: move into translate packages
	flowVarsMap := make(map[string]any, len(flowVars))
	for _, flowVar := range flowVars {
		if flowVar.Enabled {
			flowVarsMap[flowVar.Name] = flowVar.Value
		}
	}

	flowRunErr := runnerInst.Run(ctx, flowNodeStatusChan, flowStatusChan, flowVarsMap)

	// wait for the flow to finish
	flowErr := <-done

	close(updateNodeChan)
	close(requestNodeRespChan)

	flow.VersionParentID = &flow.ID
	res, err := c.PrepareCopyFlow(ctx, flow.WorkspaceID, flow)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	tx, err := c.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer devtoolsdb.TxnRollback(tx)

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

	err = c.CopyFlow(ctx, tx, res)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	resp := &flowv1.FlowRunResponse{
		Version: tflowversion.ModelToRPC(res.Flow),
	}

	err = stream.Send(resp)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if flowErr != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if flowRunErr != nil {
		return connect.NewError(connect.CodeInternal, flowRunErr)
	}

	return nil
}

func (c *FlowServiceRPC) HandleExampleChanges(ctx context.Context, requestNodeResp nrequest.NodeRequestSideResp) error {
	FullHeaders := append(requestNodeResp.Resp.CreateHeaders, requestNodeResp.Resp.UpdateHeaders...)

	var assertResults []massertres.AssertResult
	var assert []massert.Assert
	for _, assertCouple := range requestNodeResp.Resp.AssertCouples {
		assertResults = append(assertResults, assertCouple.AssertRes)
		assert = append(assert, assertCouple.Assert)
	}

	example := requestNodeResp.Example
	endpoint, err := c.ias.GetItemApi(ctx, example.ItemApiID)
	if err != nil {
		return err
	}

	endpoint.VersionParentID = &endpoint.ID
	endpointNewID := idwrap.NewNow()
	endpoint.ID = endpointNewID

	err = c.ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		return err
	}

	example.VersionParentID = &example.ID

	// TODO: should use same transaction as flow
	tx2, err := c.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer devtoolsdb.TxnRollback(tx2)

	txExampleResp, err := sexampleresp.NewTX(ctx, tx2)
	if err != nil {
		return err
	}

	err = txExampleResp.CreateExampleResp(ctx, requestNodeResp.Resp.ExampleResp)
	if err != nil {
		return err
	}

	txHeaderResp, err := sexamplerespheader.NewTX(ctx, tx2)
	if err != nil {
		return err
	}

	for _, header := range requestNodeResp.Resp.CreateHeaders {
		err = txHeaderResp.CreateExampleRespHeader(ctx, header)
		if err != nil {
			return err
		}
	}
	for _, header := range requestNodeResp.Resp.UpdateHeaders {
		err = txHeaderResp.UpdateExampleRespHeader(ctx, header)
		if err != nil {
			return err
		}
	}
	for _, headerID := range requestNodeResp.Resp.DeleteHeaderIds {
		err = txHeaderResp.DeleteExampleRespHeader(ctx, headerID)
		if err != nil {
			return err
		}
	}

	res, err := ritemapiexample.PrepareCopyExampleNoService(ctx, endpointNewID, example,
		requestNodeResp.Queries, requestNodeResp.Headers, assert,
		&requestNodeResp.RawBody, requestNodeResp.FormBody, requestNodeResp.UrlBody,
		&requestNodeResp.Resp.ExampleResp, FullHeaders, assertResults)
	if err != nil {
		return err
	}

	err = ritemapiexample.CreateCopyExample(ctx, tx2, res)
	if err != nil {
		return err
	}

	err = tx2.Commit()
	if err != nil {
		return err
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

	translatedFlows := tgeneric.MassConvert(flows, tflowversion.ModelToRPC)
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
