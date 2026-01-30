package naiprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tmc/langchaingo/llms"
)

func TestExtractTokensFromResponse_OpenAIStyle(t *testing.T) {
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				GenerationInfo: map[string]any{
					"PromptTokens":     100,
					"CompletionTokens": 50,
				},
			},
		},
	}

	prompt, completion := ExtractTokensFromResponse(resp)

	assert.Equal(t, int32(100), prompt)
	assert.Equal(t, int32(50), completion)
}

func TestExtractTokensFromResponse_AnthropicStyle(t *testing.T) {
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				GenerationInfo: map[string]any{
					"InputTokens":  75,
					"OutputTokens": 25,
				},
			},
		},
	}

	prompt, completion := ExtractTokensFromResponse(resp)

	assert.Equal(t, int32(75), prompt)
	assert.Equal(t, int32(25), completion)
}

func TestExtractTokensFromResponse_NilResponse(t *testing.T) {
	prompt, completion := ExtractTokensFromResponse(nil)

	assert.Equal(t, int32(0), prompt)
	assert.Equal(t, int32(0), completion)
}

func TestExtractTokensFromResponse_EmptyChoices(t *testing.T) {
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{},
	}

	prompt, completion := ExtractTokensFromResponse(resp)

	assert.Equal(t, int32(0), prompt)
	assert.Equal(t, int32(0), completion)
}

func TestExtractTokensFromResponse_NilGenerationInfo(t *testing.T) {
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				Content:        "Some content",
				GenerationInfo: nil,
			},
		},
	}

	prompt, completion := ExtractTokensFromResponse(resp)

	assert.Equal(t, int32(0), prompt)
	assert.Equal(t, int32(0), completion)
}

func TestExtractFinishReason(t *testing.T) {
	tests := []struct {
		name     string
		resp     *llms.ContentResponse
		expected string
	}{
		{
			name:     "nil response",
			resp:     nil,
			expected: "",
		},
		{
			name: "empty choices",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{},
			},
			expected: "",
		},
		{
			name: "stop reason",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{StopReason: "stop"},
				},
			},
			expected: "stop",
		},
		{
			name: "tool_calls reason",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{StopReason: "tool_calls"},
				},
			},
			expected: "tool_calls",
		},
		{
			name: "end_turn reason (Anthropic)",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{StopReason: "end_turn"},
				},
			},
			expected: "end_turn",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractFinishReason(tt.resp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name     string
		resp     *llms.ContentResponse
		expected string
	}{
		{
			name:     "nil response",
			resp:     nil,
			expected: "",
		},
		{
			name: "empty choices",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{},
			},
			expected: "",
		},
		{
			name: "single choice with content",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{Content: "Hello, world!"},
				},
			},
			expected: "Hello, world!",
		},
		{
			name: "multiple choices - first with content",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{Content: "First response"},
					{Content: "Second response"},
				},
			},
			expected: "First response",
		},
		{
			name: "choice with tool calls but no text",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{
						ToolCalls: []llms.ToolCall{
							{ID: "call_1"},
						},
					},
				},
			},
			expected: "",
		},
		{
			name: "Anthropic style - text in second choice",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{ToolCalls: []llms.ToolCall{{ID: "call_1"}}},
					{Content: "Text content here"},
				},
			},
			expected: "Text content here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTextContent(tt.resp)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractToolCalls(t *testing.T) {
	tests := []struct {
		name          string
		resp          *llms.ContentResponse
		expectedCount int
	}{
		{
			name:          "nil response",
			resp:          nil,
			expectedCount: 0,
		},
		{
			name: "empty choices",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{},
			},
			expectedCount: 0,
		},
		{
			name: "single tool call",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{
						ToolCalls: []llms.ToolCall{
							{
								ID: "call_1",
								FunctionCall: &llms.FunctionCall{
									Name:      "get_weather",
									Arguments: `{"location": "NYC"}`,
								},
							},
						},
					},
				},
			},
			expectedCount: 1,
		},
		{
			name: "multiple tool calls in single choice",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{
						ToolCalls: []llms.ToolCall{
							{ID: "call_1", FunctionCall: &llms.FunctionCall{Name: "func1"}},
							{ID: "call_2", FunctionCall: &llms.FunctionCall{Name: "func2"}},
						},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "tool calls across multiple choices",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{ToolCalls: []llms.ToolCall{{ID: "call_1"}}},
					{ToolCalls: []llms.ToolCall{{ID: "call_2"}}},
				},
			},
			expectedCount: 2,
		},
		{
			name: "no tool calls",
			resp: &llms.ContentResponse{
				Choices: []*llms.ContentChoice{
					{Content: "Just text, no tools"},
				},
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractToolCalls(tt.resp)
			assert.Len(t, result, tt.expectedCount)
		})
	}
}

func TestExtractToolCalls_PreservesData(t *testing.T) {
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				ToolCalls: []llms.ToolCall{
					{
						ID: "call_abc123",
						FunctionCall: &llms.FunctionCall{
							Name:      "get_user_info",
							Arguments: `{"user_id": 42}`,
						},
					},
				},
			},
		},
	}

	result := ExtractToolCalls(resp)

	assert.Len(t, result, 1)
	assert.Equal(t, "call_abc123", result[0].ID)
	assert.Equal(t, "get_user_info", result[0].FunctionCall.Name)
	assert.Equal(t, `{"user_id": 42}`, result[0].FunctionCall.Arguments)
}

func TestExtractInt32(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		key      string
		expected int32
	}{
		{
			name:     "int value",
			input:    map[string]any{"count": 42},
			key:      "count",
			expected: 42,
		},
		{
			name:     "int32 value",
			input:    map[string]any{"count": int32(100)},
			key:      "count",
			expected: 100,
		},
		{
			name:     "int64 value",
			input:    map[string]any{"count": int64(200)},
			key:      "count",
			expected: 200,
		},
		{
			name:     "float64 value",
			input:    map[string]any{"count": 150.0},
			key:      "count",
			expected: 150,
		},
		{
			name:     "missing key",
			input:    map[string]any{"other": 42},
			key:      "count",
			expected: 0,
		},
		{
			name:     "string value",
			input:    map[string]any{"count": "42"},
			key:      "count",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInt32(tt.input, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}
