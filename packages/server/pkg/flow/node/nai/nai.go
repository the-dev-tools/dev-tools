package nai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/naiprovider"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nmemory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner/flowlocalrunner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/varsystem"
)

type NodeAI struct {
	FlowNodeID    idwrap.IDWrap
	Name          string
	Prompt        string
	MaxIterations int32
	// ProviderFactory creates LLM clients from credentials
	ProviderFactory *scredential.LLMProviderFactory
	// Override model for testing
	LLM llms.Model
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

	providerNode, ok := req.NodeMap[providerNodeIDs[0]].(*naiprovider.NodeAiProvider)
	if !ok {
		return node.FlowNodeResult{
			NextNodeID: next,
			Err:        fmt.Errorf("connected node is not an AI Provider node"),
		}
	}

	// Use model configuration from connected AI Provider node
	aiModel := providerNode.Model
	customModel := providerNode.CustomModel
	credentialID := providerNode.CredentialID // Can be nil if not set
	temperature := providerNode.Temperature
	maxTokens := providerNode.MaxTokens

	// 2. OPTIONAL: Get connected Memory node via HandleAiMemory edge
	var memoryNode *nmemory.NodeMemory
	memoryNodeIDs := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleAiMemory)
	if len(memoryNodeIDs) > 0 {
		if mn, ok := req.NodeMap[memoryNodeIDs[0]].(*nmemory.NodeMemory); ok {
			memoryNode = mn
		}
	}

	// 3. Resolve LLM Model
	model := n.LLM
	if model == nil {
		if n.ProviderFactory == nil {
			return node.FlowNodeResult{
				NextNodeID: next,
				Err:        fmt.Errorf("AI Agent node requires LLM provider factory - ensure an AI Provider node is connected and credentials are configured"),
			}
		}

		if credentialID == nil {
			return node.FlowNodeResult{
				NextNodeID: next,
				Err:        fmt.Errorf("AI Provider node has no credential configured - please set a credential on the connected AI Provider node"),
			}
		}

		var err error
		model, err = n.ProviderFactory.CreateModelWithCredential(ctx, aiModel, customModel, *credentialID)
		if err != nil {
			return node.FlowNodeResult{
				NextNodeID: next,
				Err:        fmt.Errorf("failed to create LLM model: %w", err),
			}
		}
	}

	// 4. Discover and Wrap Tools
	lcTools := []llms.Tool{
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
		lcTools = append(lcTools, nt.AsLangChainTool())
		toolMap[sanitizeToolName(targetNode.GetName())] = nt
	}

	// PoC #3: Add discover_tools if enabled
	if n.EnableDiscoveryTool {
		lcTools = append(lcTools, discoverToolsTool())
	}

	// 5. Initialize Agent executor using the flow runner (like ForEach does)
	// This ensures proper status emission, AuxiliaryID propagation, and HTTP response linking
	executor := func(ctx context.Context, name string, args string) (string, error) {
		switch name {
		case "get_variable":
			return handleGetVariable(ctx, req, args)
		case "set_variable":
			return handleSetVariable(ctx, req, args)
		case "discover_tools":
			// PoC #3: Handle discover_tools call
			n.DiscoverToolCalls++
			return handleDiscoverTools(ctx, toolMap, args)
		default:
			tool, ok := toolMap[name]
			if !ok {
				return "", fmt.Errorf("tool '%s' not found", name)
			}

			toolNodeID := tool.TargetNode.GetID()
			toolNodeName := tool.TargetNode.GetName()

			// Build the tool chain: find all nodes reachable from the tool node
			// This allows chained request nodes to execute as a single tool
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
			// Create a new RWMutex for the child request to avoid lock contention
			childReq.ReadWriteLock = &sync.RWMutex{}

			// Provide a no-op status func if LogPushFunc is nil (e.g., in tests)
			statusFunc := req.LogPushFunc
			if statusFunc == nil {
				statusFunc = func(s runner.FlowNodeStatus) {} // no-op
			}

			// Execute via runner - this handles status emission automatically including AuxiliaryID
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
	}

	// Build LLM options with temperature/maxTokens from AI Provider node
	options := []llms.CallOption{
		llms.WithTools(lcTools),
	}
	if temperature != nil {
		options = append(options, llms.WithTemperature(float64(*temperature)))
	}
	if maxTokens != nil {
		options = append(options, llms.WithMaxTokens(int(*maxTokens)))
	}

	// 6. Resolve prompt variables
	vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
	resolvedPrompt, err := vm.ReplaceVars(n.Prompt)
	if err != nil {
		// Use raw prompt as fallback if variable resolution fails
		resolvedPrompt = n.Prompt
	}

	// 7. Build messages with memory context
	messages := []llms.MessageContent{}

	// Add conversation history from Memory node if connected
	if memoryNode != nil {
		for _, msg := range memoryNode.GetMessages() {
			var role llms.ChatMessageType
			switch msg.Role {
			case "user":
				role = llms.ChatMessageTypeHuman
			case "assistant":
				role = llms.ChatMessageTypeAI
			case "system":
				role = llms.ChatMessageTypeSystem
			default:
				role = llms.ChatMessageTypeHuman
			}
			messages = append(messages, llms.MessageContent{
				Role:  role,
				Parts: []llms.ContentPart{llms.TextPart(msg.Content)},
			})
		}
	}

	// Add current prompt
	messages = append(messages, llms.MessageContent{
		Role:  llms.ChatMessageTypeHuman,
		Parts: []llms.ContentPart{llms.TextPart(resolvedPrompt)},
	})

	// 8. Run Agent Loop
	var finalResponse string
	maxIters := int(n.MaxIterations)
	if maxIters <= 0 {
		maxIters = 5
	}
	for range maxIters {
		resp, err := model.GenerateContent(ctx, messages, options...)
		if err != nil {
			return node.FlowNodeResult{NextNodeID: next, Err: fmt.Errorf("agent error: %w", err)}
		}

		if len(resp.Choices) == 0 {
			return node.FlowNodeResult{NextNodeID: next, Err: fmt.Errorf("LLM returned empty response (no choices)")}
		}

		// Collect text content and tool calls from ALL choices
		// Anthropic returns multiple choices: one with text, another with tool calls
		var textContent string
		var allToolCalls []llms.ToolCall

		for _, choice := range resp.Choices {
			if choice.Content != "" {
				textContent = choice.Content
			}
			allToolCalls = append(allToolCalls, choice.ToolCalls...)
		}

		// Check if we should stop (no tool calls to execute)
		// Stop reasons: "stop" (OpenAI), "end_turn" (Anthropic) = final response
		// Continue reasons: "tool_calls" (OpenAI), "tool_use" (Anthropic) = need to execute tools
		if len(allToolCalls) == 0 {
			finalResponse = textContent
			break
		}

		// Add assistant's response to history
		// HACK: langchaingo's Anthropic handler only reads Parts[0], so we must
		// put ToolCalls first and skip text content when there are tool calls.
		// https://github.com/tmc/langchaingo/issues/1468
		assistantMsg := llms.MessageContent{
			Role: llms.ChatMessageTypeAI,
		}
		for _, tc := range allToolCalls {
			assistantMsg.Parts = append(assistantMsg.Parts, tc)
		}
		messages = append(messages, assistantMsg)

		// Execute tools
		for _, tc := range allToolCalls {
			result, err := executor(ctx, tc.FunctionCall.Name, tc.FunctionCall.Arguments)
			if err != nil {
				// Feed the error back to the LLM instead of failing the node
				result = fmt.Sprintf("Error: %v", err)
			}

			// Add tool response to history
			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: tc.ID,
						Name:       tc.FunctionCall.Name,
						Content:    result,
					},
				},
			})
		}
	}

	// 9. Update memory with the conversation if Memory node is connected
	if memoryNode != nil {
		memoryNode.AddMessage("user", resolvedPrompt)
		memoryNode.AddMessage("assistant", finalResponse)
	}

	// 10. Store Result
	resultMap := map[string]interface{}{
		"text": finalResponse,
	}

	if req.VariableTracker != nil {
		if err := node.WriteNodeVarBulkWithTracking(req, n.Name, resultMap, req.VariableTracker); err != nil {
			return node.FlowNodeResult{
				NextNodeID: next,
				Err:        fmt.Errorf("failed to write AI result with tracking: %w", err),
			}
		}
	} else {
		if err := node.WriteNodeVarBulk(req, n.Name, resultMap); err != nil {
			return node.FlowNodeResult{
				NextNodeID: next,
				Err:        fmt.Errorf("failed to write AI result: %w", err),
			}
		}
	}

	return node.FlowNodeResult{
		NextNodeID: next,
		Err:        nil,
	}
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
