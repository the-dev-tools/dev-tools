//nolint:revive // exported
package mflow

import (
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/compress"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcondition"
)

// --- Request Node ---

type NodeRequest struct {
	FlowNodeID idwrap.IDWrap
	HttpID     *idwrap.IDWrap

	DeltaHttpID *idwrap.IDWrap

	HasRequestConfig bool
}

// --- JS Node ---

type NodeJS struct {
	FlowNodeID       idwrap.IDWrap
	Code             []byte
	CodeCompressType compress.CompressType
}

// --- ManualStart Node ---

type NodeManualStart struct {
	FlowNodeID idwrap.IDWrap
}

// --- If/Condition Node ---
type NodeIf struct {
	FlowNodeID idwrap.IDWrap
	Condition  mcondition.Condition
	// TODO: Condition type
}

// --- For/ForEach Node ---

type ErrorHandling int8

const (
	ErrorHandling_ERROR_HANDLING_UNSPECIFIED ErrorHandling = 0
	ErrorHandling_ERROR_HANDLING_IGNORE      ErrorHandling = 1
	ErrorHandling_ERROR_HANDLING_BREAK       ErrorHandling = 2
)

type NodeFor struct {
	FlowNodeID    idwrap.IDWrap
	IterCount     int64
	Condition     mcondition.Condition
	ErrorHandling ErrorHandling
}

type NodeForEach struct {
	FlowNodeID     idwrap.IDWrap
	IterExpression string
	Condition      mcondition.Condition
	ErrorHandling  ErrorHandling
}

// --- AI Node ---

// Model string constants
const (
	ModelStringGpt52 = "gpt-5.2"
)

type AiModel int8

const (
	// Unspecified - must be 0 to match proto enum
	AiModelUnspecified AiModel = iota

	// OpenAI - GPT-5.2 family
	AiModelGpt52
	AiModelGpt52Pro
	AiModelGpt52Codex

	// OpenAI - Reasoning models
	AiModelO3
	AiModelO4Mini

	// Anthropic - Claude 4.5 family
	AiModelClaudeOpus45
	AiModelClaudeSonnet45
	AiModelClaudeHaiku45

	// Google - Gemini 3 family
	AiModelGemini3Pro
	AiModelGemini3Flash

	// Custom
	AiModelCustom
)

// ModelString returns the API model string for the LLM provider
func (m AiModel) ModelString() string {
	switch m {
	case AiModelUnspecified:
		return "" // Unspecified model
	case AiModelGpt52:
		return ModelStringGpt52
	case AiModelGpt52Pro:
		return "gpt-5.2-pro"
	case AiModelGpt52Codex:
		return "gpt-5.2-codex"
	case AiModelO3:
		return "o3"
	case AiModelO4Mini:
		return "o4-mini"
	case AiModelClaudeOpus45:
		return "claude-opus-4-5"
	case AiModelClaudeSonnet45:
		return "claude-sonnet-4-5"
	case AiModelClaudeHaiku45:
		return "claude-haiku-4-5"
	// HACK: Using 2.5 instead of 3.0 due to langchaingo bug
	// https://github.com/tmc/langchaingo/issues/1464
	case AiModelGemini3Pro:
		return "gemini-2.5-pro"
	case AiModelGemini3Flash:
		return "gemini-2.5-flash"
	case AiModelCustom:
		return "" // Custom models not yet supported
	default:
		return ModelStringGpt52
	}
}

// Provider returns the provider type for the model
func (m AiModel) Provider() string {
	switch m {
	case AiModelUnspecified:
		return "" // Unspecified
	case AiModelGpt52, AiModelGpt52Pro, AiModelGpt52Codex, AiModelO3, AiModelO4Mini:
		return "openai"
	case AiModelClaudeOpus45, AiModelClaudeSonnet45, AiModelClaudeHaiku45:
		return "anthropic"
	case AiModelGemini3Pro, AiModelGemini3Flash:
		return "google"
	case AiModelCustom:
		return "custom"
	default:
		return "openai"
	}
}

// AiModelFromString parses a model string and returns the corresponding AiModel.
// Returns AiModelCustom if the string doesn't match any known model.
func AiModelFromString(s string) AiModel {
	switch s {
	case ModelStringGpt52:
		return AiModelGpt52
	case "gpt-5.2-pro":
		return AiModelGpt52Pro
	case "gpt-5.2-codex":
		return AiModelGpt52Codex
	case "o3":
		return AiModelO3
	case "o4-mini":
		return AiModelO4Mini
	case "claude-opus-4-5":
		return AiModelClaudeOpus45
	case "claude-sonnet-4-5":
		return AiModelClaudeSonnet45
	case "claude-haiku-4-5":
		return AiModelClaudeHaiku45
	case "gemini-2.5-pro":
		return AiModelGemini3Pro
	case "gemini-2.5-flash":
		return AiModelGemini3Flash
	case "custom", "":
		return AiModelCustom
	default:
		return AiModelCustom
	}
}

type NodeAI struct {
	FlowNodeID    idwrap.IDWrap
	Prompt        string
	MaxIterations int32
}

// --- AI Provider Node ---
// NodeAiProvider is an active LLM executor node that makes LLM calls and tracks metrics.
// It connects via HandleAiProvider edge and is orchestrated by the NodeAI node.
// Each LLM call through this node gets its own node_execution record with metrics.
type NodeAiProvider struct {
	FlowNodeID   idwrap.IDWrap
	CredentialID *idwrap.IDWrap // nil means no credential set yet
	Model        AiModel
	Temperature  *float32 // nil means use provider default
	MaxTokens    *int32   // nil means use provider default
}

// --- AI Metrics and Output Types ---

// AIMetrics contains metrics for a single LLM call
type AIMetrics struct {
	PromptTokens     int32  `json:"prompt_tokens"`
	CompletionTokens int32  `json:"completion_tokens"`
	TotalTokens      int32  `json:"total_tokens"`
	Model            string `json:"model"`
	Provider         string `json:"provider"`
	FinishReason     string `json:"finish_reason,omitempty"`
}

// AIToolCall represents a tool call request from the LLM
type AIToolCall struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // Usually "function"
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// AIProviderOutput represents the output of a single LLM call from NodeAiProvider
type AIProviderOutput struct {
	Text      string       `json:"text,omitempty"`
	ToolCalls []AIToolCall `json:"tool_calls,omitempty"`
	Metrics   AIMetrics    `json:"metrics"`
}

// AITotalMetrics contains aggregated metrics for the entire AI orchestration
type AITotalMetrics struct {
	PromptTokens     int32  `json:"prompt_tokens"`
	CompletionTokens int32  `json:"completion_tokens"`
	TotalTokens      int32  `json:"total_tokens"`
	Model            string `json:"model"`
	Provider         string `json:"provider"`
	LLMCalls         int32  `json:"llm_calls"`
	ToolCalls        int32  `json:"tool_calls"`
}

// --- Memory Node ---
// AiMemoryType represents the type of conversation memory
type AiMemoryType int8

const (
	AiMemoryTypeWindowBuffer AiMemoryType = 0 // Keeps last N messages
)

// NodeMemory is a passive configuration node that provides conversation memory to connected AI Agent nodes.
// It connects via HandleAiMemory edge and manages conversation history.
type NodeMemory struct {
	FlowNodeID idwrap.IDWrap
	MemoryType AiMemoryType
	WindowSize int32
}
