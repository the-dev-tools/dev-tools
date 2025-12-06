package rflowv2

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"connectrpc.com/connect"

	"the-dev-tools/server/internal/api"
	"the-dev-tools/server/internal/api/rhttp"
	"the-dev-tools/server/internal/api/rlog"
	"the-dev-tools/server/pkg/eventstream"
	"the-dev-tools/server/pkg/http/resolver"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/service/flow/sedge"
	"the-dev-tools/server/pkg/service/senv"
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
	"the-dev-tools/server/pkg/service/svar"
	"the-dev-tools/server/pkg/service/sworkspace"
	flowv1 "the-dev-tools/spec/dist/buf/go/api/flow/v1"
	"the-dev-tools/spec/dist/buf/go/api/flow/v1/flowv1connect"
	"the-dev-tools/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
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
	ws       *sworkspace.WorkspaceService
	fs       *sflow.FlowService
	es       *sedge.EdgeService
	ns       *snode.NodeService
	nrs      *snoderequest.NodeRequestService
	nfs      *snodefor.NodeForService
	nfes     *snodeforeach.NodeForEachService
	nifs     *snodeif.NodeIfService
	nnos     *snodenoop.NodeNoopService
	njss     *snodejs.NodeJSService
	nes      *snodeexecution.NodeExecutionService
	fvs      *sflowvariable.FlowVariableService
	envs     *senv.EnvironmentService
	vs       *svar.VarService
	hs       *shttp.HTTPService
	hbr      *shttp.HttpBodyRawService
	resolver resolver.RequestResolver
	logger   *slog.Logger
	// V2 import services
	workspaceImportService   WorkspaceImporter
	httpResponseService      shttp.HttpResponseService
	flowStream               eventstream.SyncStreamer[FlowTopic, FlowEvent]
	nodeStream               eventstream.SyncStreamer[NodeTopic, NodeEvent]
	edgeStream               eventstream.SyncStreamer[EdgeTopic, EdgeEvent]
	varStream                eventstream.SyncStreamer[FlowVariableTopic, FlowVariableEvent]
	versionStream            eventstream.SyncStreamer[FlowVersionTopic, FlowVersionEvent]
	noopStream               eventstream.SyncStreamer[NoOpTopic, NoOpEvent]
	forStream                eventstream.SyncStreamer[ForTopic, ForEvent]
	conditionStream          eventstream.SyncStreamer[ConditionTopic, ConditionEvent]
	forEachStream            eventstream.SyncStreamer[ForEachTopic, ForEachEvent]
	jsStream                 eventstream.SyncStreamer[JsTopic, JsEvent]
	executionStream          eventstream.SyncStreamer[ExecutionTopic, ExecutionEvent]
	httpResponseStream       eventstream.SyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent]
	httpResponseHeaderStream eventstream.SyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent]
	httpResponseAssertStream eventstream.SyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent]
	logStream                eventstream.SyncStreamer[rlog.LogTopic, rlog.LogEvent]

	// JS executor client for running JS nodes (connects to worker-js)
	jsClient node_js_executorv1connect.NodeJsExecutorServiceClient

	// Running flows map for cancellation
	runningFlowsMu sync.Mutex
	runningFlows   map[string]context.CancelFunc
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
	envs *senv.EnvironmentService,
	vs *svar.VarService,
	hs *shttp.HTTPService,
	hbr *shttp.HttpBodyRawService,
	resolver resolver.RequestResolver,
	logger *slog.Logger,
	workspaceImportService WorkspaceImporter,
	httpResponseService shttp.HttpResponseService,
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
	httpResponseStream eventstream.SyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent],
	httpResponseHeaderStream eventstream.SyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent],
	httpResponseAssertStream eventstream.SyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent],
	logStream eventstream.SyncStreamer[rlog.LogTopic, rlog.LogEvent],
	jsClient node_js_executorv1connect.NodeJsExecutorServiceClient,
) *FlowServiceV2RPC {
	return &FlowServiceV2RPC{
		ws:                       ws,
		fs:                       fs,
		es:                       es,
		ns:                       ns,
		nrs:                      nrs,
		nfs:                      nfs,
		nfes:                     nfes,
		nifs:                     nifs,
		nnos:                     nnos,
		njss:                     njss,
		nes:                      nes,
		fvs:                      fvs,
		envs:                     envs,
		vs:                       vs,
		hs:                       hs,
		hbr:                      hbr,
		resolver:                 resolver,
		logger:                   logger,
		workspaceImportService:   workspaceImportService,
		httpResponseService:      httpResponseService,
		flowStream:               flowStream,
		nodeStream:               nodeStream,
		edgeStream:               edgeStream,
		varStream:                varStream,
		versionStream:            versionStream,
		noopStream:               noopStream,
		forStream:                forStream,
		conditionStream:          conditionStream,
		forEachStream:            forEachStream,
		jsStream:                 jsStream,
		executionStream:          executionStream,
		httpResponseStream:       httpResponseStream,
		httpResponseHeaderStream: httpResponseHeaderStream,
		httpResponseAssertStream: httpResponseAssertStream,
		logStream:                logStream,
		jsClient:                 jsClient,
		runningFlows:             make(map[string]context.CancelFunc),
	}
}

func CreateService(srv *FlowServiceV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := flowv1connect.NewFlowServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// Ensure FlowServiceV2RPC implements the generated interface.
var _ flowv1connect.FlowServiceHandler = (*FlowServiceV2RPC)(nil)
