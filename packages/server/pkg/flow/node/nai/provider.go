package nai

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/llm"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
)

// AIProviderInput contains typed input for LLM execution.
// This is the typed interface for passing data from orchestrator to provider,
// avoiding type loss through VarMap (map[string]any).
type AIProviderInput struct {
	Messages []llm.Message
	Tools    []llm.Tool
}

// AIProvider is the interface that LLM provider nodes must implement.
// This allows NodeAI to work with different provider implementations.
type AIProvider interface {
	node.FlowNode

	// Execute runs the LLM with typed input and returns typed output.
	// This is the primary method for orchestrator-to-provider communication,
	// maintaining type safety for messages and tool calls.
	Execute(ctx context.Context, req *node.FlowNodeRequest, input AIProviderInput) (*mflow.AIProviderOutput, error)

	// GetModelString returns the model identifier string (e.g., "gpt-5.2")
	GetModelString() string

	// GetProviderString returns the provider name (e.g., "openai", "anthropic")
	GetProviderString() string

	// SetProviderFactory sets the LLM provider factory on the provider node
	SetProviderFactory(factory *scredential.LLMProviderFactory)

	// SetLLM sets a mock LLM model for testing purposes
	SetLLM(llm any)
}
