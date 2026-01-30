// Package naiprovider provides the AI Provider node implementation for flow execution.
package naiprovider

import (
	"github.com/tmc/langchaingo/llms"
)

// ExtractTokensFromResponse extracts token usage from a LangChainGo response.
// Different providers return usage information in GenerationInfo map:
// - OpenAI: Uses PromptTokens, CompletionTokens
// - Anthropic: Uses InputTokens, OutputTokens
// Returns (promptTokens, completionTokens).
func ExtractTokensFromResponse(resp *llms.ContentResponse) (prompt, completion int32) {
	if resp == nil || len(resp.Choices) == 0 {
		return 0, 0
	}

	// LangChainGo stores token usage in GenerationInfo map of the first choice
	genInfo := resp.Choices[0].GenerationInfo
	if genInfo == nil {
		return 0, 0
	}

	// Try OpenAI-style keys first
	prompt = extractInt32(genInfo, "PromptTokens")
	if prompt == 0 {
		// Try Anthropic-style keys
		prompt = extractInt32(genInfo, "InputTokens")
	}

	completion = extractInt32(genInfo, "CompletionTokens")
	if completion == 0 {
		// Try Anthropic-style keys
		completion = extractInt32(genInfo, "OutputTokens")
	}

	return prompt, completion
}

// extractInt32 extracts an int32 value from a map with various type conversions.
func extractInt32(m map[string]any, key string) int32 {
	v, ok := m[key]
	if !ok {
		return 0
	}

	switch val := v.(type) {
	case int32:
		return val
	case int:
		//nolint:gosec // G115: bounded by int32 range in practice
		return int32(val)
	case int64:
		//nolint:gosec // G115: bounded by int32 range in practice
		return int32(val)
	case float64:
		return int32(val)
	default:
		return 0
	}
}

// ExtractFinishReason extracts the stop/finish reason from an LLM response.
// Common values: "stop" (OpenAI), "end_turn" (Anthropic), "tool_calls"/"tool_use"
func ExtractFinishReason(resp *llms.ContentResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}

	// Most providers populate StopReason in the first choice
	return resp.Choices[0].StopReason
}

// ExtractTextContent extracts the text content from an LLM response.
// Handles multiple choices by aggregating text from all choices with content.
func ExtractTextContent(resp *llms.ContentResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}

	// Iterate through choices to find text content
	// Anthropic may return multiple choices: one with text, another with tool calls
	for _, choice := range resp.Choices {
		if choice.Content != "" {
			return choice.Content
		}
	}

	return ""
}

// ExtractToolCalls extracts tool calls from an LLM response.
// Aggregates tool calls from all choices since some providers split them.
func ExtractToolCalls(resp *llms.ContentResponse) []llms.ToolCall {
	if resp == nil || len(resp.Choices) == 0 {
		return nil
	}

	var allToolCalls []llms.ToolCall
	for _, choice := range resp.Choices {
		allToolCalls = append(allToolCalls, choice.ToolCalls...)
	}

	return allToolCalls
}
