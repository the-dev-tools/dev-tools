package flowresult

import (
	"context"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/ngraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
)

// ResultProcessor handles the side effects of flow execution:
// response persistence, execution state tracking, and event publishing.
//
// Channel accessors are exposed because local execution requires wiring
// channels between the node builder (producer) and the processor (consumer).
// In a distributed scenario, remote workers would use their own channels
// locally and send results back via RPC to a different ResultProcessor
// implementation that doesn't need these channels.
type ResultProcessor interface {
	// HTTPResponseChan returns the channel for HTTP response side-effects.
	// Pass this to the node builder so request nodes can send responses.
	HTTPResponseChan() chan nrequest.NodeRequestSideResp

	// GraphQLResponseChan returns the channel for GraphQL response side-effects.
	// Pass this to the node builder so GraphQL nodes can send responses.
	GraphQLResponseChan() chan ngraphql.NodeGraphQLSideResp

	// NodeStateChan returns the channel for node execution status events.
	// Pass this to the FlowRunner via FlowEventChannels.NodeStates.
	NodeStateChan() chan runner.FlowNodeStatus

	// Start begins the drain goroutines that process responses and status events.
	// Must be called before the flow runner starts.
	Start()

	// Wait blocks until all processing is complete.
	// The runner must have finished (closing NodeStateChan) before Wait returns.
	// Wait also closes the response channels and waits for their drains.
	Wait()
}

// ServerResultProcessorOpts configures a ServerResultProcessor.
type ServerResultProcessorOpts struct {
	FlowID      idwrap.IDWrap
	WorkspaceID idwrap.IDWrap

	// Flow data — maps are built internally from these
	Nodes         []mflow.Node
	Edges         []mflow.Edge
	NodeIDMapping map[string]idwrap.IDWrap // original → versioned node ID mapping

	// Services for persistence
	HTTPResponseService    shttp.HttpResponseService
	GraphQLResponseService sgraphql.GraphQLResponseService
	NodeExecutionService   *sflow.NodeExecutionService
	NodeService            *sflow.NodeService
	EdgeService            *sflow.EdgeService

	// Event publishing
	Publisher EventPublisher
	Logger    *slog.Logger
}

// ServerResultProcessor coordinates response persistence (ResponseDrain)
// and execution state tracking (ExecutionStateTracker) during flow execution.
type ServerResultProcessor struct {
	drain   *ResponseDrain
	tracker *ExecutionStateTracker
}

var _ ResultProcessor = (*ServerResultProcessor)(nil)

// NewServerResultProcessor creates a processor that persists execution results
// and publishes real-time events for connected clients.
func NewServerResultProcessor(opts ServerResultProcessorOpts) *ServerResultProcessor {
	bufSize := len(opts.Nodes)*2 + 1

	// Build lookup maps from raw flow data
	nodeKindMap := make(map[idwrap.IDWrap]mflow.NodeKind, len(opts.Nodes))
	for _, node := range opts.Nodes {
		nodeKindMap[node.ID] = node.NodeKind
	}
	edgesBySource := make(map[idwrap.IDWrap][]mflow.Edge, len(opts.Edges))
	for _, edge := range opts.Edges {
		edgesBySource[edge.SourceID] = append(edgesBySource[edge.SourceID], edge)
	}
	inverseNodeIDMap := make(map[string]idwrap.IDWrap, len(opts.NodeIDMapping))
	for k, v := range opts.NodeIDMapping {
		inverseNodeIDMap[v.String()] = idwrap.NewTextMust(k)
	}

	drain := newResponseDrain(ResponseDrainOpts{
		WorkspaceID:            opts.WorkspaceID,
		BufSize:                bufSize,
		HTTPResponseService:    opts.HTTPResponseService,
		GraphQLResponseService: opts.GraphQLResponseService,
		Publisher:              opts.Publisher,
		Logger:                 opts.Logger,
	})

	tracker := newExecutionStateTracker(ExecutionStateTrackerOpts{
		FlowID:               opts.FlowID,
		BufSize:              bufSize,
		NodeKindMap:          nodeKindMap,
		EdgesBySource:        edgesBySource,
		InverseNodeIDMap:     inverseNodeIDMap,
		Drain:                drain,
		NodeExecutionService: opts.NodeExecutionService,
		NodeService:          opts.NodeService,
		EdgeService:          opts.EdgeService,
		Publisher:            opts.Publisher,
		Logger:               opts.Logger,
	})

	return &ServerResultProcessor{
		drain:   drain,
		tracker: tracker,
	}
}

func (p *ServerResultProcessor) HTTPResponseChan() chan nrequest.NodeRequestSideResp {
	return p.drain.httpChan
}

func (p *ServerResultProcessor) GraphQLResponseChan() chan ngraphql.NodeGraphQLSideResp {
	return p.drain.gqlChan
}

func (p *ServerResultProcessor) NodeStateChan() chan runner.FlowNodeStatus {
	return p.tracker.stateChan
}

func (p *ServerResultProcessor) Start() {
	// Background context: persistence must outlive flow cancellation
	ctx := context.Background()
	p.drain.start(ctx)
	p.tracker.start(ctx)
}

// Wait blocks until all processing completes.
// Order: state tracker finishes (nodeStateChan closed by runner) →
// close response channels → response drains finish.
func (p *ServerResultProcessor) Wait() {
	p.tracker.wait()
	p.drain.closeAndWait()
}
