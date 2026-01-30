package naiprovider

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/llm"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// mockModel implements llms.Model for testing
type mockModel struct {
	mock.Mock
}

func (m *mockModel) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	args := m.Called(ctx, prompt, options)
	return args.String(0), args.Error(1)
}

func (m *mockModel) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	args := m.Called(ctx, messages, options)
	resp := args.Get(0)
	if resp == nil {
		return nil, args.Error(1)
	}
	return resp.(*llms.ContentResponse), args.Error(1)
}

// setupTestRequest creates a FlowNodeRequest for testing Execute
func setupTestRequest() *node.FlowNodeRequest {
	return &node.FlowNodeRequest{
		EdgeSourceMap: mflow.EdgesMap{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{},
		VarMap:        map[string]any{},
		ReadWriteLock: &sync.RWMutex{},
	}
}

func TestNewNodeAiProvider(t *testing.T) {
	id := idwrap.NewNow()
	credID := idwrap.NewNow()
	temp := float32(0.7)
	maxTokens := int32(4096)

	n := New(id, "TestAiProvider", &credID, mflow.AiModelGpt52, "", &temp, &maxTokens)

	assert.Equal(t, id, n.GetID())
	assert.Equal(t, "TestAiProvider", n.GetName())
	require.NotNil(t, n.CredentialID)
	assert.Equal(t, credID, *n.CredentialID)
	assert.Equal(t, mflow.AiModelGpt52, n.Model)
	assert.Equal(t, "", n.CustomModel)
	require.NotNil(t, n.Temperature)
	assert.Equal(t, float32(0.7), *n.Temperature)
	require.NotNil(t, n.MaxTokens)
	assert.Equal(t, int32(4096), *n.MaxTokens)
}

func TestNewNodeAiProvider_NilOptionalFields(t *testing.T) {
	id := idwrap.NewNow()
	credID := idwrap.NewNow()

	n := New(id, "TestAiProvider", &credID, mflow.AiModelClaudeSonnet45, "custom-model", nil, nil)

	assert.Equal(t, id, n.GetID())
	assert.Equal(t, "TestAiProvider", n.GetName())
	assert.Equal(t, mflow.AiModelClaudeSonnet45, n.Model)
	assert.Equal(t, "custom-model", n.CustomModel)
	assert.Nil(t, n.Temperature)
	assert.Nil(t, n.MaxTokens)
}

func TestNewNodeAiProvider_NilCredentialID(t *testing.T) {
	id := idwrap.NewNow()

	n := New(id, "TestAiProvider", nil, mflow.AiModelClaudeSonnet45, "", nil, nil)

	assert.Equal(t, id, n.GetID())
	assert.Nil(t, n.CredentialID)
}

func TestNodeAiProvider_Execute_MakesLLMCall(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mm := new(mockModel)

	// Create provider node with mock LLM
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)
	n.LLM = mm

	// Set up mock to return a response with token usage in GenerationInfo
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content:    "Hello, how can I help you?",
				StopReason: "stop",
				GenerationInfo: map[string]any{
					"PromptTokens":     10,
					"CompletionTokens": 8,
				},
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	// Create typed input using our llm types
	input := nai.AIProviderInput{
		Messages: []llm.Message{
			{
				Role:  llm.RoleUser,
				Parts: []llm.ContentPart{llm.TextPart("Hello")},
			},
		},
	}

	req := setupTestRequest()
	output, err := n.Execute(context.Background(), req, input)

	assert.NoError(t, err)
	require.NotNil(t, output)
	assert.Equal(t, "Hello, how can I help you?", output.Text)
	mm.AssertExpectations(t)

	// Verify output was also written to VarMap for observability
	varMapOutput, ok := req.VarMap["AiProvider"]
	require.True(t, ok)
	outputMap, ok := varMapOutput.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Hello, how can I help you?", outputMap["text"])
}

func TestNodeAiProvider_Execute_ReturnsMetrics(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mm := new(mockModel)
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)
	n.LLM = mm

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content:    "Response text",
				StopReason: "stop",
				GenerationInfo: map[string]any{
					"PromptTokens":     100,
					"CompletionTokens": 50,
				},
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	input := nai.AIProviderInput{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.ContentPart{llm.TextPart("Test")}},
		},
	}
	req := setupTestRequest()

	output, err := n.Execute(context.Background(), req, input)
	assert.NoError(t, err)
	require.NotNil(t, output)

	// Verify metrics in typed output
	assert.Equal(t, int32(100), output.Metrics.PromptTokens)
	assert.Equal(t, int32(50), output.Metrics.CompletionTokens)
	assert.Equal(t, int32(150), output.Metrics.TotalTokens)
	assert.Equal(t, "gpt-5.2", output.Metrics.Model)
	assert.Equal(t, "openai", output.Metrics.Provider)
	assert.Equal(t, "stop", output.Metrics.FinishReason)
}

func TestNodeAiProvider_Execute_ReturnsToolCalls(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mm := new(mockModel)
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)
	n.LLM = mm

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				ToolCalls: []llms.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						FunctionCall: &llms.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location": "NYC"}`,
						},
					},
				},
				StopReason: "tool_calls",
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	input := nai.AIProviderInput{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.ContentPart{llm.TextPart("Test")}},
		},
	}
	req := setupTestRequest()

	output, err := n.Execute(context.Background(), req, input)
	assert.NoError(t, err)
	require.NotNil(t, output)

	// Verify tool calls in typed output
	require.Len(t, output.ToolCalls, 1)
	assert.Equal(t, "call_123", output.ToolCalls[0].ID)
	assert.Equal(t, "function", output.ToolCalls[0].Type)
	assert.Equal(t, "get_weather", output.ToolCalls[0].Name)
	assert.Equal(t, `{"location": "NYC"}`, output.ToolCalls[0].Arguments)
}

func TestNodeAiProvider_Execute_ResolvesPromptVariables(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mm := new(mockModel)
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)
	n.LLM = mm
	n.Prompt = "You are helping {{user_name}}. Be {{tone}}."

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: "OK", StopReason: "stop"},
		},
	}

	// Capture the messages sent to the model
	var capturedMessages []llms.MessageContent
	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedMessages = args.Get(1).([]llms.MessageContent)
	}).Return(resp, nil)

	input := nai.AIProviderInput{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.ContentPart{llm.TextPart("Hello")}},
		},
	}
	req := setupTestRequest()
	// Add variables needed for prompt resolution
	req.VarMap["user_name"] = "Alice"
	req.VarMap["tone"] = "friendly"

	output, err := n.Execute(context.Background(), req, input)
	assert.NoError(t, err)
	require.NotNil(t, output)

	// First message should be system prompt with resolved variables
	require.Len(t, capturedMessages, 2)
	assert.Equal(t, llms.ChatMessageTypeSystem, capturedMessages[0].Role)
	textPart := capturedMessages[0].Parts[0].(llms.TextContent)
	assert.Equal(t, "You are helping Alice. Be friendly.", textPart.Text)
}

func TestNodeAiProvider_Execute_MissingFactory(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	// No LLM mock and no factory
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)

	input := nai.AIProviderInput{Messages: []llm.Message{}}
	req := setupTestRequest()

	output, err := n.Execute(context.Background(), req, input)
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "requires LLM provider factory")
}

func TestNodeAiProvider_Execute_LLMError(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mm := new(mockModel)
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)
	n.LLM = mm

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("API rate limit exceeded"))

	input := nai.AIProviderInput{Messages: []llm.Message{}}
	req := setupTestRequest()

	output, err := n.Execute(context.Background(), req, input)
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "LLM call failed")
	assert.Contains(t, err.Error(), "API rate limit exceeded")
}

func TestNodeAiProvider_Execute_EmptyResponse(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mm := new(mockModel)
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)
	n.LLM = mm

	// Empty choices
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	input := nai.AIProviderInput{Messages: []llm.Message{}}
	req := setupTestRequest()

	output, err := n.Execute(context.Background(), req, input)
	assert.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "empty response")
}

func TestNodeAiProvider_Execute_WithTools(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mm := new(mockModel)
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)
	n.LLM = mm

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: "I'll help you", StopReason: "stop"},
		},
	}

	// Verify tools are passed to the model
	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.MatchedBy(func(opts []llms.CallOption) bool {
		// Just verify we got some options
		return len(opts) > 0
	})).Return(resp, nil)

	input := nai.AIProviderInput{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Parts: []llm.ContentPart{llm.TextPart("Test")}},
		},
		Tools: []llm.Tool{
			{
				Type: "function",
				Function: &llm.FunctionDef{
					Name:        "test_tool",
					Description: "A test tool",
				},
			},
		},
	}

	req := setupTestRequest()

	output, err := n.Execute(context.Background(), req, input)
	assert.NoError(t, err)
	require.NotNil(t, output)
	mm.AssertExpectations(t)
}

func TestNodeAiProvider_RunSync_ReturnsError(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	n := New(nodeID, "AiProvider", &credID, mflow.AiModelGpt52, "", nil, nil)

	req := setupTestRequest()

	// RunSync should error because provider should be called via Execute
	result := n.RunSync(context.Background(), req)
	assert.Error(t, result.Err)
	assert.Contains(t, result.Err.Error(), "should be called via Execute")
}

func TestNodeAiProvider_AllModelTypes(t *testing.T) {
	models := []mflow.AiModel{
		mflow.AiModelGpt52,
		mflow.AiModelGpt52Pro,
		mflow.AiModelClaudeSonnet45,
		mflow.AiModelClaudeOpus45,
		mflow.AiModelGemini3Flash,
		mflow.AiModelO3,
	}

	for _, model := range models {
		t.Run(fmt.Sprintf("model_%d", model), func(t *testing.T) {
			id := idwrap.NewNow()
			credID := idwrap.NewNow()

			n := New(id, "AiProvider", &credID, model, "", nil, nil)
			assert.Equal(t, model, n.Model)
		})
	}
}

func TestNodeAiProvider_GetRequiredVariables(t *testing.T) {
	n := &NodeAiProvider{
		Prompt: "Hello {{name}}, your score is {{score}}",
	}

	vars := n.GetRequiredVariables()
	assert.Contains(t, vars, "name")
	assert.Contains(t, vars, "score")
}

func TestNodeAiProvider_GetOutputVariables(t *testing.T) {
	n := &NodeAiProvider{}
	vars := n.GetOutputVariables()

	assert.Contains(t, vars, "text")
	assert.Contains(t, vars, "tool_calls")
	assert.Contains(t, vars, "metrics")
}

func TestNodeAiProvider_CustomModel(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mm := new(mockModel)
	n := New(nodeID, "AiProvider", &credID, mflow.AiModelCustom, "my-custom-model", nil, nil)
	n.LLM = mm

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{Content: "Custom response", StopReason: "stop"},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	input := nai.AIProviderInput{Messages: []llm.Message{}}
	req := setupTestRequest()

	output, err := n.Execute(context.Background(), req, input)
	assert.NoError(t, err)
	require.NotNil(t, output)

	// Verify custom model is in metrics
	assert.Equal(t, "my-custom-model", output.Metrics.Model)
}

func TestNodeAiProvider_GetModelString(t *testing.T) {
	tests := []struct {
		name        string
		model       mflow.AiModel
		customModel string
		expected    string
	}{
		{"GPT-5.2", mflow.AiModelGpt52, "", "gpt-5.2"},
		{"Custom", mflow.AiModelCustom, "my-model", "my-model"},
		{"Claude Sonnet", mflow.AiModelClaudeSonnet45, "", "claude-sonnet-4-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(idwrap.NewNow(), "Test", nil, tt.model, tt.customModel, nil, nil)
			assert.Equal(t, tt.expected, n.GetModelString())
		})
	}
}

func TestNodeAiProvider_GetProviderString(t *testing.T) {
	tests := []struct {
		name     string
		model    mflow.AiModel
		expected string
	}{
		{"OpenAI", mflow.AiModelGpt52, "openai"},
		{"Anthropic", mflow.AiModelClaudeSonnet45, "anthropic"},
		{"Google", mflow.AiModelGemini3Flash, "google"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := New(idwrap.NewNow(), "Test", nil, tt.model, "", nil, nil)
			assert.Equal(t, tt.expected, n.GetProviderString())
		})
	}
}

func TestNodeAiProvider_SetProviderFactory(t *testing.T) {
	n := New(idwrap.NewNow(), "Test", nil, mflow.AiModelGpt52, "", nil, nil)
	assert.Nil(t, n.ProviderFactory)

	// SetProviderFactory should accept the typed factory
	// (We can't easily test this without a real factory, but we verify the method exists)
	n.SetProviderFactory(nil)
	assert.Nil(t, n.ProviderFactory)
}

func TestNodeAiProvider_SetLLM(t *testing.T) {
	n := New(idwrap.NewNow(), "Test", nil, mflow.AiModelGpt52, "", nil, nil)
	assert.Nil(t, n.LLM)

	mm := new(mockModel)
	n.SetLLM(mm)
	assert.Equal(t, mm, n.LLM)
}
