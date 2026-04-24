//nolint:revive // exported
package flowbuilder

import (
	"context"
	"fmt"
	"math"
	"time"

	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/ngraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrequest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nrunsubflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/flowresult"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/httpclient"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/shttp"
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

	// Optional execution tracking: when set, sub-flow runs create flow version
	// history entries and persist node execution records. When nil (e.g. CLI),
	// sub-flows execute without history.
	NodeExecutionService   *sflow.NodeExecutionService
	HTTPResponseService    shttp.HttpResponseService
	GraphQLResponseService sgraphql.GraphQLResponseService
	EventPublisher         flowresult.EventPublisher
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

	httpClient := httpclient.New()
	bufSize := len(nodes) * 10
	if bufSize < 10 {
		bufSize = 10
	}

	// Set up execution tracking if services are available
	hasTracking := e.NodeExecutionService != nil && e.EventPublisher != nil
	var proc flowresult.ResultProcessor
	var version *mflow.Flow

	if hasTracking {
		// Create flow version for history visibility
		v, verr := e.FlowService.CreateFlowVersion(ctx, flow)
		if verr != nil {
			e.Logger.Error("failed to create sub-flow version", "error", verr)
		} else {
			version = &v
		}

		proc = flowresult.NewServerResultProcessor(flowresult.ServerResultProcessorOpts{
			FlowID:                 flow.ID,
			WorkspaceID:            flow.WorkspaceID,
			Nodes:                  nodes,
			Edges:                  edges,
			NodeIDMapping:          nil,
			HTTPResponseService:    e.HTTPResponseService,
			GraphQLResponseService: e.GraphQLResponseService,
			NodeExecutionService:   e.NodeExecutionService,
			NodeService:            e.Builder.Node,
			EdgeService:            e.EdgeService,
			Publisher:              e.EventPublisher,
			Logger:                 e.Logger,
		})
	}

	// Wire response channels: processor owns them when tracking, otherwise drain manually
	var requestRespChan chan nrequest.NodeRequestSideResp
	var gqlRespChan chan ngraphql.NodeGraphQLSideResp

	if proc != nil {
		requestRespChan = proc.HTTPResponseChan()
		gqlRespChan = proc.GraphQLResponseChan()
	} else {
		requestRespChan = make(chan nrequest.NodeRequestSideResp, bufSize)
		gqlRespChan = make(chan ngraphql.NodeGraphQLSideResp, bufSize)
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
	}

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

	var eventChannels runner.FlowEventChannels
	if proc != nil {
		proc.Start()
		eventChannels = runner.FlowEventChannels{
			NodeStates: proc.NodeStateChan(),
		}
	}

	startTime := time.Now()
	runErr := flowRunner.RunWithEvents(ctx, eventChannels, baseVars)
	duration := time.Since(startTime).Milliseconds()

	if proc != nil {
		proc.Wait()
	}

	// Update flow version with execution results
	if version != nil {
		if runErr != nil {
			errMsg := runErr.Error()
			version.Error = &errMsg
		}
		if duration > math.MaxInt32 {
			duration = math.MaxInt32
		}
		//nolint:gosec // duration clamped to MaxInt32
		version.Duration = int32(duration)
		if uerr := e.FlowService.UpdateFlow(ctx, *version); uerr != nil {
			e.Logger.Error("failed to update sub-flow version", "error", uerr)
		}
	}

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
