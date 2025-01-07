package rflow

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"the-dev-tools/backend/internal/api"
	"the-dev-tools/backend/internal/api/rworkspace"
	"the-dev-tools/backend/pkg/flow/edge"
	"the-dev-tools/backend/pkg/flow/node"
	"the-dev-tools/backend/pkg/flow/node/nfor"
	"the-dev-tools/backend/pkg/flow/node/nif"
	"the-dev-tools/backend/pkg/flow/node/nrequest"
	"the-dev-tools/backend/pkg/flow/node/nstart"
	"the-dev-tools/backend/pkg/flow/runner"
	"the-dev-tools/backend/pkg/flow/runner/flowlocalrunner"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/logconsole"
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/model/mitemapiexample"
	"the-dev-tools/backend/pkg/model/mnode"
	"the-dev-tools/backend/pkg/model/mnode/mnfor"
	"the-dev-tools/backend/pkg/model/mnode/mnif"
	"the-dev-tools/backend/pkg/model/mnode/mnrequest"
	"the-dev-tools/backend/pkg/model/mnode/mnstart"
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
	"the-dev-tools/backend/pkg/service/snodeif"
	"the-dev-tools/backend/pkg/service/snoderequest"
	"the-dev-tools/backend/pkg/service/snodestart"
	"the-dev-tools/backend/pkg/service/stag"
	"the-dev-tools/backend/pkg/service/suser"
	"the-dev-tools/backend/pkg/service/sworkspace"
	"the-dev-tools/backend/pkg/translate/tflow"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/nodes/pkg/httpclient"
	flowv1 "the-dev-tools/spec/dist/buf/go/flow/v1"
	"the-dev-tools/spec/dist/buf/go/flow/v1/flowv1connect"
	"time"

	"connectrpc.com/connect"
)

type FlowServiceRPC struct {
	DB  *sql.DB
	fs  sflow.FlowService
	ws  sworkspace.WorkspaceService
	us  suser.UserService
	ts  stag.TagService
	fts sflowtag.FlowTagService

	// flow
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
	flns snodefor.NodeForService
	sns  snodestart.NodeStartService
	ins  snodeif.NodeIfService

	logChanMap logconsole.LogChanMap
}

func New(db *sql.DB, ws sworkspace.WorkspaceService,
	us suser.UserService, ts stag.TagService, fs sflow.FlowService, fts sflowtag.FlowTagService,
	fes sedge.EdgeService, as sitemapi.ItemApiService, es sitemapiexample.ItemApiExampleService, qs sexamplequery.ExampleQueryService, hs sexampleheader.HeaderService,
	brs sbodyraw.BodyRawService, bfs sbodyform.BodyFormService, bues sbodyurl.BodyURLEncodedService,
	ns snode.NodeService, rns snoderequest.NodeRequestService, flns snodefor.NodeForService, sns snodestart.NodeStartService, ins snodeif.NodeIfService,
	logChanMap logconsole.LogChanMap,
) FlowServiceRPC {
	return FlowServiceRPC{
		DB:  db,
		fs:  fs,
		ws:  ws,
		us:  us,
		ts:  ts,
		fts: fts,

		// flow
		fes: fes,

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
		flns: flns,
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
	var tagIDPtr *idwrap.IDWrap = nil
	if req.Msg.TagId != nil {
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
	var flows []mflow.Flow

	if tagIDPtr == nil {
		flows, err = c.fs.GetFlowsByWorkspace(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO: can be better with sql query for now it's a workaround
		tag, err := c.ts.GetTag(ctx, *tagIDPtr)
		if err != nil {
			if err == stag.ErrNoTag {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, err
		}
		flowTags, err := c.fts.GetFlowTagsByTagID(ctx, tag.ID)
		if err != nil {
			return nil, err
		}
		flows = make([]mflow.Flow, len(flowTags))
		for i, flowTag := range flowTags {
			flow, err := c.fs.GetFlow(ctx, flowTag.FlowID)
			if err != nil {
				return nil, err
			}

			flows[i] = flow
		}
	}
	rpcResp := &flowv1.FlowListResponse{
		Items: tgeneric.MassConvert(flows, tflow.SeralizeModelToRPCItem),
	}
	return connect.NewResponse(rpcResp), nil
}

func (c *FlowServiceRPC) FlowGet(ctx context.Context, req *connect.Request[flowv1.FlowGetRequest]) (*connect.Response[flowv1.FlowGetResponse], error) {
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
	rpcFlow := flowv1.Flow{
		Name: req.Msg.Name,
	}
	flow := tflow.SeralizeRpcToModelWithoutID(&rpcFlow, workspaceID)
	flowID := idwrap.NewNow()
	flow.ID = flowID
	err = c.fs.CreateFlow(ctx, *flow)
	if err != nil {
		return nil, err
	}

	id, err := idwrap.NewFromBytes(req.Msg.WorkspaceId)
	if err != nil {
		return nil, err
	}

	err = c.ns.CreateNode(ctx, mnode.MNode{
		ID:        id,
		FlowID:    flowID,
		NodeKind:  mnode.NODE_KIND_START,
		PositionX: float64(0),
		PositionY: float64(0),
	})
	if err != nil {
		return nil, err
	}
	err = c.sns.CreateNodeStart(ctx, mnstart.StartNode{
		FlowNodeID: id,
		Name:       "Node1",
	})
	if err != nil {
		return nil, err
	}

	return connect.NewResponse(&flowv1.FlowCreateResponse{
		FlowId: flowID.Bytes(),
	}), nil
}

func (c *FlowServiceRPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[flowv1.FlowUpdateResponse], error) {
	rpcFlow := flowv1.Flow{
		FlowId: req.Msg.FlowId,
		Name:   req.Msg.Name,
	}
	flow, err := tflow.SeralizeRpcToModel(&rpcFlow, idwrap.IDWrap{})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	rpcErr := permcheck.CheckPerm(CheckOwnerFlow(ctx, c.fs, c.us, flow.ID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	err = c.fs.UpdateFlow(ctx, *flow)
	if err != nil {
		return nil, err
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

	nodes, err := c.ns.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get nodes"))
	}

	var forNodes []mnfor.MNFor
	var requestNodes []mnrequest.MNRequest
	var ifNodes []mnif.MNIF
	var startNode *mnstart.StartNode

	for _, node := range nodes {
		switch node.NodeKind {
		case mnode.NODE_KIND_REQUEST:
			rn, err := c.rns.GetNodeRequest(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node request: %w", err))
			}
			requestNodes = append(requestNodes, *rn)
		case mnode.NODE_KIND_FOR:
			fn, err := c.flns.GetNodeFor(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node for: %w", err))
			}
			forNodes = append(forNodes, *fn)
		case mnode.NODE_KIND_START:
			sn, err := c.sns.GetNodeStart(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get node start: %w", err))
			}
			startNode = sn
		case mnode.NODE_KIND_CONDITION:
			in, err := c.ins.GetNodeIf(ctx, node.ID)
			if err != nil {
				return connect.NewError(connect.CodeInternal, errors.New("get node if"))
			}
			ifNodes = append(ifNodes, *in)
		default:
			return connect.NewError(connect.CodeInternal, errors.New("not supported node"))
		}
	}

	if startNode == nil {
		return connect.NewError(connect.CodeInternal, errors.New("no start node"))
	}
	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, 0)
	for _, forNode := range forNodes {
		// TODO: timeout will added
		flowNodeMap[forNode.FlowNodeID] = nfor.New(forNode.FlowNodeID, forNode.Name, forNode.IterCount, time.Second)
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

		// TODO: add body later
		body := []byte{}

		httpClient := httpclient.New()

		flowNodeMap[requestNode.FlowNodeID] = nrequest.New(requestNode.FlowNodeID, *endpoint, *example, queries, headers, body, httpClient)
	}

	for _, ifNode := range ifNodes {
		flowNodeMap[ifNode.FlowNodeID] = nif.New(ifNode.FlowNodeID, ifNode.Name, ifNode.ConditionType, ifNode.Path, ifNode.Value)
	}

	flowNodeMap[startNode.FlowNodeID] = nstart.New(startNode.FlowNodeID, startNode.Name)

	edges, err := c.fes.GetEdgesByFlowID(ctx, flowID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, errors.New("get edges"))
	}
	edgeMap := edge.NewEdgesMap(edges)

	// TODO: get timeout from flow config
	runnerInst := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), flowID, startNode.FlowNodeID, flowNodeMap, edgeMap, time.Second*10)

	status := make(chan runner.FlowStatusResp, 10)

	done := make(chan error)
	go func() {
		for {
			select {
			case a := <-status:
				err = c.logChanMap.SendMsgToUserWithContext(ctx, flowID, a.Log())
				if err != nil {
					done <- err
					return
				}
				var nodeBytes []byte
				if a.CurrentNodeID != nil {
					nodeBytes = a.CurrentNodeID.Bytes()
				}
				err := ctx.Err()
				if err != nil {
					done <- err
					return
				}
				err = stream.Send(&flowv1.FlowRunResponse{
					CurrentNodeId: nodeBytes,
				},
				)
				if err != nil {
					done <- err
					return
				}
				if a.Done() {
					done <- nil
					return
				}
				continue
			case <-ctx.Done():
			}
			break
		}
		done <- nil
	}()

	err = runnerInst.Run(ctx, status)
	if err != nil {
		err = fmt.Errorf("run flow: %w", err)
		return connect.NewError(connect.CodeInternal, err)
	}

	err = <-done
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	return nil
}

func CheckOwnerFlow(ctx context.Context, fs sflow.FlowService, us suser.UserService, flowID idwrap.IDWrap) (bool, error) {
	// TODO: add sql query to make it faster
	flow, err := fs.GetFlow(ctx, flowID)
	if err != nil {
		return false, err
	}
	return rworkspace.CheckOwnerWorkspace(ctx, us, flow.WorkspaceID)
}
