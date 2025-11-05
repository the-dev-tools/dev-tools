package rflowv2

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/pkg/compress"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/eventstream"
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
	"the-dev-tools/server/pkg/httpclient"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyform"
	"the-dev-tools/server/pkg/model/mbodyraw"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/model/mexampleresp"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	"the-dev-tools/spec/dist/buf/go/api/flow/v1/flowv1connect"
)

var errUnimplemented = errors.New("rflowv2: method not implemented")

// NodeTopic identifies the flow whose nodes are being published.
type NodeTopic struct {
	FlowID idwrap.IDWrap
}

// NodeEvent describes a node change for sync streaming.
type NodeEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.Node
}

// EdgeTopic identifies the flow whose edges are being published.
type EdgeTopic struct {
	FlowID idwrap.IDWrap
}

// EdgeEvent describes an edge change for sync streaming.
type EdgeEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Edge   *flowv1.Edge
}

const (
	nodeEventCreate = "create"
	nodeEventUpdate = "update"
	nodeEventDelete = "delete"

	edgeEventCreate = "create"
	edgeEventUpdate = "update"
	edgeEventDelete = "delete"
)

type FlowServiceV2RPC struct {
	ws         *sworkspace.WorkspaceService
	fs         *sflow.FlowService
	es         *sedge.EdgeService
	ns         *snode.NodeService
	nrs        *snoderequest.NodeRequestService
	nfs        *snodefor.NodeForService
	nfes       *snodeforeach.NodeForEachService
	nifs       *snodeif.NodeIfService
	nnos       *snodenoop.NodeNoopService
	njss       *snodejs.NodeJSService
	fvs        *sflowvariable.FlowVariableService
	hs         *shttp.HTTPService
	hh         *shttp.HttpHeaderService
	hsp        *shttp.HttpSearchParamService
	hbf        *shttp.HttpBodyFormService
	hbu        *shttp.HttpBodyUrlencodedService
	has        *shttp.HttpAssertService
	nodeStream eventstream.SyncStreamer[NodeTopic, NodeEvent]
	edgeStream eventstream.SyncStreamer[EdgeTopic, EdgeEvent]
}

func New(
	ws *sworkspace.WorkspaceService,
	fs *sflow.FlowService,
	es *sedge.EdgeService,
	ns *snode.NodeService,
	nrs *snoderequest.NodeRequestService,
	nfs *snodefor.NodeForService,
	nfes *snodeforeach.NodeForEachService,
	nifs *snodeif.NodeIfService,
	nnos *snodenoop.NodeNoopService,
	njss *snodejs.NodeJSService,
	fvs *sflowvariable.FlowVariableService,
	hs *shttp.HTTPService,
	hh *shttp.HttpHeaderService,
	hsp *shttp.HttpSearchParamService,
	hbf *shttp.HttpBodyFormService,
	hbu *shttp.HttpBodyUrlencodedService,
	has *shttp.HttpAssertService,
	nodeStream eventstream.SyncStreamer[NodeTopic, NodeEvent],
	edgeStream eventstream.SyncStreamer[EdgeTopic, EdgeEvent],
) *FlowServiceV2RPC {
	return &FlowServiceV2RPC{
		ws:         ws,
		fs:         fs,
		es:         es,
		ns:         ns,
		nrs:        nrs,
		nfs:        nfs,
		nfes:       nfes,
		nifs:       nifs,
		nnos:       nnos,
		njss:       njss,
		fvs:        fvs,
		hs:         hs,
		hh:         hh,
		hsp:        hsp,
		hbf:        hbf,
		hbu:        hbu,
		has:        has,
		nodeStream: nodeStream,
		edgeStream: edgeStream,
	}
}

func CreateService(srv *FlowServiceV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := flowv1connect.NewFlowServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

func (s *FlowServiceV2RPC) FlowCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.FlowCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.Flow, 0, len(flows))
	for _, flow := range flows {
		items = append(items, serializeFlow(flow))
	}

	return connect.NewResponse(&flowv1.FlowCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) EdgeCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.EdgeCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var edgesPB []*flowv1.Edge

	for _, flow := range flows {
		edges, err := s.es.GetEdgesByFlowID(ctx, flow.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, e := range edges {
			edgesPB = append(edgesPB, serializeEdge(e))
		}
	}

	return connect.NewResponse(&flowv1.EdgeCollectionResponse{Items: edgesPB}), nil
}

func (s *FlowServiceV2RPC) NodeCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	var nodesPB []*flowv1.Node
	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, node := range nodes {
			if isStartNode(node) {
				continue
			}
			nodesPB = append(nodesPB, serializeNode(node))
		}
	}

	return connect.NewResponse(&flowv1.NodeCollectionResponse{Items: nodesPB}), nil
}

func (s *FlowServiceV2RPC) NodeHttpCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeHttpCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeHttp, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_REQUEST {
				continue
			}
			nodeReq, err := s.nrs.GetNodeRequest(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeReq == nil || isZeroID(nodeReq.HttpID) {
				continue
			}
			items = append(items, serializeNodeHTTP(*nodeReq))
		}
	}

	return connect.NewResponse(&flowv1.NodeHttpCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) FlowCreate(ctx context.Context, req *connect.Request[flowv1.FlowCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetItems()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one flow is required"))
	}

	workspaceID, err := workspaceIDFromHeaders(req.Header())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	workspace, err := s.ws.Get(ctx, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, item := range req.Msg.GetItems() {
		name := strings.TrimSpace(item.GetName())
		if name == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name is required"))
		}

		flowID := idwrap.NewNow()
		if len(item.GetFlowId()) != 0 {
			flowID, err = idwrap.NewFromBytes(item.GetFlowId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
			}
		}

		flow := mflow.Flow{
			ID:          flowID,
			WorkspaceID: workspaceID,
			Name:        name,
		}

		if err := s.fs.CreateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Seed start node so the flow is immediately runnable.
		startNodeID := idwrap.NewNow()
		startNode := mnnode.MNode{
			ID:        startNodeID,
			FlowID:    flowID,
			Name:      "Start",
			NodeKind:  mnnode.NODE_KIND_NO_OP,
			PositionX: 0,
			PositionY: 0,
		}
		if err := s.ns.CreateNode(ctx, startNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.nnos.CreateNodeNoop(ctx, mnnoop.NoopNode{
			FlowNodeID: startNodeID,
			Type:       mnnoop.NODE_NO_OP_KIND_START,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		workspace.FlowCount++
	}

	workspace.Updated = dbtime.DBNow()
	if err := s.ws.Update(ctx, workspace); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowUpdate(ctx context.Context, req *connect.Request[flowv1.FlowUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		flow, err := s.fs.GetFlow(ctx, flowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		if item.Name != nil {
			flow.Name = strings.TrimSpace(item.GetName())
			if flow.Name == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow name cannot be empty"))
			}
		}

		if du := item.GetDuration(); du != nil {
			switch du.GetKind() {
			case flowv1.FlowUpdate_DurationUnion_KIND_UNSET:
				flow.Duration = 0
			case flowv1.FlowUpdate_DurationUnion_KIND_INT32:
				flow.Duration = du.GetInt32()
			}
		}

		if err := s.fs.UpdateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowDelete(ctx context.Context, req *connect.Request[flowv1.FlowDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		flow, err := s.fs.GetFlow(ctx, flowID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		if err := s.fs.DeleteFlow(ctx, flowID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		workspace, err := s.ws.Get(ctx, flow.WorkspaceID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if workspace.FlowCount > 0 {
			workspace.FlowCount--
		}
		workspace.Updated = dbtime.DBNow()
		if err := s.ws.Update(ctx, workspace); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.FlowSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}

	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return err
	}

	items := buildFlowSyncCreates(flows)
	if len(items) == 0 {
		return nil
	}

	return stream.Send(&flowv1.FlowSyncResponse{Items: items})
}

func (s *FlowServiceV2RPC) FlowRun(ctx context.Context, req *connect.Request[flowv1.FlowRunRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}

	flowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	flow, err := s.fs.GetFlow(ctx, flowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	nodes, err := s.ns.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	edges, err := s.es.GetEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	flowVars, err := s.fvs.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil && !errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	baseVars := make(map[string]any, len(flowVars))
	for _, variable := range flowVars {
		if variable.Enabled {
			baseVars[variable.Name] = variable.Value
		}
	}

	requestRespChan := make(chan nrequest.NodeRequestSideResp, len(nodes)*2+1)
	var respDrain sync.WaitGroup
	respDrain.Add(1)
	go func() {
		defer respDrain.Done()
		for range requestRespChan {
		}
	}()
	defer func() {
		close(requestRespChan)
		respDrain.Wait()
	}()

	sharedHTTPClient := httpclient.New()
	edgeMap := edge.NewEdgesMap(edges)
	flowNodeMap := make(map[idwrap.IDWrap]node.FlowNode, len(nodes))

	var startNodeID idwrap.IDWrap
	const defaultNodeTimeout = 60 // seconds
	timeoutDuration := time.Duration(defaultNodeTimeout) * time.Second

	for _, nodeModel := range nodes {
		switch nodeModel.NodeKind {
		case mnnode.NODE_KIND_NO_OP:
			noopModel, err := s.nnos.GetNodeNoop(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowNodeMap[nodeModel.ID] = nnoop.New(nodeModel.ID, nodeModel.Name)
			if noopModel.Type == mnnoop.NODE_NO_OP_KIND_START {
				startNodeID = nodeModel.ID
			}
		case mnnode.NODE_KIND_REQUEST:
			requestCfg, err := s.nrs.GetNodeRequest(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if requestCfg == nil || isZeroID(requestCfg.HttpID) {
				return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("request node %s missing http configuration", nodeModel.ID.String()))
			}
			requestNode, err := s.buildRequestFlowNode(ctx, flow, nodeModel, *requestCfg, sharedHTTPClient, requestRespChan)
			if err != nil {
				return nil, err
			}
			flowNodeMap[nodeModel.ID] = requestNode
		case mnnode.NODE_KIND_FOR:
			forCfg, err := s.nfs.GetNodeFor(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if forCfg.Condition.Comparisons.Expression != "" {
				flowNodeMap[nodeModel.ID] = nfor.NewWithCondition(nodeModel.ID, nodeModel.Name, forCfg.IterCount, timeoutDuration, forCfg.ErrorHandling, forCfg.Condition)
			} else {
				flowNodeMap[nodeModel.ID] = nfor.New(nodeModel.ID, nodeModel.Name, forCfg.IterCount, timeoutDuration, forCfg.ErrorHandling)
			}
		case mnnode.NODE_KIND_FOR_EACH:
			forEachCfg, err := s.nfes.GetNodeForEach(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowNodeMap[nodeModel.ID] = nforeach.New(nodeModel.ID, nodeModel.Name, forEachCfg.IterExpression, timeoutDuration, forEachCfg.Condition, forEachCfg.ErrorHandling)
		case mnnode.NODE_KIND_CONDITION:
			condCfg, err := s.nifs.GetNodeIf(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			flowNodeMap[nodeModel.ID] = nif.New(nodeModel.ID, nodeModel.Name, condCfg.Condition)
		case mnnode.NODE_KIND_JS:
			jsCfg, err := s.njss.GetNodeJS(ctx, nodeModel.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			codeBytes := jsCfg.Code
			if jsCfg.CodeCompressType != compress.CompressTypeNone {
				codeBytes, err = compress.Decompress(jsCfg.Code, jsCfg.CodeCompressType)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("decompress js code: %w", err))
				}
			}
			flowNodeMap[nodeModel.ID] = njs.New(nodeModel.ID, nodeModel.Name, string(codeBytes), nil)
		default:
			return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("node kind %d not supported in FlowRun", nodeModel.NodeKind))
		}
	}

	if startNodeID == (idwrap.IDWrap{}) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("flow missing start node"))
	}

	flowRunner := flowlocalrunner.CreateFlowRunner(idwrap.NewNow(), flow.ID, startNodeID, flowNodeMap, edgeMap, 0, nil)
	if err := flowRunner.RunWithEvents(ctx, runner.FlowEventChannels{}, baseVars); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVersionCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVersionCollectionResponse], error) {
	flowID, err := flowIDFromHeaders(req.Header())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	versions, err := s.fs.GetFlowsByVersionParentID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	items := make([]*flowv1.FlowVersion, 0, len(versions))
	for _, version := range versions {
		items = append(items, &flowv1.FlowVersion{
			FlowVersionId: version.ID.Bytes(),
			FlowId:        flowID.Bytes(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return bytes.Compare(items[i].GetFlowVersionId(), items[j].GetFlowVersionId()) < 0
	})

	return connect.NewResponse(&flowv1.FlowVersionCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) FlowVersionSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.FlowVersionSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowVariableCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVariableCollectionResponse], error) {
	flowID, err := flowIDFromHeaders(req.Header())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.ensureFlowAccess(ctx, flowID); err != nil {
		return nil, err
	}

	variables, err := s.fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
	if err != nil {
		if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
			variables = nil
		} else {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	items := make([]*flowv1.FlowVariable, 0, len(variables))
	for idx, variable := range variables {
		items = append(items, serializeFlowVariable(variable, float32(idx)))
	}

	return connect.NewResponse(&flowv1.FlowVariableCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) FlowVariableCreate(ctx context.Context, req *connect.Request[flowv1.FlowVariableCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}

		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		variableID := idwrap.NewNow()
		if len(item.GetFlowVariableId()) != 0 {
			variableID, err = idwrap.NewFromBytes(item.GetFlowVariableId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow variable id: %w", err))
			}
		}

		key := strings.TrimSpace(item.GetKey())
		if key == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable key is required"))
		}

		variable := mflowvariable.FlowVariable{
			ID:          variableID,
			FlowID:      flowID,
			Name:        key,
			Value:       item.GetValue(),
			Enabled:     item.GetEnabled(),
			Description: item.GetDescription(),
		}

		if err := s.fvs.CreateFlowVariable(ctx, variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableUpdate(ctx context.Context, req *connect.Request[flowv1.FlowVariableUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowVariableId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable id is required"))
		}

		variableID, err := idwrap.NewFromBytes(item.GetFlowVariableId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow variable id: %w", err))
		}

		variable, err := s.fvs.GetFlowVariable(ctx, variableID)
		if err != nil {
			if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow variable %s not found", variableID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, variable.FlowID); err != nil {
			return nil, err
		}

		if len(item.GetFlowId()) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow reassignment is not supported"))
		}

		if item.Key != nil {
			key := strings.TrimSpace(item.GetKey())
			if key == "" {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable key cannot be empty"))
			}
			variable.Name = key
		}

		if item.Value != nil {
			variable.Value = item.GetValue()
		}

		if item.Enabled != nil {
			variable.Enabled = item.GetEnabled()
		}

		if item.Description != nil {
			variable.Description = item.GetDescription()
		}

		if item.Order != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow variable order updates are not supported"))
		}

		if err := s.fvs.UpdateFlowVariable(ctx, variable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableDelete(ctx context.Context, req *connect.Request[flowv1.FlowVariableDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		variableID, err := idwrap.NewFromBytes(item.GetFlowVariableId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow variable id: %w", err))
		}

		variable, err := s.fvs.GetFlowVariable(ctx, variableID)
		if err != nil {
			if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.ensureFlowAccess(ctx, variable.FlowID); err != nil {
			return nil, err
		}

		if err := s.fvs.DeleteFlowVariable(ctx, variableID); err != nil && !errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.FlowVariableSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) EdgeCreate(ctx context.Context, req *connect.Request[flowv1.EdgeCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		if len(item.GetFlowId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
		}
		if len(item.GetSourceId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source id is required"))
		}
		if len(item.GetTargetId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("target id is required"))
		}

		flowID, err := idwrap.NewFromBytes(item.GetFlowId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
		}
		if err := s.ensureFlowAccess(ctx, flowID); err != nil {
			return nil, err
		}

		sourceID, err := idwrap.NewFromBytes(item.GetSourceId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid source id: %w", err))
		}
		if _, err := s.ensureNodeAccess(ctx, sourceID); err != nil {
			return nil, err
		}

		targetID, err := idwrap.NewFromBytes(item.GetTargetId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target id: %w", err))
		}
		if _, err := s.ensureNodeAccess(ctx, targetID); err != nil {
			return nil, err
		}

		edgeID := idwrap.NewNow()
		if len(item.GetEdgeId()) != 0 {
			edgeID, err = idwrap.NewFromBytes(item.GetEdgeId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
			}
		}

		model := edge.Edge{
			ID:            edgeID,
			FlowID:        flowID,
			SourceID:      sourceID,
			TargetID:      targetID,
			SourceHandler: convertHandle(item.GetSourceHandle()),
			Kind:          int32(item.GetKind()),
		}

		if err := s.es.CreateEdge(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishEdgeEvent(edgeEventCreate, model)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) EdgeUpdate(ctx context.Context, req *connect.Request[flowv1.EdgeUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		edgeID, err := idwrap.NewFromBytes(item.GetEdgeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
		}

		existing, err := s.ensureEdgeAccess(ctx, edgeID)
		if err != nil {
			return nil, err
		}

		if len(item.GetFlowId()) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow reassignment is not supported"))
		}

		if len(item.GetSourceId()) != 0 {
			sourceID, err := idwrap.NewFromBytes(item.GetSourceId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid source id: %w", err))
			}
			if _, err := s.ensureNodeAccess(ctx, sourceID); err != nil {
				return nil, err
			}
			existing.SourceID = sourceID
		}

		if len(item.GetTargetId()) != 0 {
			targetID, err := idwrap.NewFromBytes(item.GetTargetId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid target id: %w", err))
			}
			if _, err := s.ensureNodeAccess(ctx, targetID); err != nil {
				return nil, err
			}
			existing.TargetID = targetID
		}

		if item.SourceHandle != nil {
			existing.SourceHandler = convertHandle(item.GetSourceHandle())
		}

		if item.Kind != nil {
			existing.Kind = int32(item.GetKind())
		}

		if err := s.es.UpdateEdge(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishEdgeEvent(edgeEventUpdate, *existing)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) EdgeDelete(ctx context.Context, req *connect.Request[flowv1.EdgeDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		edgeID, err := idwrap.NewFromBytes(item.GetEdgeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid edge id: %w", err))
		}

		existing, err := s.ensureEdgeAccess(ctx, edgeID)
		if err != nil {
			return nil, err
		}

		if err := s.es.DeleteEdge(ctx, edgeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if existing != nil {
			s.publishEdgeEvent(edgeEventDelete, *existing)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) EdgeSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.EdgeSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamEdgeSync(ctx, func(resp *flowv1.EdgeSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamEdgeSync(
	ctx context.Context,
	send func(*flowv1.EdgeSyncResponse) error,
) error {
	if s.edgeStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("edge stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[EdgeTopic, EdgeEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[EdgeTopic, EdgeEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			edges, err := s.es.GetEdgesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, edgeModel := range edges {
				events = append(events, eventstream.Event[EdgeTopic, EdgeEvent]{
					Topic: EdgeTopic{FlowID: flow.ID},
					Payload: EdgeEvent{
						Type:   edgeEventCreate,
						FlowID: flow.ID,
						Edge:   serializeEdge(edgeModel),
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic EdgeTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.edgeStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := edgeEventToSyncResponse(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *FlowServiceV2RPC) publishNodeEvent(eventType string, model mnnode.MNode) {
	if s.nodeStream == nil {
		return
	}
	if isStartNode(model) {
		return
	}
	nodePB := serializeNode(model)
	s.nodeStream.Publish(NodeTopic{FlowID: model.FlowID}, NodeEvent{
		Type:   eventType,
		FlowID: model.FlowID,
		Node:   nodePB,
	})
}

func (s *FlowServiceV2RPC) publishEdgeEvent(eventType string, model edge.Edge) {
	if s.edgeStream == nil {
		return
	}
	edgePB := serializeEdge(model)
	s.edgeStream.Publish(EdgeTopic{FlowID: model.FlowID}, EdgeEvent{
		Type:   eventType,
		FlowID: model.FlowID,
		Edge:   edgePB,
	})
}

func (s *FlowServiceV2RPC) NodeCreate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeCreateRequest],
) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeModel, err := s.deserializeNodeCreate(item)
		if err != nil {
			return nil, err
		}

		if err := s.ensureFlowAccess(ctx, nodeModel.FlowID); err != nil {
			return nil, err
		}

		if err := s.ns.CreateNode(ctx, *nodeModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishNodeEvent(nodeEventCreate, *nodeModel)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeUpdate(
	ctx context.Context,
	req *connect.Request[flowv1.NodeUpdateRequest],
) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		existing, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		if item.Kind != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node kind updates are not supported"))
		}
		if len(item.GetFlowId()) != 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node flow reassignment is not supported"))
		}

		if item.Name != nil {
			existing.Name = item.GetName()
		}

		if item.Position != nil {
			existing.PositionX = float64(item.Position.GetX())
			existing.PositionY = float64(item.Position.GetY())
		}

		if err := s.ns.UpdateNode(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishNodeEvent(nodeEventUpdate, *existing)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeDelete(
	ctx context.Context,
	req *connect.Request[flowv1.NodeDeleteRequest],
) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		existing, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		if err := s.ns.DeleteNode(ctx, nodeID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishNodeEvent(nodeEventDelete, *existing)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeSync(ctx, func(resp *flowv1.NodeSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeSync(
	ctx context.Context,
	send func(*flowv1.NodeSyncResponse) error,
) error {
	if s.nodeStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("node stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[NodeTopic, NodeEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[NodeTopic, NodeEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, nodeModel := range nodes {
				if isStartNode(nodeModel) {
					continue
				}
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventCreate,
						FlowID: flow.ID,
						Node:   serializeNode(nodeModel),
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic NodeTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.nodeStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := nodeEventToSyncResponse(evt.Payload)
			if resp == nil {
				continue
			}
			if err := send(resp); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *FlowServiceV2RPC) NodeNoOpCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeNoOpCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeNoOpCreate(ctx context.Context, req *connect.Request[flowv1.NodeNoOpCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		noop := mnnoop.NoopNode{
			FlowNodeID: nodeID,
			Type:       mnnoop.NoopTypes(item.GetKind()),
		}
		if err := s.nnos.CreateNodeNoop(ctx, noop); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpUpdate(ctx context.Context, req *connect.Request[flowv1.NodeNoOpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if item.Kind == nil {
			continue
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		noop := mnnoop.NoopNode{
			FlowNodeID: nodeID,
			Type:       mnnoop.NoopTypes(item.GetKind()),
		}
		if err := s.nnos.CreateNodeNoop(ctx, noop); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpDelete(ctx context.Context, req *connect.Request[flowv1.NodeNoOpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeNoOpSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeHttpCreate(ctx context.Context, req *connect.Request[flowv1.NodeHttpCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if len(item.GetHttpId()) == 0 {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("http id is required"))
		}

		httpID, err := idwrap.NewFromBytes(item.GetHttpId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
		}

		if err := s.nrs.CreateNodeRequest(ctx, mnrequest.MNRequest{
			FlowNodeID:       nodeID,
			HttpID:           httpID,
			HasRequestConfig: !isZeroID(httpID),
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpUpdate(ctx context.Context, req *connect.Request[flowv1.NodeHttpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		var httpID idwrap.IDWrap
		if len(item.GetHttpId()) != 0 {
			httpID, err = idwrap.NewFromBytes(item.GetHttpId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
			}
		}

		if err := s.nrs.UpdateNodeRequest(ctx, mnrequest.MNRequest{
			FlowNodeID:       nodeID,
			HttpID:           httpID,
			HasRequestConfig: !isZeroID(httpID),
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpDelete(ctx context.Context, req *connect.Request[flowv1.NodeHttpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nrs.DeleteNodeRequest(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeHttpSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeHttpSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeForCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForCreate(ctx context.Context, req *connect.Request[flowv1.NodeForCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		model := mnfor.MNFor{
			FlowNodeID:    nodeID,
			IterCount:     int64(item.GetIterations()),
			Condition:     buildCondition(item.GetCondition()),
			ErrorHandling: mnfor.ErrorHandling(item.GetErrorHandling()),
		}

		if err := s.nfs.CreateNodeFor(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.nfs.GetNodeFor(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have FOR config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Iterations != nil {
			existing.IterCount = int64(item.GetIterations())
		}
		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}
		if item.ErrorHandling != nil {
			existing.ErrorHandling = mnfor.ErrorHandling(item.GetErrorHandling())
		}

		if err := s.nfs.UpdateNodeFor(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForDelete(ctx context.Context, req *connect.Request[flowv1.NodeForDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nfs.DeleteNodeFor(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeForSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeForEachCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeForEachCreate(ctx context.Context, req *connect.Request[flowv1.NodeForEachCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		model := mnforeach.MNForEach{
			FlowNodeID:     nodeID,
			IterExpression: item.GetPath(),
			Condition:      buildCondition(item.GetCondition()),
			ErrorHandling:  mnfor.ErrorHandling(item.GetErrorHandling()),
		}

		if err := s.nfes.CreateNodeForEach(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForEachUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.nfes.GetNodeForEach(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have FOREACH config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Path != nil {
			existing.IterExpression = item.GetPath()
		}
		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}
		if item.ErrorHandling != nil {
			existing.ErrorHandling = mnfor.ErrorHandling(item.GetErrorHandling())
		}

		if err := s.nfes.UpdateNodeForEach(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachDelete(ctx context.Context, req *connect.Request[flowv1.NodeForEachDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nfes.DeleteNodeForEach(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForEachSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeForEachSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeConditionCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeConditionCreate(ctx context.Context, req *connect.Request[flowv1.NodeConditionCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		model := mnif.MNIF{
			FlowNodeID: nodeID,
			Condition:  buildCondition(item.GetCondition()),
		}

		if err := s.nifs.CreateNodeIf(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionUpdate(ctx context.Context, req *connect.Request[flowv1.NodeConditionUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.nifs.GetNodeIf(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have CONDITION config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Condition != nil {
			existing.Condition = buildCondition(item.GetCondition())
		}

		if err := s.nifs.UpdateNodeIf(ctx, *existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionDelete(ctx context.Context, req *connect.Request[flowv1.NodeConditionDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.nifs.DeleteNodeIf(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeConditionSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeConditionSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeJsCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeJsCreate(ctx context.Context, req *connect.Request[flowv1.NodeJsCreateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		model := mnjs.MNJS{
			FlowNodeID:       nodeID,
			Code:             []byte(item.GetCode()),
			CodeCompressType: compress.CompressTypeNone,
		}

		if err := s.njss.CreateNodeJS(ctx, model); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsUpdate(ctx context.Context, req *connect.Request[flowv1.NodeJsUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		existing, err := s.njss.GetNodeJS(ctx, nodeID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s does not have JS config", nodeID.String()))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if item.Code != nil {
			existing.Code = []byte(item.GetCode())
		}

		if err := s.njss.UpdateNodeJS(ctx, existing); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsDelete(ctx context.Context, req *connect.Request[flowv1.NodeJsDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		if err := s.njss.DeleteNodeJS(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeJsSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeJsSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeExecutionCollection(context.Context, *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.NodeExecutionCollectionResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) NodeExecutionSync(context.Context, *connect.Request[emptypb.Empty], *connect.ServerStream[flowv1.NodeExecutionSyncResponse]) error {
	return connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) listAccessibleFlows(ctx context.Context) ([]mflow.Flow, error) {
	workspaces, err := s.listUserWorkspaces(ctx)
	if err != nil {
		return nil, err
	}

	var allFlows []mflow.Flow
	for _, ws := range workspaces {
		flows, err := s.fs.GetFlowsByWorkspaceID(ctx, ws.ID)
		if err != nil {
			if errors.Is(err, sflow.ErrNoFlowFound) {
				continue
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		allFlows = append(allFlows, flows...)
	}
	return allFlows, nil
}

func (s *FlowServiceV2RPC) listUserWorkspaces(ctx context.Context) ([]mworkspace.Workspace, error) {
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}

	workspaces, err := s.ws.GetWorkspacesByUserIDOrdered(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return workspaces, nil
}

func buildFlowSyncCreates(flows []mflow.Flow) []*flowv1.FlowSync {
	if len(flows) == 0 {
		return nil
	}

	sort.Slice(flows, func(i, j int) bool {
		return bytes.Compare(flows[i].ID.Bytes(), flows[j].ID.Bytes()) < 0
	})

	items := make([]*flowv1.FlowSync, 0, len(flows))
	for _, flow := range flows {
		create := &flowv1.FlowSyncCreate{
			FlowId: flow.ID.Bytes(),
			Name:   flow.Name,
		}
		if flow.Duration != 0 {
			duration := flow.Duration
			create.Duration = &duration
		}

		items = append(items, &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind:   flowv1.FlowSync_ValueUnion_KIND_CREATE,
				Create: create,
			},
		})
	}

	return items
}

func nodeEventToSyncResponse(evt NodeEvent) *flowv1.NodeSyncResponse {
	if evt.Node == nil {
		return nil
	}

	node := evt.Node

	if strings.EqualFold(node.GetName(), "start") && node.GetKind() == flowv1.NodeKind_NODE_KIND_NO_OP {
		return nil
	}

	switch evt.Type {
	case nodeEventCreate:
		create := &flowv1.NodeSyncCreate{
			NodeId:   node.GetNodeId(),
			FlowId:   node.GetFlowId(),
			Kind:     node.GetKind(),
			Name:     node.GetName(),
			Position: node.GetPosition(),
			State:    node.GetState(),
		}
		if info := node.GetInfo(); info != "" {
			create.Info = &info
		}
		return &flowv1.NodeSyncResponse{
			Items: []*flowv1.NodeSync{{
				Value: &flowv1.NodeSync_ValueUnion{
					Kind:   flowv1.NodeSync_ValueUnion_KIND_CREATE,
					Create: create,
				},
			}},
		}
	case nodeEventUpdate:
		update := &flowv1.NodeSyncUpdate{
			NodeId: node.GetNodeId(),
		}
		if flowID := node.GetFlowId(); len(flowID) > 0 {
			update.FlowId = flowID
		}
		if kind := node.GetKind(); kind != flowv1.NodeKind_NODE_KIND_UNSPECIFIED {
			k := kind
			update.Kind = &k
		}
		if name := node.GetName(); name != "" {
			update.Name = &name
		}
		if pos := node.GetPosition(); pos != nil {
			update.Position = pos
		}
		if state := node.GetState(); state != flowv1.NodeState_NODE_STATE_UNSPECIFIED {
			st := state
			update.State = &st
		}
		if info := node.GetInfo(); info != "" {
			update.Info = &flowv1.NodeSyncUpdate_InfoUnion{
				Kind:    flowv1.NodeSyncUpdate_InfoUnion_KIND_STRING,
				String_: &info,
			}
		}
		return &flowv1.NodeSyncResponse{
			Items: []*flowv1.NodeSync{{
				Value: &flowv1.NodeSync_ValueUnion{
					Kind:   flowv1.NodeSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case nodeEventDelete:
		return &flowv1.NodeSyncResponse{
			Items: []*flowv1.NodeSync{{
				Value: &flowv1.NodeSync_ValueUnion{
					Kind: flowv1.NodeSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.NodeSyncDelete{
						NodeId: node.GetNodeId(),
					},
				},
			}},
		}
	default:
		return nil
	}
}

func edgeEventToSyncResponse(evt EdgeEvent) *flowv1.EdgeSyncResponse {
	if evt.Edge == nil {
		return nil
	}

	edgePB := evt.Edge

	switch evt.Type {
	case edgeEventCreate:
		create := &flowv1.EdgeSyncCreate{
			EdgeId:       edgePB.GetEdgeId(),
			FlowId:       edgePB.GetFlowId(),
			Kind:         edgePB.GetKind(),
			SourceId:     edgePB.GetSourceId(),
			TargetId:     edgePB.GetTargetId(),
			SourceHandle: edgePB.GetSourceHandle(),
		}
		return &flowv1.EdgeSyncResponse{
			Items: []*flowv1.EdgeSync{{
				Value: &flowv1.EdgeSync_ValueUnion{
					Kind:   flowv1.EdgeSync_ValueUnion_KIND_CREATE,
					Create: create,
				},
			}},
		}
	case edgeEventUpdate:
		update := &flowv1.EdgeSyncUpdate{
			EdgeId: edgePB.GetEdgeId(),
		}
		if flowID := edgePB.GetFlowId(); len(flowID) > 0 {
			update.FlowId = flowID
		}
		if kind := edgePB.GetKind(); kind != flowv1.EdgeKind_EDGE_KIND_UNSPECIFIED {
			k := kind
			update.Kind = &k
		}
		if sourceID := edgePB.GetSourceId(); len(sourceID) > 0 {
			update.SourceId = sourceID
		}
		if targetID := edgePB.GetTargetId(); len(targetID) > 0 {
			update.TargetId = targetID
		}
		if handle := edgePB.GetSourceHandle(); handle != flowv1.Handle_HANDLE_UNSPECIFIED {
			h := handle
			update.SourceHandle = &h
		}
		return &flowv1.EdgeSyncResponse{
			Items: []*flowv1.EdgeSync{{
				Value: &flowv1.EdgeSync_ValueUnion{
					Kind:   flowv1.EdgeSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case edgeEventDelete:
		return &flowv1.EdgeSyncResponse{
			Items: []*flowv1.EdgeSync{{
				Value: &flowv1.EdgeSync_ValueUnion{
					Kind: flowv1.EdgeSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.EdgeSyncDelete{
						EdgeId: edgePB.GetEdgeId(),
					},
				},
			}},
		}
	default:
		return nil
	}
}

func isStartNode(node mnnode.MNode) bool {
	if node.NodeKind != mnnode.NODE_KIND_NO_OP {
		return false
	}
	return strings.EqualFold(node.Name, "start")
}

func (s *FlowServiceV2RPC) buildRequestFlowNode(
	ctx context.Context,
	flow mflow.Flow,
	nodeModel mnnode.MNode,
	cfg mnrequest.MNRequest,
	client httpclient.HttpClient,
	respChan chan nrequest.NodeRequestSideResp,
) (*nrequest.NodeRequest, error) {
	httpRecord, err := s.hs.Get(ctx, cfg.HttpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load http %s: %w", cfg.HttpID.String(), err))
	}

	exampleID := cfg.HttpID
	endpoint := mitemapi.ItemApi{
		ID:           httpRecord.ID,
		CollectionID: flow.WorkspaceID,
		Name:         httpRecord.Name,
		Url:          httpRecord.Url,
		Method:       httpRecord.Method,
	}

	var headers []mhttp.HTTPHeader
	if s.hh != nil {
		headers, err = s.hh.GetByHttpID(ctx, cfg.HttpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load http headers: %w", err))
		}
	}

	var queries []mhttp.HTTPSearchParam
	if s.hsp != nil {
		queries, err = s.hsp.GetByHttpID(ctx, cfg.HttpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load http queries: %w", err))
		}
	}

	var forms []mhttp.HTTPBodyForm
	if s.hbf != nil {
		forms, err = s.hbf.GetByHttpID(ctx, cfg.HttpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load http body forms: %w", err))
		}
	}

	var urlEncoded []*mhttp.HTTPBodyUrlencoded
	if s.hbu != nil {
		urlEncoded, err = s.hbu.List(ctx, cfg.HttpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load http body urlencoded: %w", err))
		}
	}

	headersOld := convertHTTPHeadersToExampleHeaders(headers, exampleID)
	queriesOld := convertHTTPQueriesToExampleQueries(queries, exampleID)
	formsOld := convertHTTPFormsToExample(forms, exampleID)
	urlEncodedOld := convertHTTPUrlencodedToExample(urlEncoded, exampleID)

	rawBody := mbodyraw.ExampleBodyRaw{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
		Data:      nil,
	}

	example := mitemapiexample.ItemApiExample{
		ID:        exampleID,
		ItemApiID: endpoint.ID,
		Name:      nodeModel.Name,
		BodyType:  determineExampleBodyType(formsOld, urlEncodedOld),
	}

	exampleResp := mexampleresp.ExampleResp{
		ID:        idwrap.NewNow(),
		ExampleID: exampleID,
	}

	requestNode := nrequest.New(
		nodeModel.ID,
		nodeModel.Name,
		endpoint,
		example,
		queriesOld,
		headersOld,
		rawBody,
		formsOld,
		urlEncodedOld,
		exampleResp,
		nil,
		nil,
		client,
		respChan,
		nil,
	)
	return requestNode, nil
}

func convertHTTPHeadersToExampleHeaders(headers []mhttp.HTTPHeader, exampleID idwrap.IDWrap) []mexampleheader.Header {
	if len(headers) == 0 {
		return nil
	}
	result := make([]mexampleheader.Header, 0, len(headers))
	for _, header := range headers {
		result = append(result, mexampleheader.Header{
			ID:            header.ID,
			ExampleID:     exampleID,
			HeaderKey:     header.HeaderKey,
			Value:         header.HeaderValue,
			Description:   header.Description,
			Enable:        header.Enabled,
			DeltaParentID: header.ParentHeaderID,
			Prev:          header.Prev,
			Next:          header.Next,
		})
	}
	return result
}

func convertHTTPQueriesToExampleQueries(queries []mhttp.HTTPSearchParam, exampleID idwrap.IDWrap) []mexamplequery.Query {
	if len(queries) == 0 {
		return nil
	}
	result := make([]mexamplequery.Query, 0, len(queries))
	for _, query := range queries {
		result = append(result, mexamplequery.Query{
			ID:            query.ID,
			ExampleID:     exampleID,
			QueryKey:      query.ParamKey,
			Value:         query.ParamValue,
			Description:   query.Description,
			Enable:        query.Enabled,
			DeltaParentID: query.ParentSearchParamID,
		})
	}
	return result
}

func convertHTTPFormsToExample(forms []mhttp.HTTPBodyForm, exampleID idwrap.IDWrap) []mbodyform.BodyForm {
	if len(forms) == 0 {
		return nil
	}
	result := make([]mbodyform.BodyForm, 0, len(forms))
	for _, form := range forms {
		result = append(result, mbodyform.BodyForm{
			ID:            form.ID,
			ExampleID:     exampleID,
			BodyKey:       form.FormKey,
			Value:         form.FormValue,
			Description:   form.Description,
			Enable:        form.Enabled,
			DeltaParentID: form.ParentBodyFormID,
		})
	}
	return result
}

func convertHTTPUrlencodedToExample(items []*mhttp.HTTPBodyUrlencoded, exampleID idwrap.IDWrap) []mbodyurl.BodyURLEncoded {
	if len(items) == 0 {
		return nil
	}
	result := make([]mbodyurl.BodyURLEncoded, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		result = append(result, mbodyurl.BodyURLEncoded{
			ID:            item.ID,
			ExampleID:     exampleID,
			BodyKey:       item.UrlencodedKey,
			Value:         item.UrlencodedValue,
			Description:   item.Description,
			Enable:        item.Enabled,
			DeltaParentID: item.ParentBodyUrlencodedID,
		})
	}
	return result
}

func determineExampleBodyType(forms []mbodyform.BodyForm, urlEncoded []mbodyurl.BodyURLEncoded) mitemapiexample.BodyType {
	switch {
	case len(forms) > 0:
		return mitemapiexample.BodyTypeForm
	case len(urlEncoded) > 0:
		return mitemapiexample.BodyTypeUrlencoded
	default:
		return mitemapiexample.BodyTypeRaw
	}
}

func serializeFlow(flow mflow.Flow) *flowv1.Flow {
	msg := &flowv1.Flow{
		FlowId: flow.ID.Bytes(),
		Name:   flow.Name,
	}
	if flow.Duration != 0 {
		duration := flow.Duration
		msg.Duration = &duration
	}
	return msg
}

func serializeEdge(e edge.Edge) *flowv1.Edge {
	return &flowv1.Edge{
		EdgeId:       e.ID.Bytes(),
		FlowId:       e.FlowID.Bytes(),
		Kind:         flowv1.EdgeKind(e.Kind),
		SourceId:     e.SourceID.Bytes(),
		TargetId:     e.TargetID.Bytes(),
		SourceHandle: flowv1.Handle(e.SourceHandler),
	}
}

func serializeNode(n mnnode.MNode) *flowv1.Node {
	position := &flowv1.Position{
		X: float32(n.PositionX),
		Y: float32(n.PositionY),
	}

	return &flowv1.Node{
		NodeId:   n.ID.Bytes(),
		FlowId:   n.FlowID.Bytes(),
		Kind:     flowv1.NodeKind(n.NodeKind),
		Name:     n.Name,
		Position: position,
		State:    flowv1.NodeState_NODE_STATE_UNSPECIFIED,
	}
}

func serializeNodeHTTP(n mnrequest.MNRequest) *flowv1.NodeHttp {
	return &flowv1.NodeHttp{
		NodeId: n.FlowNodeID.Bytes(),
		HttpId: n.HttpID.Bytes(),
	}
}

func serializeFlowVariable(variable mflowvariable.FlowVariable, order float32) *flowv1.FlowVariable {
	return &flowv1.FlowVariable{
		FlowVariableId: variable.ID.Bytes(),
		FlowId:         variable.FlowID.Bytes(),
		Key:            variable.Name,
		Value:          variable.Value,
		Enabled:        variable.Enabled,
		Description:    variable.Description,
		Order:          order,
	}
}

func isZeroID(id idwrap.IDWrap) bool {
	return id == (idwrap.IDWrap{})
}

func buildCondition(expression string) mcondition.Condition {
	return mcondition.Condition{
		Comparisons: mcondition.Comparison{
			Expression: expression,
		},
	}
}

func convertHandle(h flowv1.Handle) edge.EdgeHandle {
	return edge.EdgeHandle(h)
}

func workspaceIDFromHeaders(header http.Header) (idwrap.IDWrap, error) {
	value := header.Get("workspace-id")
	if value == "" {
		value = header.Get("x-workspace-id")
	}
	if value == "" {
		return idwrap.IDWrap{}, errors.New("workspace id header is required")
	}
	return idwrap.NewText(value)
}

func flowIDFromHeaders(header http.Header) (idwrap.IDWrap, error) {
	value := header.Get("flow-id")
	if value == "" {
		value = header.Get("x-flow-id")
	}
	if value == "" {
		return idwrap.IDWrap{}, errors.New("flow id header is required")
	}
	return idwrap.NewText(value)
}

func (s *FlowServiceV2RPC) deserializeNodeCreate(item *flowv1.NodeCreate) (*mnnode.MNode, error) {
	if item == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node create item is required"))
	}

	if len(item.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}

	flowID, err := idwrap.NewFromBytes(item.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	nodeID := idwrap.NewNow()
	if len(item.GetNodeId()) != 0 {
		nodeID, err = idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}
	}

	var posX, posY float64
	if p := item.GetPosition(); p != nil {
		posX = float64(p.GetX())
		posY = float64(p.GetY())
	}

	return &mnnode.MNode{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      item.GetName(),
		NodeKind:  mnnode.NodeKind(item.GetKind()),
		PositionX: posX,
		PositionY: posY,
	}, nil
}

func (s *FlowServiceV2RPC) ensureWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	workspaces, err := s.listUserWorkspaces(ctx)
	if err != nil {
		return err
	}
	for _, ws := range workspaces {
		if ws.ID == workspaceID {
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("workspace %s not accessible to current user", workspaceID.String()))
}

func (s *FlowServiceV2RPC) ensureFlowAccess(ctx context.Context, flowID idwrap.IDWrap) error {
	flow, err := s.fs.GetFlow(ctx, flowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", flowID.String()))
		}
		return connect.NewError(connect.CodeInternal, err)
	}

	workspaces, err := s.listUserWorkspaces(ctx)
	if err != nil {
		return err
	}
	for _, ws := range workspaces {
		if ws.ID == flow.WorkspaceID {
			return nil
		}
	}
	return connect.NewError(connect.CodePermissionDenied, fmt.Errorf("flow %s not accessible to current user", flowID.String()))
}

func (s *FlowServiceV2RPC) ensureNodeAccess(ctx context.Context, nodeID idwrap.IDWrap) (*mnnode.MNode, error) {
	node, err := s.ns.GetNode(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("node %s not found", nodeID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.ensureFlowAccess(ctx, node.FlowID); err != nil {
		return nil, err
	}
	return node, nil
}

func (s *FlowServiceV2RPC) ensureEdgeAccess(ctx context.Context, edgeID idwrap.IDWrap) (*edge.Edge, error) {
	edgeModel, err := s.es.GetEdge(ctx, edgeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("edge %s not found", edgeID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if err := s.ensureFlowAccess(ctx, edgeModel.FlowID); err != nil {
		return nil, err
	}
	return edgeModel, nil
}

// Ensure FlowServiceV2RPC implements the generated interface.
var _ flowv1connect.FlowServiceHandler = (*FlowServiceV2RPC)(nil)
