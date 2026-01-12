package nai

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/varsystem"
)

type NodeAI struct {
	FlowNodeID   idwrap.IDWrap
	Name         string
	AiModel      mflow.AiModel
	CredentialID idwrap.IDWrap
	Prompt       string
	// ProviderFactory creates LLM clients from credentials
	ProviderFactory *scredential.LLMProviderFactory
	// Override model for testing
	LLM llms.Model
}

func New(id idwrap.IDWrap, name string, aiModel mflow.AiModel, credentialID idwrap.IDWrap, prompt string, factory *scredential.LLMProviderFactory) *NodeAI {
	return &NodeAI{
		FlowNodeID:      id,
		Name:            name,
		AiModel:         aiModel,
		CredentialID:    credentialID,
		Prompt:          prompt,
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

	// 1. Resolve LLM Model
	model := n.LLM
	if model == nil {
		if n.ProviderFactory == nil {
			return node.FlowNodeResult{
				NextNodeID: next,
				Err:        fmt.Errorf("LLM provider factory is missing"),
			}
		}

		var err error
		model, err = n.ProviderFactory.CreateModelWithCredential(ctx, n.AiModel, n.CredentialID)
		if err != nil {
			return node.FlowNodeResult{
				NextNodeID: next,
				Err:        fmt.Errorf("failed to create LLM model: %w", err),
			}
		}
	}

	// 2. Discover and Wrap Tools
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
		toolMap[targetNode.GetName()] = nt
	}

	// 3. Initialize Agent
	// Use LangChain's Structured Config or manual loop. 
	// For simplicity and full control over our custom NodeTool execution, 
	// we'll use a standard LLM completion with tools for now, 
	// or properly integrate with agents.NewToolUser.
	
	// We need to map our handle functions
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

	// 4. Run Agent Loop
	// Note: langchaingo's Agent support is evolving. 
	// We'll use GenerateContent with tool support.
	
	// TODO: Replace placeholders in prompt with variables from req.VarMap
	
	// For now, let's stick to model.Call until we have a more robust agent loop implemented.
	// But let's at least pass the tools to the call.
	
	options := []llms.CallOption{
		llms.WithTools(lcTools),
	}

	// Actually calling the model with tool support requires GenerateContent.
	// completion, err := model.Call(ctx, n.Prompt, options...)
	
	// Let's refine the RunSync to use GenerateContent for proper tool use.
	
	// --- AGENT LOOP START ---
	// Resolve prompt variables
	vm := varsystem.NewVarMapFromAnyMap(req.VarMap)
	resolvedPrompt, err := vm.ReplaceVars(n.Prompt)
	if err != nil {
		// Log warning but continue with raw prompt if resolution fails partially? 
		// Actually ReplaceVars returns error if key not found.
		// For AI nodes, we might want to be more lenient or just use raw prompt as fallback.
		resolvedPrompt = n.Prompt
	}

	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart(resolvedPrompt)},
		},
	}

	var finalResponse string
	for range 5 { // Limit to 5 iterations
		resp, err := model.GenerateContent(ctx, messages, options...)
		if err != nil {
			return node.FlowNodeResult{NextNodeID: next, Err: fmt.Errorf("agent error: %w", err)}
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
				return node.FlowNodeResult{NextNodeID: next, Err: fmt.Errorf("tool execution failed: %w", err)}
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
	// --- AGENT LOOP END ---

	// 5. Store Result
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
