package rflowv2

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/converter"
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
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnfor"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
	"the-dev-tools/server/pkg/model/mnnode/mnif"
	"the-dev-tools/server/pkg/model/mnnode/mnjs"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mnodeexecution"
	"the-dev-tools/server/pkg/model/mworkspace"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/sflow"
	"the-dev-tools/server/pkg/service/sflowvariable"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/service/snode"
	"the-dev-tools/server/pkg/service/snodeexecution"
	"the-dev-tools/server/pkg/service/snodefor"
	"the-dev-tools/server/pkg/service/snodeforeach"
	"the-dev-tools/server/pkg/service/snodeif"
	"the-dev-tools/server/pkg/service/snodejs"
	"the-dev-tools/server/pkg/service/snodenoop"
	"the-dev-tools/server/pkg/service/snoderequest"
	"the-dev-tools/server/pkg/service/sworkspace"
	"the-dev-tools/server/pkg/translate/tcurlv2"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	"the-dev-tools/spec/dist/buf/go/api/flow/v1/flowv1connect"
)

var errUnimplemented = errors.New("rflowv2: method not implemented")

// FlowTopic identifies the workspace whose flows are being published.
type FlowTopic struct {
	WorkspaceID idwrap.IDWrap
}

// FlowEvent describes a flow change for sync streaming.
type FlowEvent struct {
	Type string
	Flow *flowv1.Flow
}

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

// FlowVersionTopic identifies the flow whose versions are being published.
type FlowVersionTopic struct {
	FlowID idwrap.IDWrap
}

// FlowVersionEvent describes a flow version change for sync streaming.
type FlowVersionEvent struct {
	Type      string
	FlowID    idwrap.IDWrap
	VersionID idwrap.IDWrap
}

// FlowVariableTopic identifies the flow whose variables are being published.
type FlowVariableTopic struct {
	FlowID idwrap.IDWrap
}

// FlowVariableEvent describes a flow variable change for sync streaming.
type FlowVariableEvent struct {
	Type     string
	FlowID   idwrap.IDWrap
	Variable mflowvariable.FlowVariable
	Order    float32
}

// NoOpTopic identifies the flow whose NoOp nodes are being published.
type NoOpTopic struct {
	FlowID idwrap.IDWrap
}

// NoOpEvent describes a NoOp node change for sync streaming.
type NoOpEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeNoOp
}

// ForTopic identifies the flow whose For nodes are being published.
type ForTopic struct {
	FlowID idwrap.IDWrap
}

// ForEvent describes a For node change for sync streaming.
type ForEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeFor
}

// ConditionTopic identifies the flow whose condition nodes are being published.
type ConditionTopic struct {
	FlowID idwrap.IDWrap
}

// ConditionEvent describes a Condition node change for sync streaming.
type ConditionEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeCondition
}

// ForEachTopic identifies the flow whose ForEach nodes are being published.
type ForEachTopic struct {
	FlowID idwrap.IDWrap
}

// ForEachEvent describes a ForEach node change for sync streaming.
type ForEachEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeForEach
}

// JsTopic identifies the flow whose JavaScript nodes are being published.
type JsTopic struct {
	FlowID idwrap.IDWrap
}

// JsEvent describes a JavaScript node change for sync streaming.
type JsEvent struct {
	Type   string
	FlowID idwrap.IDWrap
	Node   *flowv1.NodeJs
}

// ExecutionTopic identifies the flow whose node executions are being published.
type ExecutionTopic struct {
	FlowID idwrap.IDWrap
}

// ExecutionEvent describes a node execution change for sync streaming.
type ExecutionEvent struct {
	Type      string
	FlowID    idwrap.IDWrap
	Execution *flowv1.NodeExecution
}

const (
	flowEventInsert = "insert"
	flowEventUpdate = "update"
	flowEventDelete = "delete"

	nodeEventInsert = "insert"
	nodeEventUpdate = "update"
	nodeEventDelete = "delete"

	edgeEventInsert = "insert"
	edgeEventUpdate = "update"
	edgeEventDelete = "delete"

	flowVarEventInsert = "insert"
	flowVarEventUpdate = "update"
	flowVarEventDelete = "delete"

	flowVersionEventInsert = "insert"
	flowVersionEventUpdate = "update"
	flowVersionEventDelete = "delete"

	noopEventInsert = "insert"
	noopEventUpdate = "update"
	noopEventDelete = "delete"

	forEventInsert = "insert"
	forEventUpdate = "update"
	forEventDelete = "delete"

	conditionEventInsert = "insert"
	conditionEventUpdate = "update"
	conditionEventDelete = "delete"

	forEachEventInsert = "insert"
	forEachEventUpdate = "update"
	forEachEventDelete = "delete"

	jsEventInsert = "insert"
	jsEventUpdate = "update"
	jsEventDelete = "delete"

	executionEventInsert = "insert"
	executionEventUpdate = "update"
	executionEventDelete = "delete"
)

type FlowServiceV2RPC struct {
	ws   *sworkspace.WorkspaceService
	fs   *sflow.FlowService
	es   *sedge.EdgeService
	ns   *snode.NodeService
	nrs  *snoderequest.NodeRequestService
	nfs  *snodefor.NodeForService
	nfes *snodeforeach.NodeForEachService
	nifs *snodeif.NodeIfService
	nnos *snodenoop.NodeNoopService
	njss *snodejs.NodeJSService
	nes  *snodeexecution.NodeExecutionService
	fvs  *sflowvariable.FlowVariableService
	hs   *shttp.HTTPService
	hh   *shttp.HttpHeaderService
	hsp  *shttp.HttpSearchParamService
	hbf  *shttp.HttpBodyFormService
	hbu  *shttp.HttpBodyUrlencodedService
	has  *shttp.HttpAssertService
	hbr  *shttp.HttpBodyRawService
	logger *slog.Logger
	// V2 import services
	workspaceImportService WorkspaceImporter
	flowStream             eventstream.SyncStreamer[FlowTopic, FlowEvent]
	nodeStream             eventstream.SyncStreamer[NodeTopic, NodeEvent]
	edgeStream             eventstream.SyncStreamer[EdgeTopic, EdgeEvent]
	varStream              eventstream.SyncStreamer[FlowVariableTopic, FlowVariableEvent]
	versionStream          eventstream.SyncStreamer[FlowVersionTopic, FlowVersionEvent]
	noopStream             eventstream.SyncStreamer[NoOpTopic, NoOpEvent]
	forStream              eventstream.SyncStreamer[ForTopic, ForEvent]
	conditionStream        eventstream.SyncStreamer[ConditionTopic, ConditionEvent]
	forEachStream          eventstream.SyncStreamer[ForEachTopic, ForEachEvent]
	jsStream               eventstream.SyncStreamer[JsTopic, JsEvent]
	executionStream        eventstream.SyncStreamer[ExecutionTopic, ExecutionEvent]
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
	nes *snodeexecution.NodeExecutionService,
	fvs *sflowvariable.FlowVariableService,
	hs *shttp.HTTPService,
	hh *shttp.HttpHeaderService,
	hsp *shttp.HttpSearchParamService,
	hbf *shttp.HttpBodyFormService,
	hbu *shttp.HttpBodyUrlencodedService,
	has *shttp.HttpAssertService,
	hbr *shttp.HttpBodyRawService,
	logger *slog.Logger,
	workspaceImportService WorkspaceImporter,
	flowStream eventstream.SyncStreamer[FlowTopic, FlowEvent],
	nodeStream eventstream.SyncStreamer[NodeTopic, NodeEvent],
	edgeStream eventstream.SyncStreamer[EdgeTopic, EdgeEvent],
	varStream eventstream.SyncStreamer[FlowVariableTopic, FlowVariableEvent],
	versionStream eventstream.SyncStreamer[FlowVersionTopic, FlowVersionEvent],
	noopStream eventstream.SyncStreamer[NoOpTopic, NoOpEvent],
	forStream eventstream.SyncStreamer[ForTopic, ForEvent],
	conditionStream eventstream.SyncStreamer[ConditionTopic, ConditionEvent],
	forEachStream eventstream.SyncStreamer[ForEachTopic, ForEachEvent],
	jsStream eventstream.SyncStreamer[JsTopic, JsEvent],
	executionStream eventstream.SyncStreamer[ExecutionTopic, ExecutionEvent],
) *FlowServiceV2RPC {
	return &FlowServiceV2RPC{
		ws:                     ws,
		fs:                     fs,
		es:                     es,
		ns:                     ns,
		nrs:                    nrs,
		nfs:                    nfs,
		nfes:                   nfes,
		nifs:                   nifs,
		nnos:                   nnos,
		njss:                   njss,
		nes:                    nes,
		fvs:                    fvs,
		hs:                     hs,
		hh:                     hh,
		hsp:                    hsp,
		hbf:                    hbf,
		hbu:                    hbu,
		has:                    has,
		hbr:                    hbr,
		logger:                 logger,
		workspaceImportService: workspaceImportService,
		flowStream:             flowStream,
		nodeStream:             nodeStream,
		edgeStream:             edgeStream,
		varStream:              varStream,
		versionStream:          versionStream,
		noopStream:             noopStream,
		forStream:              forStream,
		conditionStream:        conditionStream,
		forEachStream:          forEachStream,
		jsStream:               jsStream,
		executionStream:        executionStream,
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

func (s *FlowServiceV2RPC) FlowSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.FlowSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamFlowSync(ctx, func(resp *flowv1.FlowSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamFlowSync(
	ctx context.Context,
	send func(*flowv1.FlowSyncResponse) error,
) error {
	if s.flowStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("flow stream not configured"))
	}

	var workspaceSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[FlowTopic, FlowEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[FlowTopic, FlowEvent], 0)

		for _, flow := range flows {
			workspaceSet.Store(flow.WorkspaceID.String(), struct{}{})

			events = append(events, eventstream.Event[FlowTopic, FlowEvent]{
				Topic: FlowTopic{WorkspaceID: flow.WorkspaceID},
				Payload: FlowEvent{
					Type: flowEventInsert,
					Flow: serializeFlow(flow),
				},
			})
		}

		return events, nil
	}

	filter := func(topic FlowTopic) bool {
		if _, ok := workspaceSet.Load(topic.WorkspaceID.String()); ok {
			return true
		}
		if err := s.ensureWorkspaceAccess(ctx, topic.WorkspaceID); err != nil {
			return false
		}
		workspaceSet.Store(topic.WorkspaceID.String(), struct{}{})
		return true
	}

	events, err := s.flowStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := flowEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) publishFlowEvent(eventType string, flow mflow.Flow) {
	if s.flowStream == nil {
		return
	}
	s.flowStream.Publish(FlowTopic{WorkspaceID: flow.WorkspaceID}, FlowEvent{
		Type: eventType,
		Flow: serializeFlow(flow),
	})
}

func flowEventToSyncResponse(evt FlowEvent) *flowv1.FlowSyncResponse {
	if evt.Flow == nil {
		return nil
	}

	var syncEvent *flowv1.FlowSync
	switch evt.Type {
	case flowEventInsert:
		insert := &flowv1.FlowSyncInsert{
			FlowId: evt.Flow.FlowId,
			Name:   evt.Flow.Name,
		}
		if evt.Flow.Duration != nil {
			insert.Duration = evt.Flow.Duration
		}
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind:   flowv1.FlowSync_ValueUnion_KIND_INSERT,
				Insert: insert,
			},
		}
	case flowEventUpdate:
		update := &flowv1.FlowSyncUpdate{
			FlowId: evt.Flow.FlowId,
		}
		if evt.Flow.Name != "" {
			update.Name = &evt.Flow.Name
		}
		if evt.Flow.Duration != nil {
			update.Duration = &flowv1.FlowSyncUpdate_DurationUnion{
				Kind:  flowv1.FlowSyncUpdate_DurationUnion_KIND_VALUE,
				Value: evt.Flow.Duration,
			}
		}
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind:   flowv1.FlowSync_ValueUnion_KIND_UPDATE,
				Update: update,
			},
		}
	case flowEventDelete:
		syncEvent = &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind: flowv1.FlowSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.FlowSyncDelete{
					FlowId: evt.Flow.FlowId,
				},
			},
		}
	default:
		return nil
	}

	return &flowv1.FlowSyncResponse{
		Items: []*flowv1.FlowSync{syncEvent},
	}
}

func (s *FlowServiceV2RPC) FlowInsert(ctx context.Context, req *connect.Request[flowv1.FlowInsertRequest]) (*connect.Response[emptypb.Empty], error) {
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

		if created, err := s.fs.GetFlow(ctx, flowID); err == nil {
			s.publishFlowEvent(flowEventInsert, created)
			if created.VersionParentID != nil {
				s.publishFlowVersionEvent(flowVersionEventInsert, created)
			}
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
			case flowv1.FlowUpdate_DurationUnion_KIND_VALUE:
				flow.Duration = du.GetValue()
			}
		}

		if err := s.fs.UpdateFlow(ctx, flow); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishFlowEvent(flowEventUpdate, flow)

		if flow.VersionParentID != nil {
			s.publishFlowVersionEvent(flowVersionEventUpdate, flow)
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

		s.publishFlowEvent(flowEventDelete, flow)

		if flow.VersionParentID != nil {
			s.publishFlowVersionEvent(flowVersionEventDelete, flow)
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

	nodeStateChan := make(chan runner.FlowNodeStatus, len(nodes)*2+1)
	var stateDrain sync.WaitGroup
	stateDrain.Add(1)
	go func() {
		defer stateDrain.Done()
		for status := range nodeStateChan {
			if s.nodeStream != nil {
				s.nodeStream.Publish(NodeTopic{FlowID: flow.ID}, NodeEvent{
					Type:   nodeEventUpdate,
					FlowID: flow.ID,
					Node: &flowv1.Node{
						NodeId: status.NodeID.Bytes(),
						FlowId: flow.ID.Bytes(),
						State:  flowv1.FlowItemState(status.State),
					},
				})
			}
		}
	}()

	if err := flowRunner.RunWithEvents(ctx, runner.FlowEventChannels{
		NodeStates: nodeStateChan,
	}, baseVars); err != nil {
		stateDrain.Wait()
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	stateDrain.Wait()
	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowStop(ctx context.Context, req *connect.Request[flowv1.FlowStopRequest]) (*connect.Response[emptypb.Empty], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errUnimplemented)
}

func (s *FlowServiceV2RPC) FlowDuplicate(ctx context.Context, req *connect.Request[flowv1.FlowDuplicateRequest]) (*connect.Response[emptypb.Empty], error) {
	if len(req.Msg.GetFlowId()) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("flow id is required"))
	}

	sourceFlowID, err := idwrap.NewFromBytes(req.Msg.GetFlowId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid flow id: %w", err))
	}

	// Validate access to source flow
	if err := s.ensureFlowAccess(ctx, sourceFlowID); err != nil {
		return nil, err
	}

	// Get source flow details
	sourceFlow, err := s.fs.GetFlow(ctx, sourceFlowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("flow %s not found", sourceFlowID.String()))
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get workspace access for creating the new flow
	if err := s.ensureWorkspaceAccess(ctx, sourceFlow.WorkspaceID); err != nil {
		return nil, err
	}

	// Get workspace to update flow count
	workspace, err := s.ws.Get(ctx, sourceFlow.WorkspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create new flow with duplicated name
	newFlowID := idwrap.NewNow()
	duplicatedName := fmt.Sprintf("Copy of %s", sourceFlow.Name)

	newFlow := mflow.Flow{
		ID:          newFlowID,
		WorkspaceID: sourceFlow.WorkspaceID,
		Name:        duplicatedName,
	}

	if err := s.fs.CreateFlow(ctx, newFlow); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create a mapping from old node IDs to new node IDs for edge remapping
	nodeIDMapping := make(map[string]string)

	// Duplicate all nodes
	sourceNodes, err := s.ns.GetNodesByFlowID(ctx, sourceFlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, sourceNode := range sourceNodes {
		newNodeID := idwrap.NewNow()
		nodeIDMapping[sourceNode.ID.String()] = newNodeID.String()

		// Create the basic node
		newNode := mnnode.MNode{
			ID:        newNodeID,
			FlowID:    newFlowID,
			Name:      sourceNode.Name,
			NodeKind:  sourceNode.NodeKind,
			PositionX: sourceNode.PositionX,
			PositionY: sourceNode.PositionY,
		}

		if err := s.ns.CreateNode(ctx, newNode); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Duplicate node-type specific data
		switch sourceNode.NodeKind {
		case mnnode.NODE_KIND_NO_OP:
			noopData, err := s.nnos.GetNodeNoop(ctx, sourceNode.ID)
			if err == nil {
				newNoopData := mnnoop.NoopNode{
					FlowNodeID: newNodeID,
					Type:       noopData.Type,
				}
				if err := s.nnos.CreateNodeNoop(ctx, newNoopData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}

		case mnnode.NODE_KIND_REQUEST:
			requestData, err := s.nrs.GetNodeRequest(ctx, sourceNode.ID)
			if err == nil {
				// Get the original HTTP data
				httpData, err := s.hs.Get(ctx, requestData.HttpID)
				if err == nil {
					// Create a new HTTP record for the duplicated node
					newHttpID := idwrap.NewNow()
					duplicatedHttp := *httpData
					duplicatedHttp.ID = newHttpID
					duplicatedHttp.Name = fmt.Sprintf("Copy of %s", httpData.Name)

					if err := s.hs.Create(ctx, &duplicatedHttp); err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}

					newRequestData := mnrequest.MNRequest{
						FlowNodeID:       newNodeID,
						HttpID:           newHttpID,
						HasRequestConfig: requestData.HasRequestConfig,
					}
					if err := s.nrs.CreateNodeRequest(ctx, newRequestData); err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
				}
			}

		case mnnode.NODE_KIND_FOR:
			forData, err := s.nfs.GetNodeFor(ctx, sourceNode.ID)
			if err == nil {
				newForData := mnfor.MNFor{
					FlowNodeID:    newNodeID,
					IterCount:     forData.IterCount,
					Condition:     forData.Condition,
					ErrorHandling: forData.ErrorHandling,
				}
				if err := s.nfs.CreateNodeFor(ctx, newForData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}

		case mnnode.NODE_KIND_FOR_EACH:
			forEachData, err := s.nfes.GetNodeForEach(ctx, sourceNode.ID)
			if err == nil {
				newForEachData := mnforeach.MNForEach{
					FlowNodeID:     newNodeID,
					IterExpression: forEachData.IterExpression,
					Condition:      forEachData.Condition,
					ErrorHandling:  forEachData.ErrorHandling,
				}
				if err := s.nfes.CreateNodeForEach(ctx, newForEachData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}

		case mnnode.NODE_KIND_CONDITION:
			conditionData, err := s.nifs.GetNodeIf(ctx, sourceNode.ID)
			if err == nil {
				newConditionData := mnif.MNIF{
					FlowNodeID: newNodeID,
					Condition:  conditionData.Condition,
				}
				if err := s.nifs.CreateNodeIf(ctx, newConditionData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}

		case mnnode.NODE_KIND_JS:
			jsData, err := s.njss.GetNodeJS(ctx, sourceNode.ID)
			if err == nil {
				newJsData := mnjs.MNJS{
					FlowNodeID: newNodeID,
					Code:       jsData.Code,
				}
				if err := s.njss.CreateNodeJS(ctx, newJsData); err != nil {
					return nil, connect.NewError(connect.CodeInternal, err)
				}
			}
		}
	}

	// Duplicate all edges with remapped node IDs
	sourceEdges, err := s.es.GetEdgesByFlowID(ctx, sourceFlowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, sourceEdge := range sourceEdges {
		newEdgeID := idwrap.NewNow()

		// Map old node IDs to new node IDs
		newSourceIDStr, sourceOK := nodeIDMapping[sourceEdge.SourceID.String()]
		newTargetIDStr, targetOK := nodeIDMapping[sourceEdge.TargetID.String()]

		if !sourceOK || !targetOK {
			// This should not happen in normal circumstances, but skip invalid edges
			continue
		}

		newSourceID, err := idwrap.NewText(newSourceIDStr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid new source node id: %w", err))
		}

		newTargetID, err := idwrap.NewText(newTargetIDStr)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid new target node id: %w", err))
		}

		newEdge := edge.Edge{
			ID:            newEdgeID,
			FlowID:        newFlowID,
			SourceID:      newSourceID,
			TargetID:      newTargetID,
			SourceHandler: sourceEdge.SourceHandler,
			Kind:          sourceEdge.Kind,
		}

		if err := s.es.CreateEdge(ctx, newEdge); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Duplicate all flow variables
	sourceVariables, err := s.fvs.GetFlowVariablesByFlowID(ctx, sourceFlowID)
	if err != nil && !errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	for _, sourceVariable := range sourceVariables {
		newVariableID := idwrap.NewNow()
		newVariable := mflowvariable.FlowVariable{
			ID:          newVariableID,
			FlowID:      newFlowID,
			Name:        sourceVariable.Name,
			Value:       sourceVariable.Value,
			Enabled:     sourceVariable.Enabled,
			Description: sourceVariable.Description,
		}

		if err := s.fvs.CreateFlowVariable(ctx, newVariable); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Update workspace flow count
	workspace.FlowCount++
	workspace.Updated = dbtime.DBNow()
	if err := s.ws.Update(ctx, workspace); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVersionCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVersionCollectionResponse], error) {
	// Get all accessible flows
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	// Collect all versions from all flows
	var allVersions []*flowv1.FlowVersion
	for _, flow := range flows {
		versions, err := s.fs.GetFlowsByVersionParentID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, version := range versions {
			allVersions = append(allVersions, &flowv1.FlowVersion{
				FlowVersionId: version.ID.Bytes(),
				FlowId:        flow.ID.Bytes(),
			})
		}
	}

	// Sort by flow version ID for consistent ordering
	sort.Slice(allVersions, func(i, j int) bool {
		return bytes.Compare(allVersions[i].GetFlowVersionId(), allVersions[j].GetFlowVersionId()) < 0
	})

	return connect.NewResponse(&flowv1.FlowVersionCollectionResponse{Items: allVersions}), nil
}

func (s *FlowServiceV2RPC) FlowVersionSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.FlowVersionSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamFlowVersionSync(ctx, func(resp *flowv1.FlowVersionSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) FlowVariableCollection(ctx context.Context, req *connect.Request[emptypb.Empty]) (*connect.Response[flowv1.FlowVariableCollectionResponse], error) {
	// Get all accessible flows
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	// Collect all variables from all flows
	var allVariables []*flowv1.FlowVariable
	var globalIndex float32
	for _, flow := range flows {
		variables, err := s.fvs.GetFlowVariablesByFlowIDOrdered(ctx, flow.ID)
		if err != nil {
			if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
				continue // No variables for this flow, continue to next
			} else {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}

		for _, variable := range variables {
			allVariables = append(allVariables, serializeFlowVariable(variable, globalIndex))
			globalIndex++
		}
	}

	return connect.NewResponse(&flowv1.FlowVariableCollectionResponse{Items: allVariables}), nil
}

func (s *FlowServiceV2RPC) FlowVariableInsert(ctx context.Context, req *connect.Request[flowv1.FlowVariableInsertRequest]) (*connect.Response[emptypb.Empty], error) {
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

		order, err := s.flowVariableOrder(ctx, flowID, variableID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		s.publishFlowVariableEvent(flowVarEventInsert, variable, order)
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

		order, err := s.flowVariableOrder(ctx, variable.FlowID, variable.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		s.publishFlowVariableEvent(flowVarEventUpdate, variable, order)
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

		s.publishFlowVariableEvent(flowVarEventDelete, variable, 0)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) FlowVariableSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.FlowVariableSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamFlowVariableSync(ctx, func(resp *flowv1.FlowVariableSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) EdgeInsert(ctx context.Context, req *connect.Request[flowv1.EdgeInsertRequest]) (*connect.Response[emptypb.Empty], error) {
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

		s.publishEdgeEvent(edgeEventInsert, model)
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
						Type:   edgeEventInsert,
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

func (s *FlowServiceV2RPC) publishFlowVariableEvent(eventType string, variable mflowvariable.FlowVariable, order float32) {
	if s.varStream == nil {
		return
	}
	s.varStream.Publish(FlowVariableTopic{FlowID: variable.FlowID}, FlowVariableEvent{
		Type:     eventType,
		FlowID:   variable.FlowID,
		Variable: variable,
		Order:    order,
	})
}

func (s *FlowServiceV2RPC) publishFlowVersionEvent(eventType string, flow mflow.Flow) {
	if s.versionStream == nil {
		return
	}
	if flow.VersionParentID == nil {
		return
	}
	parent := *flow.VersionParentID
	s.versionStream.Publish(FlowVersionTopic{FlowID: parent}, FlowVersionEvent{
		Type:      eventType,
		FlowID:    parent,
		VersionID: flow.ID,
	})
}

func (s *FlowServiceV2RPC) publishNoOpEvent(eventType string, flowID idwrap.IDWrap, node mnnoop.NoopNode) {
	if s.noopStream == nil {
		return
	}

	nodePB := serializeNodeNoop(node)
	s.noopStream.Publish(NoOpTopic{FlowID: flowID}, NoOpEvent{
		Type:   eventType,
		FlowID: flowID,
		Node:   nodePB,
	})
}

func (s *FlowServiceV2RPC) publishForEvent(eventType string, flowID idwrap.IDWrap, node mnfor.MNFor) {
	if s.forStream == nil {
		return
	}

	nodePB := serializeNodeFor(node)
	s.forStream.Publish(ForTopic{FlowID: flowID}, ForEvent{
		Type:   eventType,
		FlowID: flowID,
		Node:   nodePB,
	})
}

func (s *FlowServiceV2RPC) publishJsEvent(eventType string, flowID idwrap.IDWrap, node mnjs.MNJS) {
	if s.jsStream == nil {
		return
	}

	nodePB := serializeNodeJs(node)
	s.jsStream.Publish(JsTopic{FlowID: flowID}, JsEvent{
		Type:   eventType,
		FlowID: flowID,
		Node:   nodePB,
	})
}

func (s *FlowServiceV2RPC) NodeInsert(
	ctx context.Context,
	req *connect.Request[flowv1.NodeInsertRequest],
) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeModel, err := s.deserializeNodeInsert(item)
		if err != nil {
			return nil, err
		}

		if err := s.ensureFlowAccess(ctx, nodeModel.FlowID); err != nil {
			return nil, err
		}

		if err := s.ns.CreateNode(ctx, *nodeModel); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		s.publishNodeEvent(nodeEventInsert, *nodeModel)
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
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
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

func (s *FlowServiceV2RPC) streamFlowVariableSync(
	ctx context.Context,
	send func(*flowv1.FlowVariableSyncResponse) error,
) error {
	if s.varStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("flow variable stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[FlowVariableTopic, FlowVariableEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[FlowVariableTopic, FlowVariableEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			variables, err := s.fvs.GetFlowVariablesByFlowIDOrdered(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sflowvariable.ErrNoFlowVariableFound) {
					continue
				}
				return nil, err
			}

			for idx, variable := range variables {
				events = append(events, eventstream.Event[FlowVariableTopic, FlowVariableEvent]{
					Topic: FlowVariableTopic{FlowID: flow.ID},
					Payload: FlowVariableEvent{
						Type:     flowVarEventInsert,
						FlowID:   flow.ID,
						Variable: variable,
						Order:    float32(idx),
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic FlowVariableTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.varStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := flowVariableEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) streamFlowVersionSync(
	ctx context.Context,
	send func(*flowv1.FlowVersionSyncResponse) error,
) error {
	if s.versionStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("flow version stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[FlowVersionTopic, FlowVersionEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		topics := make(map[string]struct{}, len(flows))
		events := make([]eventstream.Event[FlowVersionTopic, FlowVersionEvent], 0)

		for _, flow := range flows {
			parentID := flow.ID
			key := parentID.String()
			if _, seen := topics[key]; seen {
				continue
			}
			topics[key] = struct{}{}
			flowSet.Store(key, struct{}{})

			versions, err := s.fs.GetFlowsByVersionParentID(ctx, parentID)
			if err != nil {
				if errors.Is(err, sflow.ErrNoFlowFound) {
					continue
				}
				return nil, err
			}

			for _, version := range versions {
				events = append(events, eventstream.Event[FlowVersionTopic, FlowVersionEvent]{
					Topic: FlowVersionTopic{FlowID: parentID},
					Payload: FlowVersionEvent{
						Type:      flowVersionEventInsert,
						FlowID:    parentID,
						VersionID: version.ID,
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic FlowVersionTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.versionStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := flowVersionEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) streamNoOpSync(
	ctx context.Context,
	send func(*flowv1.NodeNoOpSyncResponse) error,
) error {
	if s.noopStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("noop stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[NoOpTopic, NoOpEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[NoOpTopic, NoOpEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			// Get all nodes in the flow and filter for NoOp nodes
			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, node := range nodes {
				// Only process NoOp nodes
				if node.NodeKind != mnnode.NODE_KIND_NO_OP {
					continue
				}

				// Get the NoOp configuration for this node
				noopNode, err := s.nnos.GetNodeNoop(ctx, node.ID)
				if err != nil {
					if err == sql.ErrNoRows {
						continue
					}
					return nil, err
				}

				if noopNode == nil {
					continue
				}

				noopPB := serializeNodeNoop(*noopNode)
				events = append(events, eventstream.Event[NoOpTopic, NoOpEvent]{
					Topic: NoOpTopic{FlowID: flow.ID},
					Payload: NoOpEvent{
						Type:   noopEventInsert,
						FlowID: flow.ID,
						Node:   noopPB,
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic NoOpTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.noopStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := noopEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) streamNodeForSync(
	ctx context.Context,
	send func(*flowv1.NodeForSyncResponse) error,
) error {
	if s.forStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("for stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[ForTopic, ForEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[ForTopic, ForEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			// Get all nodes in the flow and filter for For nodes
			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			for _, node := range nodes {
				// Only process For nodes
				if node.NodeKind != mnnode.NODE_KIND_FOR {
					continue
				}

				// Skip start nodes (For nodes shouldn't be start nodes, but just in case)
				if isStartNode(node) {
					continue
				}

				// Get the For configuration for this node
				forNode, err := s.nfs.GetNodeFor(ctx, node.ID)
				if err != nil {
					if err == sql.ErrNoRows {
						continue
					}
					return nil, err
				}

				if forNode == nil {
					continue
				}

				forPB := serializeNodeFor(*forNode)
				events = append(events, eventstream.Event[ForTopic, ForEvent]{
					Topic: ForTopic{FlowID: flow.ID},
					Payload: ForEvent{
						Type:   forEventInsert,
						FlowID: flow.ID,
						Node:   forPB,
					},
				})
			}
		}

		return events, nil
	}

	filter := func(topic ForTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.forStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp := forEventToSyncResponse(evt.Payload)
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

func (s *FlowServiceV2RPC) NodeNoOpCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeNoOpCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeNoOp, 0)

	for _, flow := range flows {
		// Get all nodes in the flow and filter for NoOp nodes
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		for _, node := range nodes {
			// Only process NoOp nodes
			if node.NodeKind != mnnode.NODE_KIND_NO_OP {
				continue
			}

			// Get the NoOp configuration for this node
			noopNode, err := s.nnos.GetNodeNoop(ctx, node.ID)
			if err != nil {
				if err == sql.ErrNoRows {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			if noopNode == nil {
				continue
			}

			items = append(items, serializeNodeNoop(*noopNode))
		}
	}

	return connect.NewResponse(&flowv1.NodeNoOpCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpInsert(ctx context.Context, req *connect.Request[flowv1.NodeNoOpInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
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

		// Publish insert event
		s.publishNoOpEvent(noopEventInsert, baseNode.FlowID, noop)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpUpdate(ctx context.Context, req *connect.Request[flowv1.NodeNoOpUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		if item.Kind == nil {
			continue
		}

		// Get existing NoOp node to publish delete event
		existingNoOp, err := s.nnos.GetNodeNoop(ctx, nodeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
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

		// Publish events
		if existingNoOp != nil {
			s.publishNoOpEvent(noopEventDelete, baseNode.FlowID, *existingNoOp)
		}
		s.publishNoOpEvent(noopEventInsert, baseNode.FlowID, noop)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpDelete(ctx context.Context, req *connect.Request[flowv1.NodeNoOpDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		// Get existing NoOp node to publish delete event
		existingNoOp, err := s.nnos.GetNodeNoop(ctx, nodeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.nnos.DeleteNodeNoop(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish delete event
		if existingNoOp != nil {
			s.publishNoOpEvent(noopEventDelete, baseNode.FlowID, *existingNoOp)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeNoOpSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeNoOpSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNoOpSync(ctx, func(resp *flowv1.NodeNoOpSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) NodeHttpInsert(ctx context.Context, req *connect.Request[flowv1.NodeHttpInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		if _, err := s.ensureNodeAccess(ctx, nodeID); err != nil {
			return nil, err
		}

		var httpID idwrap.IDWrap
		if len(item.GetHttpId()) > 0 {
			httpID, err = idwrap.NewFromBytes(item.GetHttpId())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid http id: %w", err))
			}
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
		union := item.GetHttpId()
		if union != nil && union.Kind == flowv1.NodeHttpUpdate_HttpIdUnion_KIND_VALUE {
			httpID, err = idwrap.NewFromBytes(union.GetValue())
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

func (s *FlowServiceV2RPC) NodeHttpSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeHttpSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeHttpSync(ctx, func(resp *flowv1.NodeHttpSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeHttpSync(
	ctx context.Context,
	send func(*flowv1.NodeHttpSyncResponse) error,
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
				// Filter for HTTP nodes (REQUEST nodes)
				if nodeModel.NodeKind != mnnode.NODE_KIND_REQUEST {
					continue
				}

				nodeReq, err := s.nrs.GetNodeRequest(ctx, nodeModel.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}
				if nodeReq == nil || isZeroID(nodeReq.HttpID) {
					continue
				}

				// Create a custom NodeEvent that includes HTTP node data
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node:   serializeNode(nodeModel), // Pass regular node for compatibility
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
			resp, err := s.nodeHttpEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert HTTP node event: %w", err))
			}
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

func (s *FlowServiceV2RPC) streamNodeConditionSync(
	ctx context.Context,
	send func(*flowv1.NodeConditionSyncResponse) error,
) error {
	if s.conditionStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("condition stream not configured"))
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
				// Filter for Condition nodes
				if nodeModel.NodeKind != mnnode.NODE_KIND_CONDITION {
					continue
				}

				nodeCondition, err := s.nifs.GetNodeIf(ctx, nodeModel.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}
				if nodeCondition == nil {
					continue
				}

				// Create a custom NodeEvent that includes Condition node data
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node: &flowv1.Node{
							NodeId: nodeCondition.FlowNodeID.Bytes(),
							Kind:   flowv1.NodeKind_NODE_KIND_CONDITION,
						},
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
			resp, err := s.conditionEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert Condition node event: %w", err))
			}
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

func (s *FlowServiceV2RPC) streamNodeForEachSync(
	ctx context.Context,
	send func(*flowv1.NodeForEachSyncResponse) error,
) error {
	if s.forEachStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("forEach stream not configured"))
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
				// Filter for ForEach nodes
				if nodeModel.NodeKind != mnnode.NODE_KIND_FOR_EACH {
					continue
				}

				nodeForEach, err := s.nfes.GetNodeForEach(ctx, nodeModel.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}
				if nodeForEach == nil {
					continue
				}

				// Create a custom NodeEvent that includes ForEach node data
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node: &flowv1.Node{
							NodeId: nodeForEach.FlowNodeID.Bytes(),
							Kind:   flowv1.NodeKind_NODE_KIND_FOR_EACH,
						},
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
			resp, err := s.forEachEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert ForEach node event: %w", err))
			}
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

func (s *FlowServiceV2RPC) streamNodeJsSync(
	ctx context.Context,
	send func(*flowv1.NodeJsSyncResponse) error,
) error {
	if s.jsStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("js stream not configured"))
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
				// Filter for JS nodes
				if nodeModel.NodeKind != mnnode.NODE_KIND_JS {
					continue
				}

				nodeJs, err := s.njss.GetNodeJS(ctx, nodeModel.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}

				// Create a custom NodeEvent that includes JS node data
				events = append(events, eventstream.Event[NodeTopic, NodeEvent]{
					Topic: NodeTopic{FlowID: flow.ID},
					Payload: NodeEvent{
						Type:   nodeEventInsert,
						FlowID: flow.ID,
						Node: &flowv1.Node{
							NodeId: nodeJs.FlowNodeID.Bytes(),
							Kind:   flowv1.NodeKind_NODE_KIND_JS,
						},
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
			resp, err := s.jsEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert JS node event: %w", err))
			}
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

func (s *FlowServiceV2RPC) nodeHttpEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeHttpSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process HTTP nodes (REQUEST nodes)
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_HTTP {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the HTTP configuration for this node
	nodeReq, err := s.nrs.GetNodeRequest(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Node exists but doesn't have HTTP config, skip
			return nil, nil
		}
		return nil, err
	}
	if nodeReq == nil || isZeroID(nodeReq.HttpID) {
		return nil, nil
	}

	var syncEvent *flowv1.NodeHttpSync
	switch evt.Type {
	case nodeEventInsert:
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind: flowv1.NodeHttpSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeHttpSyncInsert{
					NodeId: nodeReq.FlowNodeID.Bytes(),
					HttpId: nodeReq.HttpID.Bytes(),
				},
			},
		}
	case nodeEventUpdate:
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind: flowv1.NodeHttpSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeHttpSyncUpdate{
					NodeId: nodeReq.FlowNodeID.Bytes(),
					HttpId: &flowv1.NodeHttpSyncUpdate_HttpIdUnion{
						Kind:  flowv1.NodeHttpSyncUpdate_HttpIdUnion_KIND_VALUE,
						Value: nodeReq.HttpID.Bytes(),
					},
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeHttpSync{
			Value: &flowv1.NodeHttpSync_ValueUnion{
				Kind: flowv1.NodeHttpSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeHttpSyncDelete{
					NodeId: nodeReq.FlowNodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeHttpSyncResponse{
		Items: []*flowv1.NodeHttpSync{syncEvent},
	}, nil
}

func (s *FlowServiceV2RPC) conditionEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeConditionSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process Condition nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_CONDITION {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the Condition configuration for this node
	nodeCondition, err := s.nifs.GetNodeIf(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Node exists but doesn't have Condition config, skip
			return nil, nil
		}
		return nil, err
	}
	if nodeCondition == nil {
		return nil, nil
	}

	var syncEvent *flowv1.NodeConditionSync
	switch evt.Type {
	case nodeEventInsert:
		syncEvent = &flowv1.NodeConditionSync{
			Value: &flowv1.NodeConditionSync_ValueUnion{
				Kind: flowv1.NodeConditionSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeConditionSyncInsert{
					NodeId: nodeCondition.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventUpdate:
		syncEvent = &flowv1.NodeConditionSync{
			Value: &flowv1.NodeConditionSync_ValueUnion{
				Kind: flowv1.NodeConditionSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeConditionSyncUpdate{
					NodeId: nodeCondition.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeConditionSync{
			Value: &flowv1.NodeConditionSync_ValueUnion{
				Kind: flowv1.NodeConditionSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeConditionSyncDelete{
					NodeId: nodeCondition.FlowNodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeConditionSyncResponse{
		Items: []*flowv1.NodeConditionSync{syncEvent},
	}, nil
}

func (s *FlowServiceV2RPC) forEachEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeForEachSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process ForEach nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_FOR_EACH {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the ForEach configuration for this node
	nodeForEach, err := s.nfes.GetNodeForEach(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Node exists but doesn't have ForEach config, skip
			return nil, nil
		}
		return nil, err
	}
	if nodeForEach == nil {
		return nil, nil
	}

	var syncEvent *flowv1.NodeForEachSync
	switch evt.Type {
	case nodeEventInsert:
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind: flowv1.NodeForEachSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeForEachSyncInsert{
					NodeId: nodeForEach.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventUpdate:
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind: flowv1.NodeForEachSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeForEachSyncUpdate{
					NodeId: nodeForEach.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeForEachSync{
			Value: &flowv1.NodeForEachSync_ValueUnion{
				Kind: flowv1.NodeForEachSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeForEachSyncDelete{
					NodeId: nodeForEach.FlowNodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeForEachSyncResponse{
		Items: []*flowv1.NodeForEachSync{syncEvent},
	}, nil
}

func (s *FlowServiceV2RPC) jsEventToSyncResponse(
	ctx context.Context,
	evt NodeEvent,
) (*flowv1.NodeJsSyncResponse, error) {
	if evt.Node == nil {
		return nil, nil
	}

	// Only process JS nodes
	if evt.Node.GetKind() != flowv1.NodeKind_NODE_KIND_JS {
		return nil, nil
	}

	nodeID, err := idwrap.NewFromBytes(evt.Node.GetNodeId())
	if err != nil {
		return nil, fmt.Errorf("invalid node id: %w", err)
	}

	// Fetch the JavaScript configuration for this node
	nodeJs, err := s.njss.GetNodeJS(ctx, nodeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Node exists but doesn't have JS config, skip
			return nil, nil
		}
		return nil, err
	}

	var syncEvent *flowv1.NodeJsSync
	switch evt.Type {
	case nodeEventInsert:
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind: flowv1.NodeJsSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeJsSyncInsert{
					NodeId: nodeJs.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventUpdate:
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind: flowv1.NodeJsSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeJsSyncUpdate{
					NodeId: nodeJs.FlowNodeID.Bytes(),
				},
			},
		}
	case nodeEventDelete:
		syncEvent = &flowv1.NodeJsSync{
			Value: &flowv1.NodeJsSync_ValueUnion{
				Kind: flowv1.NodeJsSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeJsSyncDelete{
					NodeId: nodeJs.FlowNodeID.Bytes(),
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeJsSyncResponse{
		Items: []*flowv1.NodeJsSync{syncEvent},
	}, nil
}

func (s *FlowServiceV2RPC) NodeForCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeForCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeFor, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_FOR {
				continue
			}
			nodeFor, err := s.nfs.GetNodeFor(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeFor == nil {
				continue
			}
			items = append(items, serializeNodeFor(*nodeFor))
		}
	}

	return connect.NewResponse(&flowv1.NodeForCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeForInsert(ctx context.Context, req *connect.Request[flowv1.NodeForInsertRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
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

		// Publish insert event
		s.publishForEvent(forEventInsert, baseNode.FlowID, model)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForUpdate(ctx context.Context, req *connect.Request[flowv1.NodeForUpdateRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
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

		// Publish update event
		s.publishForEvent(forEventUpdate, baseNode.FlowID, *existing)
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForDelete(ctx context.Context, req *connect.Request[flowv1.NodeForDeleteRequest]) (*connect.Response[emptypb.Empty], error) {
	for _, item := range req.Msg.GetItems() {
		nodeID, err := idwrap.NewFromBytes(item.GetNodeId())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid node id: %w", err))
		}

		baseNode, err := s.ensureNodeAccess(ctx, nodeID)
		if err != nil {
			return nil, err
		}

		// Get existing For node to publish delete event
		existingFor, err := s.nfs.GetNodeFor(ctx, nodeID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		if err := s.nfs.DeleteNodeFor(ctx, nodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// Publish delete event
		if existingFor != nil {
			s.publishForEvent(forEventDelete, baseNode.FlowID, *existingFor)
		}
	}

	return connect.NewResponse(&emptypb.Empty{}), nil
}

func (s *FlowServiceV2RPC) NodeForSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeForSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeForSync(ctx, func(resp *flowv1.NodeForSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) NodeForEachCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeForEachCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeForEach, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_FOR_EACH {
				continue
			}
			nodeForEach, err := s.nfes.GetNodeForEach(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeForEach == nil {
				continue
			}
			items = append(items, serializeNodeForEach(*nodeForEach))
		}
	}

	return connect.NewResponse(&flowv1.NodeForEachCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeForEachInsert(ctx context.Context, req *connect.Request[flowv1.NodeForEachInsertRequest]) (*connect.Response[emptypb.Empty], error) {
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

func (s *FlowServiceV2RPC) NodeForEachSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeForEachSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeForEachSync(ctx, func(resp *flowv1.NodeForEachSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) NodeConditionCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeConditionCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeCondition, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_CONDITION {
				continue
			}
			nodeCondition, err := s.nifs.GetNodeIf(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			if nodeCondition == nil {
				continue
			}
			items = append(items, serializeNodeCondition(*nodeCondition))
		}
	}

	return connect.NewResponse(&flowv1.NodeConditionCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeConditionInsert(ctx context.Context, req *connect.Request[flowv1.NodeConditionInsertRequest]) (*connect.Response[emptypb.Empty], error) {
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

func (s *FlowServiceV2RPC) NodeConditionSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeConditionSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeConditionSync(ctx, func(resp *flowv1.NodeConditionSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) NodeJsCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeJsCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeJs, 0)

	for _, flow := range flows {
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		for _, n := range nodes {
			if n.NodeKind != mnnode.NODE_KIND_JS {
				continue
			}
			nodeJs, err := s.njss.GetNodeJS(ctx, n.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			items = append(items, serializeNodeJs(nodeJs))
		}
	}

	return connect.NewResponse(&flowv1.NodeJsCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeJsInsert(ctx context.Context, req *connect.Request[flowv1.NodeJsInsertRequest]) (*connect.Response[emptypb.Empty], error) {
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

func (s *FlowServiceV2RPC) NodeJsSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeJsSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeJsSync(ctx, func(resp *flowv1.NodeJsSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) NodeExecutionCollection(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
) (*connect.Response[flowv1.NodeExecutionCollectionResponse], error) {
	flows, err := s.listAccessibleFlows(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*flowv1.NodeExecution, 0)

	for _, flow := range flows {
		// Get all nodes for this flow
		nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		// For each node, get its executions
		for _, node := range nodes {
			executions, err := s.nes.ListNodeExecutionsByNodeID(ctx, node.ID)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}

			// Serialize each execution
			for _, execution := range executions {
				items = append(items, serializeNodeExecution(execution))
			}
		}
	}

	return connect.NewResponse(&flowv1.NodeExecutionCollectionResponse{Items: items}), nil
}

func (s *FlowServiceV2RPC) NodeExecutionSync(
	ctx context.Context,
	_ *connect.Request[emptypb.Empty],
	stream *connect.ServerStream[flowv1.NodeExecutionSyncResponse],
) error {
	if stream == nil {
		return connect.NewError(connect.CodeInternal, errors.New("stream is required"))
	}
	return s.streamNodeExecutionSync(ctx, func(resp *flowv1.NodeExecutionSyncResponse) error {
		return stream.Send(resp)
	})
}

func (s *FlowServiceV2RPC) streamNodeExecutionSync(
	ctx context.Context,
	send func(*flowv1.NodeExecutionSyncResponse) error,
) error {
	if s.executionStream == nil {
		return connect.NewError(connect.CodeUnavailable, errors.New("execution stream not configured"))
	}

	var flowSet sync.Map

	snapshot := func(ctx context.Context) ([]eventstream.Event[ExecutionTopic, ExecutionEvent], error) {
		flows, err := s.listAccessibleFlows(ctx)
		if err != nil {
			return nil, err
		}

		events := make([]eventstream.Event[ExecutionTopic, ExecutionEvent], 0)

		for _, flow := range flows {
			flowSet.Store(flow.ID.String(), struct{}{})

			// Get all nodes for this flow
			nodes, err := s.ns.GetNodesByFlowID(ctx, flow.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					continue
				}
				return nil, err
			}

			// For each node, get its executions
			for _, node := range nodes {
				executions, err := s.nes.ListNodeExecutionsByNodeID(ctx, node.ID)
				if err != nil {
					if errors.Is(err, sql.ErrNoRows) {
						continue
					}
					return nil, err
				}

				// Create events for each execution
				for _, execution := range executions {
					serializedExecution := serializeNodeExecution(execution)
					events = append(events, eventstream.Event[ExecutionTopic, ExecutionEvent]{
						Topic: ExecutionTopic{FlowID: flow.ID},
						Payload: ExecutionEvent{
							Type:      executionEventInsert,
							FlowID:    flow.ID,
							Execution: serializedExecution,
						},
					})
				}
			}
		}

		return events, nil
	}

	filter := func(topic ExecutionTopic) bool {
		if _, ok := flowSet.Load(topic.FlowID.String()); ok {
			return true
		}
		if err := s.ensureFlowAccess(ctx, topic.FlowID); err != nil {
			return false
		}
		flowSet.Store(topic.FlowID.String(), struct{}{})
		return true
	}

	events, err := s.executionStream.Subscribe(ctx, filter, eventstream.WithSnapshot(snapshot))
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				return nil
			}
			resp, err := s.executionEventToSyncResponse(ctx, evt.Payload)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert execution event: %w", err))
			}
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

func (s *FlowServiceV2RPC) executionEventToSyncResponse(
	ctx context.Context,
	evt ExecutionEvent,
) (*flowv1.NodeExecutionSyncResponse, error) {
	if evt.Execution == nil {
		return nil, nil
	}

	var syncEvent *flowv1.NodeExecutionSync
	switch evt.Type {
	case executionEventInsert:
		syncEvent = &flowv1.NodeExecutionSync{
			Value: &flowv1.NodeExecutionSync_ValueUnion{
				Kind: flowv1.NodeExecutionSync_ValueUnion_KIND_INSERT,
				Insert: &flowv1.NodeExecutionSyncInsert{
					NodeExecutionId: evt.Execution.NodeExecutionId,
					NodeId:          evt.Execution.NodeId,
					Name:            evt.Execution.Name,
					State:           evt.Execution.State,
				},
			},
		}

		// Add optional fields to INSERT event
		if evt.Execution.Error != nil {
			syncEvent.Value.GetInsert().Error = evt.Execution.Error
		}
		if evt.Execution.Input != nil {
			syncEvent.Value.GetInsert().Input = evt.Execution.Input
		}
		if evt.Execution.Output != nil {
			syncEvent.Value.GetInsert().Output = evt.Execution.Output
		}
		if evt.Execution.HttpResponseId != nil {
			syncEvent.Value.GetInsert().HttpResponseId = evt.Execution.HttpResponseId
		}
		if evt.Execution.CompletedAt != nil {
			syncEvent.Value.GetInsert().CompletedAt = evt.Execution.CompletedAt
		}

	case executionEventUpdate:
		syncEvent = &flowv1.NodeExecutionSync{
			Value: &flowv1.NodeExecutionSync_ValueUnion{
				Kind: flowv1.NodeExecutionSync_ValueUnion_KIND_UPDATE,
				Update: &flowv1.NodeExecutionSyncUpdate{
					NodeExecutionId: evt.Execution.NodeExecutionId,
				},
			},
		}

		// Add optional fields to UPDATE event
		update := syncEvent.Value.GetUpdate()

		// Only include NodeId if it's being updated
		if evt.Execution.NodeId != nil {
			update.NodeId = evt.Execution.NodeId
		}

		// Only include Name if it's being updated
		if evt.Execution.Name != "" {
			update.Name = &evt.Execution.Name
		}

		// Only include State if it's being updated
		if evt.Execution.State != flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED {
			update.State = &evt.Execution.State
		}

		// Handle Error union
		if evt.Execution.Error != nil {
			update.Error = &flowv1.NodeExecutionSyncUpdate_ErrorUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_ErrorUnion_KIND_VALUE,
				Value: evt.Execution.Error,
			}
		}

		// Handle Input union
		if evt.Execution.Input != nil {
			update.Input = &flowv1.NodeExecutionSyncUpdate_InputUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_InputUnion_KIND_VALUE,
				Value: evt.Execution.Input,
			}
		}

		// Handle Output union
		if evt.Execution.Output != nil {
			update.Output = &flowv1.NodeExecutionSyncUpdate_OutputUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_OutputUnion_KIND_VALUE,
				Value: evt.Execution.Output,
			}
		}

		// Handle HttpResponseId union
		if evt.Execution.HttpResponseId != nil {
			update.HttpResponseId = &flowv1.NodeExecutionSyncUpdate_HttpResponseIdUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_HttpResponseIdUnion_KIND_VALUE,
				Value: evt.Execution.HttpResponseId,
			}
		}

		// Handle CompletedAt union
		if evt.Execution.CompletedAt != nil {
			update.CompletedAt = &flowv1.NodeExecutionSyncUpdate_CompletedAtUnion{
				Kind:  flowv1.NodeExecutionSyncUpdate_CompletedAtUnion_KIND_VALUE,
				Value: evt.Execution.CompletedAt,
			}
		}

	case executionEventDelete:
		syncEvent = &flowv1.NodeExecutionSync{
			Value: &flowv1.NodeExecutionSync_ValueUnion{
				Kind: flowv1.NodeExecutionSync_ValueUnion_KIND_DELETE,
				Delete: &flowv1.NodeExecutionSyncDelete{
					NodeExecutionId: evt.Execution.NodeExecutionId,
				},
			},
		}
	default:
		return nil, nil
	}

	return &flowv1.NodeExecutionSyncResponse{
		Items: []*flowv1.NodeExecutionSync{syncEvent},
	}, nil
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

func buildFlowSyncInserts(flows []mflow.Flow) []*flowv1.FlowSync {
	if len(flows) == 0 {
		return nil
	}

	sort.Slice(flows, func(i, j int) bool {
		return bytes.Compare(flows[i].ID.Bytes(), flows[j].ID.Bytes()) < 0
	})

	items := make([]*flowv1.FlowSync, 0, len(flows))
	for _, flow := range flows {
		insert := &flowv1.FlowSyncInsert{
			FlowId: flow.ID.Bytes(),
			Name:   flow.Name,
		}
		if flow.Duration != 0 {
			duration := flow.Duration
			insert.Duration = &duration
		}

		items = append(items, &flowv1.FlowSync{
			Value: &flowv1.FlowSync_ValueUnion{
				Kind:   flowv1.FlowSync_ValueUnion_KIND_INSERT,
				Insert: insert,
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

	switch evt.Type {
	case nodeEventInsert:
		insert := &flowv1.NodeSyncInsert{
			NodeId:   node.GetNodeId(),
			FlowId:   node.GetFlowId(),
			Kind:     node.GetKind(),
			Name:     node.GetName(),
			Position: node.GetPosition(),
			State:    node.GetState(),
		}
		if info := node.GetInfo(); info != "" {
			insert.Info = &info
		}
		return &flowv1.NodeSyncResponse{
			Items: []*flowv1.NodeSync{{
				Value: &flowv1.NodeSync_ValueUnion{
					Kind:   flowv1.NodeSync_ValueUnion_KIND_INSERT,
					Insert: insert,
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
		if state := node.GetState(); state != flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED {
			st := state
			update.State = &st
		}
		if info := node.GetInfo(); info != "" {
			update.Info = &flowv1.NodeSyncUpdate_InfoUnion{
				Kind:  flowv1.NodeSyncUpdate_InfoUnion_KIND_VALUE,
				Value: &info,
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
	case edgeEventInsert:
		insert := &flowv1.EdgeSyncInsert{
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
					Kind:   flowv1.EdgeSync_ValueUnion_KIND_INSERT,
					Insert: insert,
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
		if handle := edgePB.GetSourceHandle(); handle != flowv1.HandleKind_HANDLE_KIND_UNSPECIFIED {
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

func flowVariableEventToSyncResponse(evt FlowVariableEvent) *flowv1.FlowVariableSyncResponse {
	variable := evt.Variable

	switch evt.Type {
	case flowVarEventInsert:
		insert := &flowv1.FlowVariableSyncInsert{
			FlowVariableId: variable.ID.Bytes(),
			FlowId:         variable.FlowID.Bytes(),
			Key:            variable.Name,
			Enabled:        variable.Enabled,
			Value:          variable.Value,
			Description:    variable.Description,
			Order:          evt.Order,
		}
		return &flowv1.FlowVariableSyncResponse{
			Items: []*flowv1.FlowVariableSync{{
				Value: &flowv1.FlowVariableSync_ValueUnion{
					Kind:   flowv1.FlowVariableSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case flowVarEventUpdate:
		update := &flowv1.FlowVariableSyncUpdate{
			FlowVariableId: variable.ID.Bytes(),
		}
		if flowID := variable.FlowID.Bytes(); len(flowID) > 0 {
			update.FlowId = flowID
		}
		key := variable.Name
		update.Key = &key
		enabled := variable.Enabled
		update.Enabled = &enabled
		value := variable.Value
		update.Value = &value
		description := variable.Description
		update.Description = &description
		order := evt.Order
		update.Order = &order

		return &flowv1.FlowVariableSyncResponse{
			Items: []*flowv1.FlowVariableSync{{
				Value: &flowv1.FlowVariableSync_ValueUnion{
					Kind:   flowv1.FlowVariableSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case flowVarEventDelete:
		return &flowv1.FlowVariableSyncResponse{
			Items: []*flowv1.FlowVariableSync{{
				Value: &flowv1.FlowVariableSync_ValueUnion{
					Kind: flowv1.FlowVariableSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.FlowVariableSyncDelete{
						FlowVariableId: variable.ID.Bytes(),
					},
				},
			}},
		}
	default:
		return nil
	}
}

func flowVersionEventToSyncResponse(evt FlowVersionEvent) *flowv1.FlowVersionSyncResponse {
	if evt.VersionID == (idwrap.IDWrap{}) {
		return nil
	}

	switch evt.Type {
	case flowVersionEventInsert:
		insert := &flowv1.FlowVersionSyncInsert{
			FlowVersionId: evt.VersionID.Bytes(),
			FlowId:        evt.FlowID.Bytes(),
		}
		return &flowv1.FlowVersionSyncResponse{
			Items: []*flowv1.FlowVersionSync{
				{
					Value: &flowv1.FlowVersionSync_ValueUnion{
						Kind:   flowv1.FlowVersionSync_ValueUnion_KIND_INSERT,
						Insert: insert,
					},
				},
			},
		}
	case flowVersionEventUpdate:
		update := &flowv1.FlowVersionSyncUpdate{
			FlowVersionId: evt.VersionID.Bytes(),
		}
		if evt.FlowID != (idwrap.IDWrap{}) {
			update.FlowId = evt.FlowID.Bytes()
		}
		return &flowv1.FlowVersionSyncResponse{
			Items: []*flowv1.FlowVersionSync{
				{
					Value: &flowv1.FlowVersionSync_ValueUnion{
						Kind:   flowv1.FlowVersionSync_ValueUnion_KIND_UPDATE,
						Update: update,
					},
				},
			},
		}
	case flowVersionEventDelete:
		return &flowv1.FlowVersionSyncResponse{
			Items: []*flowv1.FlowVersionSync{
				{
					Value: &flowv1.FlowVersionSync_ValueUnion{
						Kind: flowv1.FlowVersionSync_ValueUnion_KIND_DELETE,
						Delete: &flowv1.FlowVersionSyncDelete{
							FlowVersionId: evt.VersionID.Bytes(),
						},
					},
				},
			},
		}
	default:
		return nil
	}
}

func noopEventToSyncResponse(evt NoOpEvent) *flowv1.NodeNoOpSyncResponse {
	if evt.Node == nil {
		return nil
	}

	node := evt.Node

	switch evt.Type {
	case noopEventInsert:
		insert := &flowv1.NodeNoOpSyncInsert{
			NodeId: node.GetNodeId(),
			Kind:   node.GetKind(),
		}
		return &flowv1.NodeNoOpSyncResponse{
			Items: []*flowv1.NodeNoOpSync{{
				Value: &flowv1.NodeNoOpSync_ValueUnion{
					Kind:   flowv1.NodeNoOpSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case noopEventUpdate:
		update := &flowv1.NodeNoOpSyncUpdate{
			NodeId: node.GetNodeId(),
		}
		if kind := node.GetKind(); kind != flowv1.NodeNoOpKind_NODE_NO_OP_KIND_UNSPECIFIED {
			k := kind
			update.Kind = &k
		}
		return &flowv1.NodeNoOpSyncResponse{
			Items: []*flowv1.NodeNoOpSync{{
				Value: &flowv1.NodeNoOpSync_ValueUnion{
					Kind:   flowv1.NodeNoOpSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case noopEventDelete:
		return &flowv1.NodeNoOpSyncResponse{
			Items: []*flowv1.NodeNoOpSync{{
				Value: &flowv1.NodeNoOpSync_ValueUnion{
					Kind: flowv1.NodeNoOpSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.NodeNoOpSyncDelete{
						NodeId: node.GetNodeId(),
					},
				},
			}},
		}
	default:
		return nil
	}
}

func forEventToSyncResponse(evt ForEvent) *flowv1.NodeForSyncResponse {
	if evt.Node == nil {
		return nil
	}

	node := evt.Node

	switch evt.Type {
	case forEventInsert:
		insert := &flowv1.NodeForSyncInsert{
			NodeId:        node.GetNodeId(),
			Iterations:    node.GetIterations(),
			Condition:     node.GetCondition(),
			ErrorHandling: node.GetErrorHandling(),
		}
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind:   flowv1.NodeForSync_ValueUnion_KIND_INSERT,
					Insert: insert,
				},
			}},
		}
	case forEventUpdate:
		update := &flowv1.NodeForSyncUpdate{
			NodeId: node.GetNodeId(),
		}
		if iterations := node.GetIterations(); iterations != 0 {
			update.Iterations = &iterations
		}
		if condition := node.GetCondition(); condition != "" {
			update.Condition = &condition
		}
		if errorHandling := node.GetErrorHandling(); errorHandling != flowv1.ErrorHandling_ERROR_HANDLING_UNSPECIFIED {
			update.ErrorHandling = &errorHandling
		}
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind:   flowv1.NodeForSync_ValueUnion_KIND_UPDATE,
					Update: update,
				},
			}},
		}
	case forEventDelete:
		return &flowv1.NodeForSyncResponse{
			Items: []*flowv1.NodeForSync{{
				Value: &flowv1.NodeForSync_ValueUnion{
					Kind: flowv1.NodeForSync_ValueUnion_KIND_DELETE,
					Delete: &flowv1.NodeForSyncDelete{
						NodeId: node.GetNodeId(),
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
	urlEncodedVals := make([]mhttp.HTTPBodyUrlencoded, 0, len(urlEncoded))
	for _, v := range urlEncoded {
		if v != nil {
			urlEncodedVals = append(urlEncodedVals, *v)
		}
	}

	var rawBody *mhttp.HTTPBodyRaw
	if s.hbr != nil {
		rawBody, err = s.hbr.GetByHttpID(ctx, cfg.HttpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, shttp.ErrNoHttpBodyRawFound) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load http body raw: %w", err))
		}
	}

	var asserts []mhttp.HTTPAssert
	if s.has != nil {
		asserts, err = s.has.GetByHttpID(ctx, cfg.HttpID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load http asserts: %w", err))
		}
	}

	requestNode := nrequest.New(
		nodeModel.ID,
		nodeModel.Name,
		*httpRecord,
		headers,
		queries,
		rawBody,
		forms,
		urlEncodedVals,
		asserts,
		client,
		respChan,
		s.logger,
	)
	return requestNode, nil
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
		SourceHandle: flowv1.HandleKind(e.SourceHandler),
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
		Kind:     converter.ToAPINodeKind(n.NodeKind),
		Name:     n.Name,
		Position: position,
		State:    flowv1.FlowItemState_FLOW_ITEM_STATE_UNSPECIFIED,
	}
}

func serializeNodeHTTP(n mnrequest.MNRequest) *flowv1.NodeHttp {
	return &flowv1.NodeHttp{
		NodeId: n.FlowNodeID.Bytes(),
		HttpId: n.HttpID.Bytes(),
	}
}

func serializeNodeNoop(n mnnoop.NoopNode) *flowv1.NodeNoOp {
	return &flowv1.NodeNoOp{
		NodeId: n.FlowNodeID.Bytes(),
		Kind:   converter.ToAPINodeNoOpKind(n.Type),
	}
}

func serializeNodeFor(n mnfor.MNFor) *flowv1.NodeFor {
	return &flowv1.NodeFor{
		NodeId:        n.FlowNodeID.Bytes(),
		Iterations:    int32(n.IterCount),
		Condition:     n.Condition.Comparisons.Expression,
		ErrorHandling: converter.ToAPIErrorHandling(n.ErrorHandling),
	}
}

func serializeNodeCondition(n mnif.MNIF) *flowv1.NodeCondition {
	return &flowv1.NodeCondition{
		NodeId:    n.FlowNodeID.Bytes(),
		Condition: n.Condition.Comparisons.Expression,
	}
}

func serializeNodeForEach(n mnforeach.MNForEach) *flowv1.NodeForEach {
	return &flowv1.NodeForEach{
		NodeId:        n.FlowNodeID.Bytes(),
		Path:          n.IterExpression,
		Condition:     n.Condition.Comparisons.Expression,
		ErrorHandling: converter.ToAPIErrorHandling(n.ErrorHandling),
	}
}

func serializeNodeJs(n mnjs.MNJS) *flowv1.NodeJs {
	return &flowv1.NodeJs{
		NodeId: n.FlowNodeID.Bytes(),
		Code:   string(n.Code),
	}
}

func serializeNodeExecution(execution mnodeexecution.NodeExecution) *flowv1.NodeExecution {
	result := &flowv1.NodeExecution{
		NodeExecutionId: execution.ID.Bytes(),
		NodeId:          execution.NodeID.Bytes(),
		Name:            execution.Name,
		State:           flowv1.FlowItemState(execution.State),
	}

	// Handle optional fields
	if execution.Error != nil {
		result.Error = execution.Error
	}

	// Handle input data - decompress if needed
	if execution.InputData != nil {
		if inputDataJSON, err := execution.GetInputJSON(); err == nil && len(inputDataJSON) > 0 {
			if inputValue, err := structpb.NewValue(string(inputDataJSON)); err == nil {
				result.Input = inputValue
			}
		}
	}

	// Handle output data - decompress if needed
	if execution.OutputData != nil {
		if outputDataJSON, err := execution.GetOutputJSON(); err == nil && len(outputDataJSON) > 0 {
			if outputValue, err := structpb.NewValue(string(outputDataJSON)); err == nil {
				result.Output = outputValue
			}
		}
	}

	// Handle HTTP response ID
	if execution.ResponseID != nil {
		result.HttpResponseId = execution.ResponseID.Bytes()
	}

	// Handle completion timestamp
	if execution.CompletedAt != nil {
		result.CompletedAt = timestamppb.New(time.Unix(*execution.CompletedAt, 0))
	}

	return result
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

func convertHandle(h flowv1.HandleKind) edge.EdgeHandle {
	return edge.EdgeHandle(h)
}

func (s *FlowServiceV2RPC) flowVariableOrder(ctx context.Context, flowID, variableID idwrap.IDWrap) (float32, error) {
	variables, err := s.fvs.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
	if err != nil {
		return 0, err
	}
	for idx, item := range variables {
		if item.ID == variableID {
			return float32(idx), nil
		}
	}
	return 0, fmt.Errorf("flow variable %s not found in flow %s", variableID.String(), flowID.String())
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

func (s *FlowServiceV2RPC) deserializeNodeInsert(item *flowv1.NodeInsert) (*mnnode.MNode, error) {
	if item == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node insert item is required"))
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

// ImportYAMLFlow imports a YAML flow definition into the workspace
func (s *FlowServiceV2RPC) ImportYAMLFlow(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*ImportResults, error) {
	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Import using the v2 workspace import service
	results, err := s.workspaceImportService.ImportWorkspaceFromYAML(ctx, data, workspaceID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to import YAML flow: %w", err))
	}

	return results, nil
}

// ImportYAMLFlowSimple imports a YAML flow with simple options
func (s *FlowServiceV2RPC) ImportYAMLFlowSimple(
	ctx context.Context,
	data []byte,
	workspaceID idwrap.IDWrap,
) (*ImportResults, error) {
	// This is a simplified version that just delegates to ImportYAMLFlow
	return s.ImportYAMLFlow(ctx, data, workspaceID)
}

// ParseYAMLFlow parses YAML flow data without importing it
func (s *FlowServiceV2RPC) ParseYAMLFlow(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*yamlflowsimplev2.SimplifiedYAMLResolvedV2, error) {
	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Create conversion options
	opts := yamlflowsimplev2.GetDefaultOptions(workspaceID)
	opts.IsDelta = false
	opts.GenerateFiles = true

	// Parse the YAML data
	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(data, opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to parse YAML flow: %w", err))
	}

	return resolved, nil
}

// DetectFlowFormat detects the format of flow data (YAML, JSON, curl, etc.)
func (s *FlowServiceV2RPC) DetectFlowFormat(ctx context.Context, data []byte) (string, error) {
	// Try to detect if it's YAML
	dataStr := string(data)
	trimmedData := strings.TrimSpace(dataStr)

	// Check for curl command first (most specific)
	if strings.HasPrefix(trimmedData, "curl ") ||
		strings.Contains(dataStr, "\ncurl ") ||
		strings.Contains(dataStr, " curl ") {
		return "curl", nil
	}

	// Simple YAML detection - check for common YAML patterns
	if strings.Contains(dataStr, "flows:") ||
		strings.Contains(dataStr, "workspace_name:") ||
		strings.Contains(dataStr, "requests:") ||
		strings.Contains(dataStr, "run:") ||
		strings.Contains(dataStr, "- name:") ||
		strings.Contains(dataStr, "steps:") {
		return "yaml", nil
	}

	// Check if it's JSON
	if strings.HasPrefix(trimmedData, "{") ||
		strings.HasPrefix(trimmedData, "[") {
		return "json", nil
	}

	return "unknown", nil
}

// ValidateYAMLFlow validates YAML flow data without importing
func (s *FlowServiceV2RPC) ValidateYAMLFlow(ctx context.Context, data []byte) error {
	// Parse the YAML data to validate structure
	var yamlFormat yamlflowsimplev2.YamlFlowFormatV2
	if err := yaml.Unmarshal(data, &yamlFormat); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid YAML format: %w", err))
	}

	// Validate the YAML structure
	if err := yamlFormat.Validate(); err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("YAML validation failed: %w", err))
	}

	return nil
}

// ImportCurlCommand imports a curl command into the workspace
func (s *FlowServiceV2RPC) ImportCurlCommand(ctx context.Context, curlData []byte, workspaceID idwrap.IDWrap) (*ImportResults, error) {
	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Convert curl command to modern HTTP models
	curlOpts := tcurlv2.ConvertCurlOptions{
		WorkspaceID: workspaceID,
		Filename:    "curl_request",
	}

	resolved, err := tcurlv2.ConvertCurl(string(curlData), curlOpts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to parse curl command: %w", err))
	}

	// Create a simple YAML structure from the curl command for import
	// This allows us to reuse the existing import infrastructure
	simpleYAML := map[string]interface{}{
		"workspace_name": "Imported from Curl",
		"requests": []map[string]interface{}{
			{
				"name":        resolved.HTTP.Name,
				"method":      resolved.HTTP.Method,
				"url":         resolved.HTTP.Url,
				"description": "Imported from curl command",
			},
		},
		"flows": []map[string]interface{}{
			{
				"name": "Curl Import Flow",
				"steps": []map[string]interface{}{
					{
						"type":    "request",
						"request": resolved.HTTP.Name,
					},
				},
			},
		},
	}

	// Convert to YAML bytes
	yamlData, err := yaml.Marshal(simpleYAML)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to convert curl to YAML: %w", err))
	}

	// Use the existing YAML import functionality
	return s.ImportYAMLFlow(ctx, yamlData, workspaceID)
}

// ParseCurlCommand parses a curl command without importing it
func (s *FlowServiceV2RPC) ParseCurlCommand(ctx context.Context, curlData []byte, workspaceID idwrap.IDWrap) (*tcurlv2.CurlResolvedV2, error) {
	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	// Parse the curl command
	curlOpts := tcurlv2.ConvertCurlOptions{
		WorkspaceID: workspaceID,
		Filename:    "curl_request",
	}

	resolved, err := tcurlv2.ConvertCurl(string(curlData), curlOpts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to parse curl command: %w", err))
	}

	return resolved, nil
}

// ParseFlowData parses flow data without importing it (supports YAML, JSON, curl)
func (s *FlowServiceV2RPC) ParseFlowData(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (interface{}, error) {
	// Detect format first
	format, err := s.DetectFlowFormat(ctx, data)
	if err != nil {
		return nil, err
	}

	// Validate workspace access
	if err := s.ensureWorkspaceAccess(ctx, workspaceID); err != nil {
		return nil, err
	}

	switch format {
	case "yaml":
		return s.ParseYAMLFlow(ctx, data, workspaceID)
	case "json":
		// For JSON, try to parse as YAML first (YAML is a superset of JSON)
		return s.ParseYAMLFlow(ctx, data, workspaceID)
	case "curl":
		return s.ParseCurlCommand(ctx, data, workspaceID)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported format: %s", format))
	}
}

// Ensure FlowServiceV2RPC implements the generated interface.
var _ flowv1connect.FlowServiceHandler = (*FlowServiceV2RPC)(nil)
