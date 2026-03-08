//nolint:revive // exported
package flowbuilder

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/ngraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrunsubflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/private/node_js_executor/v1/node_js_executorv1connect"
)

const maxSubFlowDepth = 10

// subFlowCallStackKey is the context key for the sub-flow call stack.
type subFlowCallStackKey struct{}

// subFlowCallStack tracks active flow IDs to detect circular calls.
type subFlowCallStack struct {
	flowIDs []idwrap.IDWrap
}

func getCallStack(ctx context.Context) *subFlowCallStack {
	if cs, ok := ctx.Value(subFlowCallStackKey{}).(*subFlowCallStack); ok {
		return cs
	}
	return &subFlowCallStack{}
}

func withCallStack(ctx context.Context, cs *subFlowCallStack) context.Context {
	return context.WithValue(ctx, subFlowCallStackKey{}, cs)
}

// SubFlowExecutorImpl implements nrunsubflow.SubFlowExecutor using database
// services. It loads the target flow from the DB, builds nodes using the
// Builder, and runs the flow synchronously via FlowLocalRunner.
type SubFlowExecutorImpl struct {
	Builder     *Builder
	FlowService *sflow.FlowService
	EdgeService *sflow.EdgeService
	JSClient    node_js_executorv1connect.NodeJsExecutorServiceClient
	Logger      *slog.Logger
}

var _ nrunsubflow.SubFlowExecutor = (*SubFlowExecutorImpl)(nil)

func NewSubFlowExecutor(
	builder *Builder,
	flowService *sflow.FlowService,
	edgeService *sflow.EdgeService,
	jsClient node_js_executorv1connect.NodeJsExecutorServiceClient,
	logger *slog.Logger,
) *SubFlowExecutorImpl {
	return &SubFlowExecutorImpl{
		Builder:     builder,
		FlowService: flowService,
		EdgeService: edgeService,
		JSClient:    jsClient,
		Logger:      logger,
	}
}

func (e *SubFlowExecutorImpl) ExecuteSubFlow(ctx context.Context, targetFlowID *idwrap.IDWrap, targetFlowName string, inputVars map[string]any) (map[string]any, error) {
	// Check call stack depth
	stack := getCallStack(ctx)
	if len(stack.flowIDs) >= maxSubFlowDepth {
		return nil, fmt.Errorf("sub-flow call depth exceeded (max %d)", maxSubFlowDepth)
	}

	// Resolve target flow
	flow, err := e.resolveFlow(ctx, targetFlowID, targetFlowName)
	if err != nil {
		return nil, fmt.Errorf("resolve sub-flow: %w", err)
	}

	// Check for circular calls
	for _, id := range stack.flowIDs {
		if id == flow.ID {
			return nil, fmt.Errorf("circular sub-flow call detected: flow %q (ID %s)", flow.Name, flow.ID.String())
		}
	}

	// Push current flow onto the call stack
	newStack := &subFlowCallStack{
		flowIDs: make([]idwrap.IDWrap, len(stack.flowIDs)+1),
	}
	copy(newStack.flowIDs, stack.flowIDs)
	newStack.flowIDs[len(stack.flowIDs)] = flow.ID
	ctx = withCallStack(ctx, newStack)

	// Load flow data
	nodes, err := e.Builder.Node.GetNodesByFlowID(ctx, flow.ID)
	if err != nil {
		return nil, fmt.Errorf("get sub-flow nodes: %w", err)
	}

	edges, err := e.EdgeService.GetEdgesByFlowID(ctx, flow.ID)
	if err != nil {
		return nil, fmt.Errorf("get sub-flow edges: %w", err)
	}

	flowVars, err := e.Builder.FlowVariable.GetFlowVariablesByFlowID(ctx, flow.ID)
	if err != nil {
		return nil, fmt.Errorf("get sub-flow variables: %w", err)
	}

	// Build base variables (environment + flow variables)
	baseVars, err := e.Builder.BuildVariables(ctx, flow.WorkspaceID, flowVars)
	if err != nil {
		return nil, fmt.Errorf("build sub-flow variables: %w", err)
	}

	// Inject input variables (override any existing vars with same name)
	for k, v := range inputVars {
		baseVars[k] = v
	}

	// Create HTTP client and response channels for the sub-flow
	httpClient := httpclient.New()
	bufSize := len(nodes) * 10
	if bufSize < 10 {
		bufSize = 10
	}

	requestRespChan := make(chan nrequest.NodeRequestSideResp, bufSize)
	gqlRespChan := make(chan ngraphql.NodeGraphQLSideResp, bufSize)

	// Drain response channels
	go func() {
		for resp := range requestRespChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	go func() {
		for resp := range gqlRespChan {
			if resp.Done != nil {
				close(resp.Done)
			}
		}
	}()
	defer close(requestRespChan)
	defer close(gqlRespChan)

	// Build flow nodes
	edgeMap := mflow.NewEdgesMap(edges)
	timeout := 60 * time.Second

	flowNodeMap, startNodeIDs, err := e.Builder.BuildNodes(
		ctx, flow, nodes, timeout, httpClient, requestRespChan, gqlRespChan, e.JSClient,
	)
	if err != nil {
		return nil, fmt.Errorf("build sub-flow nodes: %w", err)
	}

	// Create and run the flow
	flowRunner := flowlocalrunner.CreateFlowRunner(
		idwrap.NewMonotonic(), flow.ID, startNodeIDs, flowNodeMap, edgeMap, timeout, e.Logger,
	)

	runErr := flowRunner.RunWithEvents(ctx, runner.FlowEventChannels{}, baseVars)
	if runErr != nil {
		return nil, fmt.Errorf("sub-flow execution failed: %w", runErr)
	}

	// Extract outputs from the SubFlowReturn node
	return extractReturnOutputs(nodes, flowNodeMap, baseVars), nil
}

func (e *SubFlowExecutorImpl) resolveFlow(ctx context.Context, targetFlowID *idwrap.IDWrap, targetFlowName string) (mflow.Flow, error) {
	if targetFlowID != nil && *targetFlowID != (idwrap.IDWrap{}) {
		flow, err := e.FlowService.GetFlow(ctx, *targetFlowID)
		if err == nil {
			return flow, nil
		}
		return mflow.Flow{}, fmt.Errorf("sub-flow ID %s not found: %w", targetFlowID.String(), err)
	}
	if targetFlowName == "" {
		return mflow.Flow{}, fmt.Errorf("sub-flow target not specified: neither ID nor name provided")
	}
	// Name-based resolution is not supported without a workspace context.
	// The YAML converter resolves names to IDs at import time.
	return mflow.Flow{}, fmt.Errorf("sub-flow %q not found (name-only resolution requires prior ID assignment)", targetFlowName)
}

// extractReturnOutputs finds the SubFlowReturn node's outputs in the VarMap.
func extractReturnOutputs(nodes []mflow.Node, flowNodeMap map[idwrap.IDWrap]node.FlowNode, baseVars map[string]any) map[string]any {
	for _, n := range nodes {
		if n.NodeKind != mflow.NODE_KIND_SUB_FLOW_RETURN {
			continue
		}
		fn, ok := flowNodeMap[n.ID]
		if !ok {
			continue
		}
		if outputs, ok := baseVars[fn.GetName()]; ok {
			if outputMap, ok := outputs.(map[string]any); ok {
				return outputMap
			}
		}
	}
	return make(map[string]any)
}
