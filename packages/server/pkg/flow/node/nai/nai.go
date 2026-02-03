package nai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/expression"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nmemory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/llm"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
)

// NodeAI is an orchestrator node that manages the AI agent loop.
// It coordinates between the AI Provider (LLM executor), Memory (conversation history),
// and Tools (connected nodes). Like ForEach, it emits iteration status for each LLM call.
type NodeAI struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	Prompt        string
	MaxIterations int32
	// ProviderFactory creates LLM clients from credentials (passed to provider)
	ProviderFactory *scredential.LLMProviderFactory
	// EnableDiscoveryTool enables the discover_tools function (PoC #3)
	EnableDiscoveryTool bool
	// DiscoverToolCalls tracks how many times discover_tools was called (for metrics)
	DiscoverToolCalls int
}

func New(id idwrap.IDWrap, name string, prompt string, maxIterations int32, factory *scredential.LLMProviderFactory) *NodeAI {
	return &NodeAI{
		FlowNodeID:      id,
		Name:            name,
		Prompt:          prompt,
		MaxIterations:   maxIterations,
		ProviderFactory: factory,
	}
}

func (n NodeAI) GetID() idwrap.IDWrap {
	return n.FlowNodeID
}

func (n NodeAI) GetName() string {
	return n.Name
}

func (n NodeAI) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	next := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)

	// 1. REQUIRED: Get connected AI Provider node via HandleAiProvider edge
	providerNodeIDs := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleAiProvider)
	if len(providerNodeIDs) == 0 {
		return node.FlowNodeResult{
			NextNodeID: next,
			Err:        fmt.Errorf("AI Agent requires a connected AI Provider node"),
		}
	}

	providerFlowNode, ok := req.NodeMap[providerNodeIDs[0]]
	if !ok {
		return node.FlowNodeResult{
			NextNodeID: next,
			Err:        fmt.Errorf("AI Provider node not found in node map"),
		}
	}

	// Check if it implements AIProvider interface
	providerNode, ok := providerFlowNode.(AIProvider)
	if !ok {
		return node.FlowNodeResult{
			NextNodeID: next,
			Err:        fmt.Errorf("connected node does not implement AIProvider interface"),
		}
	}

	// Pass provider factory to provider node if available
	if n.ProviderFactory != nil {
		providerNode.SetProviderFactory(n.ProviderFactory)
	}

	// 2. OPTIONAL: Get connected Memory node via HandleAiMemory edge
	var memoryNode *nmemory.NodeMemory
	memoryNodeIDs := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleAiMemory)
	if len(memoryNodeIDs) > 0 {
		if mn, ok := req.NodeMap[memoryNodeIDs[0]].(*nmemory.NodeMemory); ok {
			memoryNode = mn
		}
	}

	// 3. Discover and Wrap Tools
	tools := []llm.Tool{
		getVariableTool(req),
		setVariableTool(req),
	}

	// Internal map for easy lookup during execution
	toolMap := make(map[string]*NodeTool)

	// Add connected nodes as tools
	connectedNodeIDs := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleAiTools)
	for _, targetID := range connectedNodeIDs {
		targetNode, ok := req.NodeMap[targetID]
		if !ok {
			continue
		}
		nt := NewNodeTool(targetNode, req)
		tools = append(tools, nt.AsTool())
		toolMap[sanitizeToolName(targetNode.GetName())] = nt
	}

	// PoC #3: Add discover_tools if enabled
	if n.EnableDiscoveryTool {
		tools = append(tools, discoverToolsTool())
	}

	// 4. Resolve prompt variables (supports expressions and AI marker functions)
	env := expression.NewUnifiedEnv(req.VarMap)
	resolvedPrompt, err := env.Interpolate(n.Prompt)
	if err != nil {
		// Use raw prompt as fallback if variable resolution fails
		resolvedPrompt = n.Prompt
	}

	// 5. Build initial messages from memory context
	messages := []llm.Message{}
	if memoryNode != nil {
		for _, msg := range memoryNode.GetMessages() {
			var role llm.MessageRole
			switch msg.Role {
			case "user":
				role = llm.RoleUser
			case "assistant":
				role = llm.RoleAssistant
			case "system":
				role = llm.RoleSystem
			default:
				role = llm.RoleUser
			}
			messages = append(messages, llm.Message{
				Role:  role,
				Parts: []llm.ContentPart{llm.TextPart(msg.Content)},
			})
		}
	}

	// Add current prompt as user message
	messages = append(messages, llm.Message{
		Role:  llm.RoleUser,
		Parts: []llm.ContentPart{llm.TextPart(resolvedPrompt)},
	})

	// 6. Create tool executor function
	executor := n.createToolExecutor(ctx, req, toolMap)

	// 7. Run Agent Loop with iteration tracking (like ForEach)
	var finalResponse string
	maxIters := int(n.MaxIterations)
	if maxIters <= 0 {
		maxIters = 5
	}

	// Metrics aggregation - use interface methods to get model/provider info
	totalMetrics := mflow.AITotalMetrics{
		Model:    providerNode.GetModelString(),
		Provider: providerNode.GetProviderString(),
	}

	for i := range maxIters {
		// Generate unique execution ID for this iteration
		executionID := idwrap.NewMonotonic()

		// Build iteration context (like ForEach does)
		iterContext := n.buildIterationContext(req, i)

		// Emit RUNNING status for this iteration
		if req.LogPushFunc != nil {
			executionName := fmt.Sprintf("%s LLM Call %d", n.Name, i+1)
			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID:      executionID,
				NodeID:           n.FlowNodeID,
				Name:             executionName,
				State:            mflow.NODE_STATE_RUNNING,
				OutputData:       map[string]any{"iteration": i + 1},
				IterationEvent:   true,
				IterationIndex:   i,
				LoopNodeID:       n.FlowNodeID,
				IterationContext: iterContext,
			})
		}

		// Execute the provider node with typed input
		// Provider has its own isolated tracking (emits its own status events)
		// Disable orchestrator's tracker so provider output doesn't leak into orchestrator's output
		savedTracker := req.VariableTracker
		req.VariableTracker = nil
		providerOutput, err := providerNode.Execute(ctx, req, AIProviderInput{
			Messages: messages,
			Tools:    tools,
		})
		req.VariableTracker = savedTracker

		if err != nil {
			// Emit FAILURE status for this iteration
			if req.LogPushFunc != nil {
				executionName := fmt.Sprintf("%s LLM Call %d", n.Name, i+1)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID,
					NodeID:           n.FlowNodeID,
					Name:             executionName,
					State:            mflow.NODE_STATE_FAILURE,
					Error:            err,
					IterationEvent:   true,
					IterationIndex:   i,
					LoopNodeID:       n.FlowNodeID,
					IterationContext: iterContext,
				})
			}
			return node.FlowNodeResult{NextNodeID: next, Err: fmt.Errorf("agent error: %w", err)}
		}

		// Aggregate metrics (orchestrator tracks totals, provider tracks per-call metrics in its own events)
		totalMetrics.PromptTokens += providerOutput.Metrics.PromptTokens
		totalMetrics.CompletionTokens += providerOutput.Metrics.CompletionTokens
		totalMetrics.TotalTokens += providerOutput.Metrics.TotalTokens
		totalMetrics.LLMCalls++

		// Check if we should stop (no tool calls to execute)
		if len(providerOutput.ToolCalls) == 0 {
			finalResponse = providerOutput.Text

			// Emit SUCCESS status for final response
			if req.LogPushFunc != nil {
				executionName := fmt.Sprintf("%s LLM Call %d", n.Name, i+1)
				req.LogPushFunc(runner.FlowNodeStatus{
					ExecutionID:      executionID,
					NodeID:           n.FlowNodeID,
					Name:             executionName,
					State:            mflow.NODE_STATE_SUCCESS,
					OutputData:       map[string]any{"text": finalResponse, "iteration": i + 1, "is_final": true},
					IterationEvent:   true,
					IterationIndex:   i,
					LoopNodeID:       n.FlowNodeID,
					IterationContext: iterContext,
				})
			}
			break
		}

		// Add assistant's response with tool calls to message history
		assistantMsg := llm.Message{
			Role: llm.RoleAssistant,
		}
		for _, tc := range providerOutput.ToolCalls {
			toolType := tc.Type
			if toolType == "" {
				toolType = "function" // Default to "function" if not specified
			}
			assistantMsg.Parts = append(assistantMsg.Parts, llm.ToolCall{
				ID:           tc.ID,
				Type:         toolType,
				FunctionName: tc.Name,
				Arguments:    tc.Arguments,
			})
		}
		messages = append(messages, assistantMsg)

		// Execute tool calls and collect all results into a single message
		// Anthropic requires all tool_results from one assistant turn to be in a single user message
		toolResultMsg := llm.Message{
			Role:  llm.RoleTool,
			Parts: []llm.ContentPart{},
		}
		for _, tc := range providerOutput.ToolCalls {
			totalMetrics.ToolCalls++

			result, execErr := executor(ctx, tc.Name, tc.Arguments)
			if execErr != nil {
				// Feed the error back to the LLM instead of failing the node
				result = fmt.Sprintf("Error: %v", execErr)
			}

			// Add tool response to the combined message
			toolResultMsg.Parts = append(toolResultMsg.Parts, llm.ToolCallResponse{
				ToolCallID: tc.ID,
				Name:       tc.Name,
				Content:    result,
			})
		}
		// Add the combined tool result message to history
		if len(toolResultMsg.Parts) > 0 {
			messages = append(messages, toolResultMsg)
		}

		// Emit SUCCESS status for this iteration with detailed info
		if req.LogPushFunc != nil {
			executionName := fmt.Sprintf("%s LLM Call %d", n.Name, i+1)

			// Collect tool call names for better observability
			toolCallNames := make([]string, 0, len(providerOutput.ToolCalls))
			for _, tc := range providerOutput.ToolCalls {
				toolCallNames = append(toolCallNames, tc.Name)
			}

			iterOutput := map[string]any{
				"iteration":  i + 1,
				"tool_calls": toolCallNames,
			}
			// Include text if the LLM produced any alongside tool calls
			if providerOutput.Text != "" {
				iterOutput["text"] = providerOutput.Text
			}

			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID:      executionID,
				NodeID:           n.FlowNodeID,
				Name:             executionName,
				State:            mflow.NODE_STATE_SUCCESS,
				OutputData:       iterOutput,
				IterationEvent:   true,
				IterationIndex:   i,
				LoopNodeID:       n.FlowNodeID,
				IterationContext: iterContext,
			})
		}

		// If this is the last iteration and we still have tool calls,
		// use whatever text we got (may be empty)
		if i == maxIters-1 {
			finalResponse = providerOutput.Text
		}
	}

	// 8. Update memory with the conversation if Memory node is connected
	if memoryNode != nil {
		memoryNode.AddMessage("user", resolvedPrompt)
		memoryNode.AddMessage("assistant", finalResponse)
	}

	// 9. Store final result - replace entirely (don't merge with set_variable artifacts)
	resultMap := map[string]any{
		"text":          finalResponse,
		"total_metrics": totalMetrics,
	}

	// Store final result
	req.ReadWriteLock.Lock()
	req.VarMap[n.Name] = resultMap
	req.ReadWriteLock.Unlock()

	// Track the final output (set_variable writes are already tracked)
	if req.VariableTracker != nil {
		req.VariableTracker.TrackWrite(n.Name+".text", finalResponse)
		req.VariableTracker.TrackWrite(n.Name+".total_metrics", totalMetrics)
	}

	return node.FlowNodeResult{
		NextNodeID: next,
		Err:        nil,
	}
}

// buildIterationContext creates the iteration context for a given iteration (like ForEach)
func (n *NodeAI) buildIterationContext(req *node.FlowNodeRequest, iterationIndex int) *runner.IterationContext {
	var parentPath []int
	var parentNodes []idwrap.IDWrap
	var parentLabels []runner.IterationLabel

	if req.IterationContext != nil {
		parentPath = req.IterationContext.IterationPath
		parentNodes = req.IterationContext.ParentNodes
		parentLabels = node.CloneIterationLabels(req.IterationContext.Labels)
	}

	labels := make([]runner.IterationLabel, len(parentLabels), len(parentLabels)+1)
	copy(labels, parentLabels)
	labels = append(labels, runner.IterationLabel{
		NodeID:    n.FlowNodeID,
		Name:      n.Name,
		Iteration: iterationIndex + 1,
	})

	return &runner.IterationContext{
		IterationPath:  append(parentPath, iterationIndex),
		ExecutionIndex: iterationIndex,
		ParentNodes:    append(parentNodes, n.FlowNodeID),
		Labels:         labels,
	}
}

// createToolExecutor creates the executor function for tool calls
func (n *NodeAI) createToolExecutor(ctx context.Context, req *node.FlowNodeRequest, toolMap map[string]*NodeTool) func(context.Context, string, string) (string, error) {
	return func(ctx context.Context, name string, args string) (string, error) {
		switch name {
		case "get_variable":
			return handleGetVariable(ctx, req, args)
		case "set_variable":
			return handleSetVariable(ctx, req, args)
		case "discover_tools":
			n.DiscoverToolCalls++
			return handleDiscoverTools(ctx, toolMap, args)
		default:
			tool, ok := toolMap[name]
			if !ok {
				return "", fmt.Errorf("tool '%s' not found", name)
			}

			return n.executeToolNode(ctx, req, tool)
		}
	}
}

// executeToolNode executes a connected tool node
func (n *NodeAI) executeToolNode(ctx context.Context, req *node.FlowNodeRequest, tool *NodeTool) (string, error) {
	toolNodeID := tool.TargetNode.GetID()
	toolNodeName := tool.TargetNode.GetName()

	// Build the tool chain: find all nodes reachable from the tool node
	chainNodeIDs := findDownstreamNodes(req.EdgeSourceMap, toolNodeID, n.FlowNodeID)

	// Build edge map for the chain (only edges between chain nodes)
	chainEdgeMap := buildChainEdgeMap(req.EdgeSourceMap, chainNodeIDs)

	// Build node map for the chain
	chainNodeMap := make(map[idwrap.IDWrap]node.FlowNode, len(chainNodeIDs))
	for nodeID := range chainNodeIDs {
		if n, ok := req.NodeMap[nodeID]; ok {
			chainNodeMap[nodeID] = n
		}
	}

	// Build predecessor map for proper scheduling
	predecessorMap := flowlocalrunner.BuildPredecessorMap(chainEdgeMap)
	pendingMap := node.BuildPendingMap(predecessorMap)

	// Create child request for the tool chain execution
	childReq := *req
	childReq.EdgeSourceMap = chainEdgeMap
	childReq.NodeMap = chainNodeMap
	childReq.PendingAtmoicMap = pendingMap
	childReq.ExecutionID = idwrap.NewMonotonic()
	childReq.ReadWriteLock = &sync.RWMutex{}

	// Provide a no-op status func if LogPushFunc is nil (e.g., in tests)
	statusFunc := req.LogPushFunc
	if statusFunc == nil {
		statusFunc = func(s runner.FlowNodeStatus) {}
	}

	// Execute via runner
	err := flowlocalrunner.RunNodeSync(ctx, toolNodeID, &childReq, statusFunc, predecessorMap)

	// Extract outputs from all nodes in the chain
	var outputs []string
	for nodeID := range chainNodeIDs {
		nodeName := ""
		if n, ok := chainNodeMap[nodeID]; ok {
			nodeName = n.GetName()
		}
		if nodeName == "" {
			continue
		}

		if data, ok := childReq.VarMap[nodeName]; ok {
			// Copy node output to parent VarMap
			req.ReadWriteLock.Lock()
			req.VarMap[nodeName] = data
			req.ReadWriteLock.Unlock()

			// Add to output summary
			if jsonBytes, jsonErr := json.Marshal(data); jsonErr == nil {
				outputs = append(outputs, fmt.Sprintf("%s: %s", nodeName, string(jsonBytes)))
			}
		}
	}

	var output string
	if len(outputs) > 0 {
		output = fmt.Sprintf("Chain executed successfully. Results:\n%s", strings.Join(outputs, "\n"))
	} else {
		output = fmt.Sprintf("Node '%s' executed successfully. No output captured.", toolNodeName)
	}

	return output, err
}

func (n NodeAI) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	result := n.RunSync(ctx, req)
	resultChan <- result
}

// findDownstreamNodes finds all nodes reachable from startNodeID via edges.
// It excludes the excludeNodeID (typically the AI node itself) to prevent cycles.
func findDownstreamNodes(edgeMap mflow.EdgesMap, startNodeID, excludeNodeID idwrap.IDWrap) map[idwrap.IDWrap]struct{} {
	visited := make(map[idwrap.IDWrap]struct{})
	queue := []idwrap.IDWrap{startNodeID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if _, seen := visited[current]; seen {
			continue
		}
		if current == excludeNodeID {
			continue // Don't traverse back to the AI node
		}

		visited[current] = struct{}{}

		// Find all nodes connected from current node
		if handles, ok := edgeMap[current]; ok {
			for _, targets := range handles {
				for _, targetID := range targets {
					if _, seen := visited[targetID]; !seen && targetID != excludeNodeID {
						queue = append(queue, targetID)
					}
				}
			}
		}
	}

	return visited
}

// GetRequiredVariables implements node.VariableIntrospector.
// It extracts all variable references from the Prompt field.
func (n *NodeAI) GetRequiredVariables() []string {
	return expression.ExtractVarRefs(n.Prompt)
}

// GetOutputVariables implements node.VariableIntrospector.
// Returns the output paths this AI node produces.
// Note: During iterations, "iteration" is written for observability.
// After completion, "text" and "total_metrics" contain the final result.
func (n *NodeAI) GetOutputVariables() []string {
	return []string{
		"text",
		"total_metrics",
		"iteration", // Available during iterations
	}
}

// buildChainEdgeMap creates an edge map containing only edges between the given nodes.
func buildChainEdgeMap(fullEdgeMap mflow.EdgesMap, chainNodes map[idwrap.IDWrap]struct{}) mflow.EdgesMap {
	chainEdgeMap := make(mflow.EdgesMap)

	for sourceID, handles := range fullEdgeMap {
		// Only include edges from nodes in the chain
		if _, inChain := chainNodes[sourceID]; !inChain {
			continue
		}

		filteredHandles := make(map[mflow.EdgeHandle][]idwrap.IDWrap)
		for handle, targets := range handles {
			var filteredTargets []idwrap.IDWrap
			for _, targetID := range targets {
				// Only include targets that are in the chain
				if _, inChain := chainNodes[targetID]; inChain {
					filteredTargets = append(filteredTargets, targetID)
				}
			}
			if len(filteredTargets) > 0 {
				filteredHandles[handle] = filteredTargets
			}
		}

		if len(filteredHandles) > 0 {
			chainEdgeMap[sourceID] = filteredHandles
		}
	}

	return chainEdgeMap
}
