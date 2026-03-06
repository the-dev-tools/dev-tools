// Package flowexec manages flow execution sessions.
// It orchestrates the flow lifecycle: variable resolution, node construction,
// runner creation, and result processing.
//
// The ExecutionSession interface supports both local execution (ServerSession)
// and future distributed execution across regions.
package flowexec

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowbuilder"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowresult"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/private/node_js_executor/v1/node_js_executorv1connect"
)

// ExecutionSession manages a single flow execution lifecycle.
// Implementations may run the flow locally (ServerSession) or dispatch
// to remote workers for distributed execution.
type ExecutionSession interface {
	// Prepare builds the execution graph from flow data.
	// Must be called before Run.
	Prepare(ctx context.Context, params ExecutionParams) error

	// Run executes the prepared flow and returns the result.
	// The processor lifecycle (Start/Wait) is managed internally.
	Run(ctx context.Context) (ExecutionResult, error)
}

// ExecutionParams contains the flow data needed for execution.
type ExecutionParams struct {
	Flow     mflow.Flow
	Nodes    []mflow.Node
	Edges    []mflow.Edge // Only valid edges (no orphaned source/target references)
	FlowVars []mflow.FlowVariable
}

// ExecutionResult contains the outcome of a flow execution.
type ExecutionResult struct {
	Duration int32
}

// SessionFactory creates ExecutionSession instances.
// Implementations control where the flow runs: locally (LocalSessionFactory)
// or on remote workers for distributed execution.
type SessionFactory interface {
	Create(processor flowresult.ResultProcessor) ExecutionSession
}

// LocalSessionFactory creates ServerSession instances for local execution.
type LocalSessionFactory struct {
	Builder  *flowbuilder.Builder
	JsClient node_js_executorv1connect.NodeJsExecutorServiceClient
}

var _ SessionFactory = (*LocalSessionFactory)(nil)

func (f *LocalSessionFactory) Create(processor flowresult.ResultProcessor) ExecutionSession {
	return NewServerSession(ServerSessionOpts{
		Builder:   f.Builder,
		JsClient:  f.JsClient,
		Processor: processor,
	})
}

// ServerSessionOpts configures a ServerSession.
type ServerSessionOpts struct {
	Builder   *flowbuilder.Builder
	JsClient  node_js_executorv1connect.NodeJsExecutorServiceClient
	Processor flowresult.ResultProcessor
}

// ServerSession implements ExecutionSession for local server execution.
// It builds the execution graph, runs the flow via FlowLocalRunner,
// and delegates result processing to a ResultProcessor.
type ServerSession struct {
	builder   *flowbuilder.Builder
	jsClient  node_js_executorv1connect.NodeJsExecutorServiceClient
	processor flowresult.ResultProcessor

	// Prepared state (set by Prepare, consumed by Run)
	flowRunner runner.FlowRunner
	baseVars   map[string]any
}

var _ ExecutionSession = (*ServerSession)(nil)

// NewServerSession creates a new ServerSession for local flow execution.
func NewServerSession(opts ServerSessionOpts) *ServerSession {
	return &ServerSession{
		builder:   opts.Builder,
		jsClient:  opts.JsClient,
		processor: opts.Processor,
	}
}

// Prepare builds execution variables, constructs flow nodes, and creates the runner.
func (s *ServerSession) Prepare(ctx context.Context, params ExecutionParams) error {
	baseVars, err := s.builder.BuildVariables(ctx, params.Flow.WorkspaceID, params.FlowVars)
	if err != nil {
		return fmt.Errorf("failed to build execution variables: %w", err)
	}
	s.baseVars = baseVars

	edgeMap := mflow.NewEdgesMap(params.Edges)
	sharedHTTPClient := httpclient.New()

	const defaultNodeTimeout = 60 // seconds
	timeoutDuration := time.Duration(defaultNodeTimeout) * time.Second

	flowNodeMap, startNodeIDs, err := s.builder.BuildNodes(
		ctx,
		params.Flow,
		params.Nodes,
		timeoutDuration,
		sharedHTTPClient,
		s.processor.HTTPResponseChan(),
		s.processor.GraphQLResponseChan(),
		s.jsClient,
	)
	if err != nil {
		return err
	}

	s.flowRunner = flowlocalrunner.CreateFlowRunner(
		idwrap.NewMonotonic(),
		params.Flow.ID,
		startNodeIDs,
		flowNodeMap,
		edgeMap,
		0,
		nil,
	)

	return nil
}

// Run starts the result processor, executes the flow, waits for all result
// processing to complete, and returns the execution duration.
func (s *ServerSession) Run(ctx context.Context) (ExecutionResult, error) {
	s.processor.Start()

	startTime := time.Now()
	runErr := s.flowRunner.RunWithEvents(ctx, runner.FlowEventChannels{
		NodeStates: s.processor.NodeStateChan(),
	}, s.baseVars)

	duration := time.Since(startTime).Milliseconds()
	if duration > math.MaxInt32 {
		duration = math.MaxInt32
	}

	s.processor.Wait()

	//nolint:gosec // duration clamped to MaxInt32
	return ExecutionResult{Duration: int32(duration)}, runErr
}
