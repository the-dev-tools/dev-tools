// Package naiprovider provides the AI Provider node implementation for flow execution.
// AI Provider nodes are active LLM executors that make LLM calls and track metrics.
// They are orchestrated by NodeAI nodes via HandleAiProvider edges.
package naiprovider

import (
	"context"
	"fmt"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/llm"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
)

// NodeAiProvider represents an AI Provider node that executes LLM calls.
// It is an active node that makes LLM API calls and returns results with metrics.
// The orchestrator (NodeAI) calls Execute() with typed input and receives typed output.
type NodeAiProvider struct {
	FlowNodeID   idwrap.IDWrap
	Name         string
	CredentialID *idwrap.IDWrap // Optional: nil means no credential set yet
	Model        mflow.AiModel
	Temperature  *float32 // Optional: nil means use provider default
	MaxTokens    *int32   // Optional: nil means use provider default

	// Runtime dependencies
	ProviderFactory *scredential.LLMProviderFactory
	// LLM allows injecting a mock model for testing
	LLM llms.Model
}

// New creates a new NodeAiProvider with the given configuration.
func New(
	id idwrap.IDWrap,
	name string,
	credentialID *idwrap.IDWrap,
	model mflow.AiModel,
	temperature *float32,
	maxTokens *int32,
) *NodeAiProvider {
	return &NodeAiProvider{
		FlowNodeID:   id,
		Name:         name,
		CredentialID: credentialID,
		Model:        model,
		Temperature:  temperature,
		MaxTokens:    maxTokens,
	}
}

// GetID returns the node's unique identifier.
func (n *NodeAiProvider) GetID() idwrap.IDWrap { return n.FlowNodeID }

// GetName returns the node's display name.
func (n *NodeAiProvider) GetName() string { return n.Name }

// Execute runs the LLM with typed input and returns typed output.
// This is the primary method for orchestrator-to-provider communication,
// maintaining type safety for messages and tool calls.
func (n *NodeAiProvider) Execute(ctx context.Context, req *node.FlowNodeRequest, input nai.AIProviderInput) (*mflow.AIProviderOutput, error) {
	startTime := time.Now()
	executionID := idwrap.NewNow()

	// Helper to emit failure status
	emitFailure := func(err error) {
		if req.LogPushFunc != nil {
			req.LogPushFunc(runner.FlowNodeStatus{
				ExecutionID: executionID,
				NodeID:      n.FlowNodeID,
				Name:        n.Name,
				State:       mflow.NODE_STATE_FAILURE,
				Error:       err,
				RunDuration: time.Since(startTime),
			})
		}
	}

	// Emit RUNNING status
	if req.LogPushFunc != nil {
		req.LogPushFunc(runner.FlowNodeStatus{
			ExecutionID: executionID,
			NodeID:      n.FlowNodeID,
			Name:        n.Name,
			State:       mflow.NODE_STATE_RUNNING,
		})
	}

	// 1. Get or create LLM model
	model := n.LLM
	if model == nil {
		if n.ProviderFactory == nil {
			err := fmt.Errorf("AI Provider node requires LLM provider factory - ensure credentials are configured")
			emitFailure(err)
			return nil, err
		}

		if n.CredentialID == nil {
			err := fmt.Errorf("AI Provider node has no credential configured")
			emitFailure(err)
			return nil, err
		}

		var err error
		model, err = n.ProviderFactory.CreateModelWithCredential(ctx, n.Model, "", *n.CredentialID)
		if err != nil {
			err = fmt.Errorf("failed to create LLM model: %w", err)
			emitFailure(err)
			return nil, err
		}
	}

	// 2. CONVERSION BOUNDARY: Convert from our types to langchaingo types
	lcMessages := llm.ToLangChainMessages(input.Messages)

	// 3. Build LLM call options
	options := []llms.CallOption{}
	if len(input.Tools) > 0 {
		options = append(options, llms.WithTools(llm.ToLangChainTools(input.Tools)))
	}
	if n.Temperature != nil {
		options = append(options, llms.WithTemperature(float64(*n.Temperature)))
	}
	if n.MaxTokens != nil {
		options = append(options, llms.WithMaxTokens(int(*n.MaxTokens)))
	}

	// 4. Make LLM call
	resp, err := model.GenerateContent(ctx, lcMessages, options...)
	if err != nil {
		err = fmt.Errorf("LLM call failed: %w", err)
		emitFailure(err)
		return nil, err
	}

	if resp == nil || len(resp.Choices) == 0 {
		err := fmt.Errorf("LLM returned empty response (no choices)")
		emitFailure(err)
		return nil, err
	}

	// 5. Extract response data using helpers
	textContent := ExtractTextContent(resp)
	toolCalls := ExtractToolCalls(resp)
	promptTokens, completionTokens := ExtractTokensFromResponse(resp)
	finishReason := ExtractFinishReason(resp)

	// 6. Determine model string for metrics
	modelStr := n.Model.ModelString()

	// 7. Build output structure
	output := &mflow.AIProviderOutput{
		Text: textContent,
		Metrics: mflow.AIMetrics{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
			Model:            modelStr,
			Provider:         n.Model.Provider(),
			FinishReason:     finishReason,
		},
	}

	// Convert tool calls to our format
	for _, tc := range toolCalls {
		toolType := tc.Type
		if toolType == "" {
			toolType = "function" // Default to "function" if not specified
		}
		output.ToolCalls = append(output.ToolCalls, mflow.AIToolCall{
			ID:        tc.ID,
			Type:      toolType,
			Name:      tc.FunctionCall.Name,
			Arguments: tc.FunctionCall.Arguments,
		})
	}

	// 8. Write output to VarMap for observability
	outputMap := map[string]any{
		"text":       output.Text,
		"tool_calls": output.ToolCalls,
		"metrics":    output.Metrics,
	}

	if req.VariableTracker != nil {
		if err := node.WriteNodeVarBulkWithTracking(req, n.Name, outputMap, req.VariableTracker); err != nil {
			err = fmt.Errorf("failed to write AI Provider result: %w", err)
			emitFailure(err)
			return nil, err
		}
	} else {
		if err := node.WriteNodeVarBulk(req, n.Name, outputMap); err != nil {
			err = fmt.Errorf("failed to write AI Provider result: %w", err)
			emitFailure(err)
			return nil, err
		}
	}

	// 9. Emit SUCCESS status with output data
	if req.LogPushFunc != nil {
		req.LogPushFunc(runner.FlowNodeStatus{
			ExecutionID: executionID,
			NodeID:      n.FlowNodeID,
			Name:        n.Name,
			State:       mflow.NODE_STATE_SUCCESS,
			OutputData:  outputMap,
			RunDuration: time.Since(startTime),
		})
	}

	return output, nil
}

// RunSync is part of the FlowNode interface but should not be used for AI Provider nodes.
// The orchestrator (NodeAI) should call Execute() directly with typed input.
// This method exists only for interface compliance and will error if called.
func (n *NodeAiProvider) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	next := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)

	// AI Provider should be called via Execute from the orchestrator, not via RunSync.
	// If RunSync is called, it means the provider is being used incorrectly.
	return node.FlowNodeResult{
		NextNodeID: next,
		Err:        fmt.Errorf("NodeAiProvider should be called via Execute from AI orchestrator, not RunSync"),
	}
}

// RunAsync runs the node asynchronously by calling RunSync and sending the result.
func (n *NodeAiProvider) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

// GetRequiredVariables implements node.VariableIntrospector.
// Returns variables referenced in node configuration (none for AI Provider).
func (n *NodeAiProvider) GetRequiredVariables() []string {
	return nil
}

// GetOutputVariables implements node.VariableIntrospector.
// Returns the output paths this AI Provider node produces.
func (n *NodeAiProvider) GetOutputVariables() []string {
	return []string{
		"text",
		"tool_calls",
		"metrics",
	}
}

// GetModelString returns the model identifier string (e.g., "gpt-5.2").
// Implements the AIProvider interface from nai package.
func (n *NodeAiProvider) GetModelString() string {
	return n.Model.ModelString()
}

// GetProviderString returns the provider name (e.g., "openai", "anthropic").
// Implements the AIProvider interface from nai package.
func (n *NodeAiProvider) GetProviderString() string {
	return n.Model.Provider()
}

// SetProviderFactory sets the LLM provider factory on this node.
// Implements the AIProvider interface.
func (n *NodeAiProvider) SetProviderFactory(factory *scredential.LLMProviderFactory) {
	n.ProviderFactory = factory
}

// SetLLM sets a mock LLM model for testing purposes.
// Implements the AIProvider interface.
func (n *NodeAiProvider) SetLLM(llm any) {
	if m, ok := llm.(llms.Model); ok {
		n.LLM = m
	}
}
