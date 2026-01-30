package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tmc/langchaingo/llms"
)

func TestToLangChainMessages(t *testing.T) {
	msgs := []Message{
		{
			Role:  RoleSystem,
			Parts: []ContentPart{TextPart("You are a helpful assistant.")},
		},
		{
			Role:  RoleUser,
			Parts: []ContentPart{TextPart("Hello!")},
		},
		{
			Role:  RoleAssistant,
			Parts: []ContentPart{TextPart("Hi! How can I help you?")},
		},
	}

	result := ToLangChainMessages(msgs)

	require.Len(t, result, 3)

	// System message
	assert.Equal(t, llms.ChatMessageTypeSystem, result[0].Role)
	require.Len(t, result[0].Parts, 1)
	textPart, ok := result[0].Parts[0].(llms.TextContent)
	require.True(t, ok)
	assert.Equal(t, "You are a helpful assistant.", textPart.Text)

	// User message
	assert.Equal(t, llms.ChatMessageTypeHuman, result[1].Role)
	require.Len(t, result[1].Parts, 1)
	textPart, ok = result[1].Parts[0].(llms.TextContent)
	require.True(t, ok)
	assert.Equal(t, "Hello!", textPart.Text)

	// Assistant message
	assert.Equal(t, llms.ChatMessageTypeAI, result[2].Role)
	require.Len(t, result[2].Parts, 1)
	textPart, ok = result[2].Parts[0].(llms.TextContent)
	require.True(t, ok)
	assert.Equal(t, "Hi! How can I help you?", textPart.Text)
}

func TestToLangChainMessage_WithToolCall(t *testing.T) {
	msg := Message{
		Role: RoleAssistant,
		Parts: []ContentPart{
			ToolCall{
				ID:           "call_123",
				Type:         "function",
				FunctionName: "get_weather",
				Arguments:    `{"location": "NYC"}`,
			},
		},
	}

	result := ToLangChainMessage(msg)

	assert.Equal(t, llms.ChatMessageTypeAI, result.Role)
	require.Len(t, result.Parts, 1)

	toolCall, ok := result.Parts[0].(llms.ToolCall)
	require.True(t, ok)
	assert.Equal(t, "call_123", toolCall.ID)
	assert.Equal(t, "function", toolCall.Type)
	require.NotNil(t, toolCall.FunctionCall)
	assert.Equal(t, "get_weather", toolCall.FunctionCall.Name)
	assert.Equal(t, `{"location": "NYC"}`, toolCall.FunctionCall.Arguments)
}

func TestToLangChainMessage_WithToolCallResponse(t *testing.T) {
	msg := Message{
		Role: RoleTool,
		Parts: []ContentPart{
			ToolCallResponse{
				ToolCallID: "call_123",
				Name:       "get_weather",
				Content:    `{"temp": 72, "condition": "sunny"}`,
			},
		},
	}

	result := ToLangChainMessage(msg)

	assert.Equal(t, llms.ChatMessageTypeTool, result.Role)
	require.Len(t, result.Parts, 1)

	toolResp, ok := result.Parts[0].(llms.ToolCallResponse)
	require.True(t, ok)
	assert.Equal(t, "call_123", toolResp.ToolCallID)
	assert.Equal(t, "get_weather", toolResp.Name)
	assert.Equal(t, `{"temp": 72, "condition": "sunny"}`, toolResp.Content)
}

func TestToLangChainMessage_ToolCallDefaultsToFunction(t *testing.T) {
	msg := Message{
		Role: RoleAssistant,
		Parts: []ContentPart{
			ToolCall{
				ID:           "call_123",
				Type:         "", // Empty type
				FunctionName: "test_tool",
				Arguments:    "{}",
			},
		},
	}

	result := ToLangChainMessage(msg)
	toolCall := result.Parts[0].(llms.ToolCall)
	assert.Equal(t, "function", toolCall.Type)
}

func TestToLangChainTools(t *testing.T) {
	tools := []Tool{
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_weather",
				Description: "Get the current weather",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type":        "string",
							"description": "The city name",
						},
					},
					"required": []string{"location"},
				},
			},
		},
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "get_time",
				Description: "Get the current time",
				Parameters:  map[string]any{},
			},
		},
	}

	result := ToLangChainTools(tools)

	require.Len(t, result, 2)

	// First tool
	assert.Equal(t, "function", result[0].Type)
	require.NotNil(t, result[0].Function)
	assert.Equal(t, "get_weather", result[0].Function.Name)
	assert.Equal(t, "Get the current weather", result[0].Function.Description)
	assert.NotNil(t, result[0].Function.Parameters)

	// Second tool
	assert.Equal(t, "function", result[1].Type)
	require.NotNil(t, result[1].Function)
	assert.Equal(t, "get_time", result[1].Function.Name)
}

func TestToLangChainTool_NilFunction(t *testing.T) {
	tool := Tool{
		Type:     "function",
		Function: nil,
	}

	result := ToLangChainTool(tool)

	assert.Equal(t, "function", result.Type)
	assert.Nil(t, result.Function)
}

func TestToLangChainOptions(t *testing.T) {
	temp := 0.7
	maxTokens := 1000
	tools := []Tool{
		{
			Type: "function",
			Function: &FunctionDef{
				Name:        "test",
				Description: "Test tool",
			},
		},
	}

	opts := &CallOptions{
		Tools:       tools,
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	}

	result := ToLangChainOptions(opts)

	// Should have 3 options: tools, temperature, max tokens
	assert.Len(t, result, 3)
}

func TestToLangChainOptions_Nil(t *testing.T) {
	result := ToLangChainOptions(nil)
	assert.Nil(t, result)
}

func TestToLangChainOptions_Empty(t *testing.T) {
	opts := &CallOptions{}
	result := ToLangChainOptions(opts)
	assert.Empty(t, result)
}

func TestFromLangChainToolCalls(t *testing.T) {
	lcToolCalls := []llms.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			FunctionCall: &llms.FunctionCall{
				Name:      "get_weather",
				Arguments: `{"city": "NYC"}`,
			},
		},
		{
			ID:   "call_2",
			Type: "function",
			FunctionCall: &llms.FunctionCall{
				Name:      "get_time",
				Arguments: `{}`,
			},
		},
	}

	result := FromLangChainToolCalls(lcToolCalls)

	require.Len(t, result, 2)

	assert.Equal(t, "call_1", result[0].ID)
	assert.Equal(t, "function", result[0].Type)
	assert.Equal(t, "get_weather", result[0].FunctionName)
	assert.Equal(t, `{"city": "NYC"}`, result[0].Arguments)

	assert.Equal(t, "call_2", result[1].ID)
	assert.Equal(t, "get_time", result[1].FunctionName)
}

func TestFromLangChainToolCalls_NilFunctionCall(t *testing.T) {
	lcToolCalls := []llms.ToolCall{
		{
			ID:           "call_1",
			Type:         "function",
			FunctionCall: nil,
		},
	}

	result := FromLangChainToolCalls(lcToolCalls)

	require.Len(t, result, 1)
	assert.Equal(t, "call_1", result[0].ID)
	assert.Equal(t, "", result[0].FunctionName)
	assert.Equal(t, "", result[0].Arguments)
}

func TestFromLangChainRole(t *testing.T) {
	tests := []struct {
		lcRole   llms.ChatMessageType
		expected MessageRole
	}{
		{llms.ChatMessageTypeSystem, RoleSystem},
		{llms.ChatMessageTypeHuman, RoleUser},
		{llms.ChatMessageTypeAI, RoleAssistant},
		{llms.ChatMessageTypeTool, RoleTool},
		{llms.ChatMessageType("unknown"), RoleUser}, // Default to user
	}

	for _, tt := range tests {
		t.Run(string(tt.lcRole), func(t *testing.T) {
			result := FromLangChainRole(tt.lcRole)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToLangChainRole(t *testing.T) {
	tests := []struct {
		role     MessageRole
		expected llms.ChatMessageType
	}{
		{RoleSystem, llms.ChatMessageTypeSystem},
		{RoleUser, llms.ChatMessageTypeHuman},
		{RoleAssistant, llms.ChatMessageTypeAI},
		{RoleTool, llms.ChatMessageTypeTool},
		{MessageRole("unknown"), llms.ChatMessageTypeHuman}, // Default to human
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			result := toLangChainRole(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyOptions(t *testing.T) {
	tools := []Tool{{Type: "function"}}

	opts := ApplyOptions(
		WithTools(tools),
		WithTemperature(0.5),
		WithMaxTokens(500),
	)

	require.NotNil(t, opts)
	assert.Equal(t, tools, opts.Tools)
	require.NotNil(t, opts.Temperature)
	assert.Equal(t, 0.5, *opts.Temperature)
	require.NotNil(t, opts.MaxTokens)
	assert.Equal(t, 500, *opts.MaxTokens)
}

func TestApplyOptions_Empty(t *testing.T) {
	opts := ApplyOptions()

	require.NotNil(t, opts)
	assert.Nil(t, opts.Tools)
	assert.Nil(t, opts.Temperature)
	assert.Nil(t, opts.MaxTokens)
}
