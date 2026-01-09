//nolint:revive // exported
package rflowv2

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	"connectrpc.com/connect"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rfile"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/rlog"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/eventstream"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/http/resolver"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/mutation"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
	flowv1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/flow/v1/flowv1connect"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/node_js_executor/v1/node_js_executorv1connect"
)

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
	Variable mflow.FlowVariable
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

// Data structures for mutation payloads
type flowNodePair struct {
	flow        mflow.Flow
	startNode   mflow.Node
	workspaceID idwrap.IDWrap
}

type nodeHttpWithFlow struct {
	nodeRequest mflow.NodeRequest
	flowID      idwrap.IDWrap
	baseNode    *mflow.Node
}

type nodeConditionWithFlow struct {
	nodeIf   mflow.NodeIf
	flowID   idwrap.IDWrap
	baseNode *mflow.Node
}

type nodeForEachWithFlow struct {
	nodeForEach mflow.NodeForEach
	flowID      idwrap.IDWrap
	baseNode    *mflow.Node
}

type nodeJsWithFlow struct {
	nodeJS   mflow.NodeJS
	flowID   idwrap.IDWrap
	baseNode *mflow.Node
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

	forEventInsert = "insert"
	forEventUpdate = "update"
	forEventDelete = "delete"

	jsEventInsert = "insert"
	jsEventUpdate = "update"
	jsEventDelete = "delete"

	executionEventInsert = "insert"
	executionEventUpdate = "update"
	executionEventDelete = "delete"
)

type FlowServiceV2Readers struct {
	Workspace     *sworkspace.WorkspaceReader
	Flow          *sflow.FlowReader
	Node          *sflow.NodeReader
	Env           *senv.EnvReader
	Http          *shttp.Reader
	Edge          *sflow.EdgeReader
	NodeRequest   *sflow.NodeRequestReader
	FlowVariable  *sflow.FlowVariableReader
	NodeExecution *sflow.NodeExecutionReader
	HttpResponse  *shttp.HttpResponseReader
}

func (r *FlowServiceV2Readers) Validate() error {
	if r.Workspace == nil {
		return fmt.Errorf("workspace reader is required")
	}
	if r.Flow == nil {
		return fmt.Errorf("flow reader is required")
	}
	if r.Node == nil {
		return fmt.Errorf("node reader is required")
	}
	if r.Env == nil {
		return fmt.Errorf("env reader is required")
	}
	if r.Http == nil {
		return fmt.Errorf("http reader is required")
	}
	if r.Edge == nil {
		return fmt.Errorf("edge reader is required")
	}
	return nil
}

type FlowServiceV2Services struct {
	Workspace     *sworkspace.WorkspaceService
	Flow          *sflow.FlowService
	Edge          *sflow.EdgeService
	Node          *sflow.NodeService
	NodeRequest   *sflow.NodeRequestService
	NodeFor       *sflow.NodeForService
	NodeForEach   *sflow.NodeForEachService
	NodeIf        *sflow.NodeIfService
	NodeJs        *sflow.NodeJsService
	NodeExecution *sflow.NodeExecutionService
	FlowVariable  *sflow.FlowVariableService
	Env           *senv.EnvironmentService
	Var           *senv.VariableService
	Http          *shttp.HTTPService
	HttpBodyRaw   *shttp.HttpBodyRawService
	HttpResponse  shttp.HttpResponseService
	File          *sfile.FileService
	Importer      WorkspaceImporter
}

func (s *FlowServiceV2Services) Validate() error {
	if s.Workspace == nil {
		return fmt.Errorf("workspace service is required")
	}
	if s.Flow == nil {
		return fmt.Errorf("flow service is required")
	}
	if s.Edge == nil {
		return fmt.Errorf("edge service is required")
	}
	if s.Node == nil {
		return fmt.Errorf("node service is required")
	}
	if s.NodeRequest == nil {
		return fmt.Errorf("node request service is required")
	}
	if s.NodeFor == nil {
		return fmt.Errorf("node for service is required")
	}
	if s.NodeForEach == nil {
		return fmt.Errorf("node for each service is required")
	}
	if s.NodeIf == nil {
		return fmt.Errorf("node if service is required")
	}
	if s.NodeJs == nil {
		return fmt.Errorf("node js service is required")
	}
	if s.NodeExecution == nil {
		return fmt.Errorf("node execution service is required")
	}
	if s.FlowVariable == nil {
		return fmt.Errorf("flow variable service is required")
	}
	if s.Env == nil {
		return fmt.Errorf("env service is required")
	}
	if s.Var == nil {
		return fmt.Errorf("var service is required")
	}
	if s.Http == nil {
		return fmt.Errorf("http service is required")
	}
	if s.HttpBodyRaw == nil {
		return fmt.Errorf("http body raw service is required")
	}
	return nil
}

type FlowServiceV2Streamers struct {
	Flow               eventstream.SyncStreamer[FlowTopic, FlowEvent]
	Node               eventstream.SyncStreamer[NodeTopic, NodeEvent]
	Edge               eventstream.SyncStreamer[EdgeTopic, EdgeEvent]
	Var                eventstream.SyncStreamer[FlowVariableTopic, FlowVariableEvent]
	Version            eventstream.SyncStreamer[FlowVersionTopic, FlowVersionEvent]
	For                eventstream.SyncStreamer[ForTopic, ForEvent]
	Condition          eventstream.SyncStreamer[ConditionTopic, ConditionEvent]
	ForEach            eventstream.SyncStreamer[ForEachTopic, ForEachEvent]
	Js                 eventstream.SyncStreamer[JsTopic, JsEvent]
	Execution          eventstream.SyncStreamer[ExecutionTopic, ExecutionEvent]
	HttpResponse       eventstream.SyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent]
	HttpResponseHeader eventstream.SyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent]
	HttpResponseAssert eventstream.SyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent]
	Log                eventstream.SyncStreamer[rlog.LogTopic, rlog.LogEvent]
	File               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]
}

type FlowServiceV2Deps struct {
	DB        *sql.DB
	Readers   FlowServiceV2Readers
	Services  FlowServiceV2Services
	Streamers FlowServiceV2Streamers
	Resolver  resolver.RequestResolver
	Logger    *slog.Logger
	JsClient  node_js_executorv1connect.NodeJsExecutorServiceClient
}

func (d *FlowServiceV2Deps) Validate() error {
	if d.DB == nil {
		return fmt.Errorf("db is required")
	}
	if err := d.Readers.Validate(); err != nil {
		return err
	}
	if err := d.Services.Validate(); err != nil {
		return err
	}
	if d.Resolver == nil {
		return fmt.Errorf("resolver is required")
	}
	if d.Logger == nil {
		return fmt.Errorf("logger is required")
	}
	return nil
}

type FlowServiceV2RPC struct {
	DB *sql.DB

	wsReader       *sworkspace.WorkspaceReader
	fsReader       *sflow.FlowReader
	nsReader       *sflow.NodeReader
	vsReader       *senv.EnvReader
	hsReader       *shttp.Reader
	flowEdgeReader *sflow.EdgeReader

	ws       *sworkspace.WorkspaceService
	fs       *sflow.FlowService
	es       *sflow.EdgeService
	ns       *sflow.NodeService
	nrs      *sflow.NodeRequestService
	nfs      *sflow.NodeForService
	nfes     *sflow.NodeForEachService
	nifs     *sflow.NodeIfService
	njss     *sflow.NodeJsService
	nes      *sflow.NodeExecutionService
	fvs      *sflow.FlowVariableService
	envs     *senv.EnvironmentService
	vs       *senv.VariableService
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
	forStream                eventstream.SyncStreamer[ForTopic, ForEvent]
	conditionStream          eventstream.SyncStreamer[ConditionTopic, ConditionEvent]
	forEachStream            eventstream.SyncStreamer[ForEachTopic, ForEachEvent]
	jsStream                 eventstream.SyncStreamer[JsTopic, JsEvent]
	executionStream          eventstream.SyncStreamer[ExecutionTopic, ExecutionEvent]
	httpResponseStream       eventstream.SyncStreamer[rhttp.HttpResponseTopic, rhttp.HttpResponseEvent]
	httpResponseHeaderStream eventstream.SyncStreamer[rhttp.HttpResponseHeaderTopic, rhttp.HttpResponseHeaderEvent]
	httpResponseAssertStream eventstream.SyncStreamer[rhttp.HttpResponseAssertTopic, rhttp.HttpResponseAssertEvent]
	logStream                eventstream.SyncStreamer[rlog.LogTopic, rlog.LogEvent]
	fileService              *sfile.FileService
	fileStream               eventstream.SyncStreamer[rfile.FileTopic, rfile.FileEvent]

	// JS executor client for running JS nodes (connects to worker-js)
	jsClient node_js_executorv1connect.NodeJsExecutorServiceClient

	// Shared builder for flow execution
	builder *flowbuilder.Builder

	// Running flows map for cancellation
	runningFlowsMu sync.Mutex
	runningFlows   map[string]context.CancelFunc
}

func New(deps FlowServiceV2Deps) *FlowServiceV2RPC {
	if err := deps.Validate(); err != nil {
		panic(fmt.Sprintf("FlowServiceV2 Deps validation failed: %v", err))
	}

	builder := flowbuilder.New(
		deps.Services.Node, deps.Services.NodeRequest, deps.Services.NodeFor, deps.Services.NodeForEach,
		deps.Services.NodeIf, deps.Services.NodeJs,
		deps.Services.Workspace, deps.Services.Var, deps.Services.FlowVariable,
		deps.Resolver, deps.Logger,
	)

	return &FlowServiceV2RPC{
		DB:                       deps.DB,
		wsReader:                 deps.Readers.Workspace,
		fsReader:                 deps.Readers.Flow,
		nsReader:                 deps.Readers.Node,
		vsReader:                 deps.Readers.Env,
		hsReader:                 deps.Readers.Http,
		flowEdgeReader:           deps.Readers.Edge,
		ws:                       deps.Services.Workspace,
		fs:                       deps.Services.Flow,
		es:                       deps.Services.Edge,
		ns:                       deps.Services.Node,
		nrs:                      deps.Services.NodeRequest,
		nfs:                      deps.Services.NodeFor,
		nfes:                     deps.Services.NodeForEach,
		nifs:                     deps.Services.NodeIf,
		njss:                     deps.Services.NodeJs,
		nes:                      deps.Services.NodeExecution,
		fvs:                      deps.Services.FlowVariable,
		envs:                     deps.Services.Env,
		vs:                       deps.Services.Var,
		hs:                       deps.Services.Http,
		hbr:                      deps.Services.HttpBodyRaw,
		resolver:                 deps.Resolver,
		logger:                   deps.Logger,
		workspaceImportService:   deps.Services.Importer,
		httpResponseService:      deps.Services.HttpResponse,
		flowStream:               deps.Streamers.Flow,
		nodeStream:               deps.Streamers.Node,
		edgeStream:               deps.Streamers.Edge,
		varStream:                deps.Streamers.Var,
		versionStream:            deps.Streamers.Version,
		forStream:                deps.Streamers.For,
		conditionStream:          deps.Streamers.Condition,
		forEachStream:            deps.Streamers.ForEach,
		jsStream:                 deps.Streamers.Js,
		executionStream:          deps.Streamers.Execution,
		httpResponseStream:       deps.Streamers.HttpResponse,
		httpResponseHeaderStream: deps.Streamers.HttpResponseHeader,
		httpResponseAssertStream: deps.Streamers.HttpResponseAssert,
		logStream:                deps.Streamers.Log,
		fileService:              deps.Services.File,
		fileStream:               deps.Streamers.File,
		jsClient:                 deps.JsClient,
		builder:                  builder,
		runningFlows:             make(map[string]context.CancelFunc),
	}
}

func CreateService(srv *FlowServiceV2RPC, options []connect.HandlerOption) (*api.Service, error) {
	path, handler := flowv1connect.NewFlowServiceHandler(srv, options...)
	return &api.Service{Path: path, Handler: handler}, nil
}

// Ensure FlowServiceV2RPC implements the generated interface.
var _ flowv1connect.FlowServiceHandler = (*FlowServiceV2RPC)(nil)

// mutationPublisher returns a unified publisher for flow-related mutation events.
func (s *FlowServiceV2RPC) mutationPublisher() mutation.Publisher {
	return &rflowPublisher{
		flowStream:      s.flowStream,
		nodeStream:      s.nodeStream,
		edgeStream:      s.edgeStream,
		varStream:       s.varStream,
		versionStream:   s.versionStream,
		forStream:       s.forStream,
		conditionStream: s.conditionStream,
		forEachStream:   s.forEachStream,
		jsStream:        s.jsStream,
	}
}

type rflowPublisher struct {
	flowStream      eventstream.SyncStreamer[FlowTopic, FlowEvent]
	nodeStream      eventstream.SyncStreamer[NodeTopic, NodeEvent]
	edgeStream      eventstream.SyncStreamer[EdgeTopic, EdgeEvent]
	varStream       eventstream.SyncStreamer[FlowVariableTopic, FlowVariableEvent]
	versionStream   eventstream.SyncStreamer[FlowVersionTopic, FlowVersionEvent]
	forStream       eventstream.SyncStreamer[ForTopic, ForEvent]
	conditionStream eventstream.SyncStreamer[ConditionTopic, ConditionEvent]
	forEachStream   eventstream.SyncStreamer[ForEachTopic, ForEachEvent]
	jsStream        eventstream.SyncStreamer[JsTopic, JsEvent]
}

func (p *rflowPublisher) PublishAll(events []mutation.Event) {
	for _, evt := range events {
		//nolint:exhaustive
		switch evt.Entity {
		case mutation.EntityFlow:
			p.publishFlow(evt)
		case mutation.EntityFlowNode:
			p.publishNode(evt)
		case mutation.EntityFlowNodeHTTP:
			p.publishNodeHttp(evt)
		case mutation.EntityFlowNodeFor:
			p.publishNodeFor(evt)
		case mutation.EntityFlowNodeCondition:
			p.publishNodeCondition(evt)
		case mutation.EntityFlowNodeForEach:
			p.publishNodeForEach(evt)
		case mutation.EntityFlowNodeJS:
			p.publishNodeJs(evt)
		case mutation.EntityFlowEdge:
			p.publishEdge(evt)
		case mutation.EntityFlowVariable:
			p.publishVariable(evt)
		}
	}
}

func (p *rflowPublisher) publishFlow(evt mutation.Event) {
	if p.flowStream == nil {
		return
	}
	var flow *flowv1.Flow
	var eventType string

	switch evt.Op {
	case mutation.OpInsert:
		eventType = flowEventInsert
		if pair, ok := evt.Payload.(flowNodePair); ok {
			flow = serializeFlow(pair.flow)
			// FlowInsert also creates a start node
			p.publishNode(mutation.Event{
				Entity:   mutation.EntityFlowNode,
				Op:       mutation.OpInsert,
				ID:       pair.startNode.ID,
				ParentID: pair.flow.ID,
				Payload:  pair.startNode,
			})
		}
	case mutation.OpUpdate:
		eventType = flowEventUpdate
		if f, ok := evt.Payload.(mflow.Flow); ok {
			flow = serializeFlow(f)
		}
	case mutation.OpDelete:
		eventType = flowEventDelete
		flow = &flowv1.Flow{
			FlowId:      evt.ID.Bytes(),
			WorkspaceId: evt.WorkspaceID.Bytes(),
		}
	}

	if flow != nil {
		p.flowStream.Publish(FlowTopic{WorkspaceID: evt.WorkspaceID}, FlowEvent{
			Type: eventType,
			Flow: flow,
		})

		// Also handle FlowVersion sync if this is a versioned flow
		var versionParentID *idwrap.IDWrap
		if f, ok := evt.Payload.(mflow.Flow); ok {
			versionParentID = f.VersionParentID
		} else if pair, ok := evt.Payload.(flowNodePair); ok {
			versionParentID = pair.flow.VersionParentID
		}

		if versionParentID != nil && p.versionStream != nil {
			var versionType string
			switch evt.Op {
			case mutation.OpInsert:
				versionType = flowVersionEventInsert
			case mutation.OpUpdate:
				versionType = flowVersionEventUpdate
			case mutation.OpDelete:
				versionType = flowVersionEventDelete
			}

			if versionType != "" {
				p.versionStream.Publish(FlowVersionTopic{FlowID: *versionParentID}, FlowVersionEvent{
					Type:      versionType,
					FlowID:    *versionParentID,
					VersionID: evt.ID,
				})
			}
		}
	}
}

func (p *rflowPublisher) publishNode(evt mutation.Event) {
	if p.nodeStream == nil {
		return
	}
	var node *flowv1.Node
	var eventType string

	switch evt.Op {
	case mutation.OpInsert:
		eventType = nodeEventInsert
		if n, ok := evt.Payload.(mflow.Node); ok {
			node = serializeNode(n)
		}
	case mutation.OpUpdate:
		eventType = nodeEventUpdate
		if n, ok := evt.Payload.(mflow.Node); ok {
			node = serializeNode(n)
		}
	case mutation.OpDelete:
		eventType = nodeEventDelete
		node = &flowv1.Node{NodeId: evt.ID.Bytes()}
	}

	if node != nil {
		p.nodeStream.Publish(NodeTopic{FlowID: evt.ParentID}, NodeEvent{
			Type:   eventType,
			FlowID: evt.ParentID,
			Node:   node,
		})
	}
}

func (p *rflowPublisher) publishEdge(evt mutation.Event) {
	if p.edgeStream == nil {
		return
	}
	var edge *flowv1.Edge
	var eventType string

	switch evt.Op {
	case mutation.OpInsert:
		eventType = edgeEventInsert
		if e, ok := evt.Payload.(mflow.Edge); ok {
			edge = serializeEdge(e)
		}
	case mutation.OpUpdate:
		eventType = edgeEventUpdate
		if e, ok := evt.Payload.(mflow.Edge); ok {
			edge = serializeEdge(e)
		}
	case mutation.OpDelete:
		eventType = edgeEventDelete
		edge = &flowv1.Edge{EdgeId: evt.ID.Bytes()}
	}

	if edge != nil {
		p.edgeStream.Publish(EdgeTopic{FlowID: evt.ParentID}, EdgeEvent{
			Type:   eventType,
			FlowID: evt.ParentID,
			Edge:   edge,
		})
	}
}

func (p *rflowPublisher) publishVariable(evt mutation.Event) {
	if p.varStream == nil {
		return
	}
	var variable mflow.FlowVariable
	var eventType string

	switch evt.Op {
	case mutation.OpInsert:
		eventType = flowVarEventInsert
		if v, ok := evt.Payload.(mflow.FlowVariable); ok {
			variable = v
		}
	case mutation.OpUpdate:
		eventType = flowVarEventUpdate
		if v, ok := evt.Payload.(mflow.FlowVariable); ok {
			variable = v
		}
	case mutation.OpDelete:
		eventType = flowVarEventDelete
		variable = mflow.FlowVariable{ID: evt.ID, FlowID: evt.ParentID}
	}

	p.varStream.Publish(FlowVariableTopic{FlowID: evt.ParentID}, FlowVariableEvent{
		Type:     eventType,
		FlowID:   evt.ParentID,
		Variable: variable,
	})
}

func (p *rflowPublisher) publishNodeHttp(evt mutation.Event) {
	if p.nodeStream == nil {
		return
	}

	var node *flowv1.Node
	var flowID idwrap.IDWrap

	// 1. Publish update to base node stream
	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		if data, ok := evt.Payload.(nodeHttpWithFlow); ok && data.baseNode != nil {
			node = serializeNode(*data.baseNode)
			flowID = data.flowID
		}
	case mutation.OpDelete:
		node = &flowv1.Node{
			NodeId: evt.ID.Bytes(),
			FlowId: evt.ParentID.Bytes(),
		}
		flowID = evt.ParentID
	}

	if node != nil {
		p.nodeStream.Publish(NodeTopic{FlowID: flowID}, NodeEvent{
			Type:   nodeEventUpdate,
			FlowID: flowID,
			Node:   node,
		})
	}

	// 2. Publish to specialized NodeHttp stream if configured
	// (Required for some tests and advanced sync)
}

func (p *rflowPublisher) publishNodeFor(evt mutation.Event) {
	if p.forStream == nil {
		return
	}
	var node *flowv1.NodeFor
	var eventType string

	switch evt.Op {
	case mutation.OpInsert:
		eventType = forEventInsert
		if n, ok := evt.Payload.(mflow.NodeFor); ok {
			node = serializeNodeFor(n)
		}
	case mutation.OpUpdate:
		eventType = forEventUpdate
		if n, ok := evt.Payload.(mflow.NodeFor); ok {
			node = serializeNodeFor(n)
		}
	case mutation.OpDelete:
		eventType = forEventDelete
		node = &flowv1.NodeFor{NodeId: evt.ID.Bytes()}
	}

	if node != nil {
		p.forStream.Publish(ForTopic{FlowID: evt.ParentID}, ForEvent{
			Type:   eventType,
			FlowID: evt.ParentID,
			Node:   node,
		})
	}
}

func (p *rflowPublisher) publishNodeCondition(evt mutation.Event) {
	if p.conditionStream == nil {
		return
	}
	var node *flowv1.NodeCondition
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		if evt.Op == mutation.OpInsert {
			eventType = "insert"
		} else {
			eventType = "update"
		}
		if data, ok := evt.Payload.(nodeConditionWithFlow); ok {
			node = serializeNodeCondition(data.nodeIf)
		} else if n, ok := evt.Payload.(mflow.NodeIf); ok {
			node = serializeNodeCondition(n)
		}
	case mutation.OpDelete:
		eventType = "delete"
		node = &flowv1.NodeCondition{NodeId: evt.ID.Bytes()}
	}

	if node != nil {
		p.conditionStream.Publish(ConditionTopic{FlowID: evt.ParentID}, ConditionEvent{
			Type:   eventType,
			FlowID: evt.ParentID,
			Node:   node,
		})
	}
}

func (p *rflowPublisher) publishNodeForEach(evt mutation.Event) {
	if p.forEachStream == nil {
		return
	}
	var node *flowv1.NodeForEach
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		if evt.Op == mutation.OpInsert {
			eventType = "insert"
		} else {
			eventType = "update"
		}
		if data, ok := evt.Payload.(nodeForEachWithFlow); ok {
			node = serializeNodeForEach(data.nodeForEach)
		} else if n, ok := evt.Payload.(mflow.NodeForEach); ok {
			node = serializeNodeForEach(n)
		}
	case mutation.OpDelete:
		eventType = "delete"
		node = &flowv1.NodeForEach{NodeId: evt.ID.Bytes()}
	}

	if node != nil {
		p.forEachStream.Publish(ForEachTopic{FlowID: evt.ParentID}, ForEachEvent{
			Type:   eventType,
			FlowID: evt.ParentID,
			Node:   node,
		})
	}
}

func (p *rflowPublisher) publishNodeJs(evt mutation.Event) {
	if p.jsStream == nil {
		return
	}
	var node *flowv1.NodeJs
	var eventType string

	switch evt.Op {
	case mutation.OpInsert, mutation.OpUpdate:
		if evt.Op == mutation.OpInsert {
			eventType = jsEventInsert
		} else {
			eventType = jsEventUpdate
		}
		if data, ok := evt.Payload.(nodeJsWithFlow); ok {
			node = serializeNodeJs(data.nodeJS)
		} else if n, ok := evt.Payload.(mflow.NodeJS); ok {
			node = serializeNodeJs(n)
		}
	case mutation.OpDelete:
		eventType = jsEventDelete
		node = &flowv1.NodeJs{NodeId: evt.ID.Bytes()}
	}

	if node != nil {
		p.jsStream.Publish(JsTopic{FlowID: evt.ParentID}, JsEvent{
			Type:   eventType,
			FlowID: evt.ParentID,
			Node:   node,
		})
	}
}
