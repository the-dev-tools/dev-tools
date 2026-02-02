// Package llm provides an abstraction layer for LLM types.
// This package wraps langchaingo types so that the orchestrator and business logic
// never import langchaingo directly. The provider layer handles conversion.
package llm

// MessageRole represents the role of a message in a conversation.
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// CallOption is a function that modifies CallOptions.
type CallOption func(*CallOptions)

// CallOptions holds the options for an LLM call.
type CallOptions struct {
	Tools       []Tool
	Temperature *float64
	MaxTokens   *int
}

// WithTools sets the tools available for the LLM call.
func WithTools(tools []Tool) CallOption {
	return func(o *CallOptions) {
		o.Tools = tools
	}
}

// WithTemperature sets the temperature for the LLM call.
func WithTemperature(temp float64) CallOption {
	return func(o *CallOptions) {
		o.Temperature = &temp
	}
}

// WithMaxTokens sets the max tokens for the LLM call.
func WithMaxTokens(tokens int) CallOption {
	return func(o *CallOptions) {
		o.MaxTokens = &tokens
	}
}

// ApplyOptions applies a list of CallOptions to a CallOptions struct.
func ApplyOptions(opts ...CallOption) *CallOptions {
	options := &CallOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}
