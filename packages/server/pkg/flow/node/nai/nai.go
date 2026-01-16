package nai

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nmemory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/naiprovider"
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
	credentialID := providerNode.CredentialID
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

		var err error
		model, err = n.ProviderFactory.CreateModelWithCredential(ctx, aiModel, customModel, credentialID)
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
	toolMap := make(map[string]interface {
		Execute(context.Context, string) (string, error)
	})

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

	// 5. Initialize Agent executor
	executor := func(ctx context.Context, name string, args string) (string, error) {
		switch name {
		case "get_variable":
			return handleGetVariable(ctx, req, args)
		case "set_variable":
			return handleSetVariable(ctx, req, args)
		default:
			if tool, ok := toolMap[name]; ok {
				return tool.Execute(ctx, args)
			}
			return "", fmt.Errorf("tool '%s' not found", name)
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

		choice := resp.Choices[0]
		if choice.StopReason == "stop" || len(choice.ToolCalls) == 0 {
			finalResponse = choice.Content
			break
		}

		// Add assistant's tool call to history
		assistantMsg := llms.MessageContent{
			Role: llms.ChatMessageTypeAI,
		}
		for _, tc := range choice.ToolCalls {
			assistantMsg.Parts = append(assistantMsg.Parts, tc)
		}
		messages = append(messages, assistantMsg)

		// Execute tools
		for _, tc := range choice.ToolCalls {
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
