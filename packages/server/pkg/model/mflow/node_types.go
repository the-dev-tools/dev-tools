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

type AiModel int8

const (
	// OpenAI - GPT-5.2 family
	AiModelGpt52Instant AiModel = iota
	AiModelGpt52Thinking
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
)

// ModelString returns the API model string for the LLM provider
func (m AiModel) ModelString() string {
	switch m {
	case AiModelGpt52Instant:
		return "gpt-5.2-instant"
	case AiModelGpt52Thinking:
		return "gpt-5.2-thinking"
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
	case AiModelGemini3Pro:
		return "gemini-3-pro"
	case AiModelGemini3Flash:
		return "gemini-3-flash"
	default:
		return "gpt-5.2-instant"
	}
}

// Provider returns the provider type for the model
func (m AiModel) Provider() string {
	switch m {
	case AiModelGpt52Instant, AiModelGpt52Thinking, AiModelGpt52Pro, AiModelGpt52Codex, AiModelO3, AiModelO4Mini:
		return "openai"
	case AiModelClaudeOpus45, AiModelClaudeSonnet45, AiModelClaudeHaiku45:
		return "anthropic"
	case AiModelGemini3Pro, AiModelGemini3Flash:
		return "google"
	default:
		return "openai"
	}
}

type NodeAI struct {
	FlowNodeID   idwrap.IDWrap
	Model        AiModel
	CredentialID idwrap.IDWrap
	Prompt       string
}
