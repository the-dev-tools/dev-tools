package llm

import (
	"github.com/tmc/langchaingo/llms"
)

// ToLangChainMessages converts our Message types to langchaingo MessageContent.
func ToLangChainMessages(msgs []Message) []llms.MessageContent {
	result := make([]llms.MessageContent, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, ToLangChainMessage(msg))
	}
	return result
}

// ToLangChainMessage converts a single Message to langchaingo MessageContent.
func ToLangChainMessage(msg Message) llms.MessageContent {
	lcMsg := llms.MessageContent{
		Role:  toLangChainRole(msg.Role),
		Parts: make([]llms.ContentPart, 0, len(msg.Parts)),
	}

	for _, part := range msg.Parts {
		switch p := part.(type) {
		case TextContent:
			lcMsg.Parts = append(lcMsg.Parts, llms.TextContent{Text: p.Text})
		case ToolCall:
			toolType := p.Type
			if toolType == "" {
				toolType = "function"
			}
			lcMsg.Parts = append(lcMsg.Parts, llms.ToolCall{
				ID:   p.ID,
				Type: toolType,
				FunctionCall: &llms.FunctionCall{
					Name:      p.FunctionName,
					Arguments: p.Arguments,
				},
			})
		case ToolCallResponse:
			lcMsg.Parts = append(lcMsg.Parts, llms.ToolCallResponse{
				ToolCallID: p.ToolCallID,
				Name:       p.Name,
				Content:    p.Content,
			})
		}
	}

	return lcMsg
}

// toLangChainRole converts our MessageRole to langchaingo ChatMessageType.
func toLangChainRole(role MessageRole) llms.ChatMessageType {
	switch role {
	case RoleSystem:
		return llms.ChatMessageTypeSystem
	case RoleUser:
		return llms.ChatMessageTypeHuman
	case RoleAssistant:
		return llms.ChatMessageTypeAI
	case RoleTool:
		return llms.ChatMessageTypeTool
	default:
		return llms.ChatMessageTypeHuman
	}
}

// ToLangChainTools converts our Tool types to langchaingo Tools.
func ToLangChainTools(tools []Tool) []llms.Tool {
	result := make([]llms.Tool, 0, len(tools))
	for _, tool := range tools {
		result = append(result, ToLangChainTool(tool))
	}
	return result
}

// ToLangChainTool converts a single Tool to langchaingo Tool.
func ToLangChainTool(tool Tool) llms.Tool {
	lcTool := llms.Tool{
		Type: tool.Type,
	}

	if tool.Function != nil {
		lcTool.Function = &llms.FunctionDefinition{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			Parameters:  tool.Function.Parameters,
		}
	}

	return lcTool
}

// ToLangChainOptions converts our CallOptions to langchaingo CallOptions.
func ToLangChainOptions(opts *CallOptions) []llms.CallOption {
	if opts == nil {
		return nil
	}

	var result []llms.CallOption

	if len(opts.Tools) > 0 {
		result = append(result, llms.WithTools(ToLangChainTools(opts.Tools)))
	}
	if opts.Temperature != nil {
		result = append(result, llms.WithTemperature(*opts.Temperature))
	}
	if opts.MaxTokens != nil {
		result = append(result, llms.WithMaxTokens(*opts.MaxTokens))
	}

	return result
}

// FromLangChainToolCalls converts langchaingo ToolCalls to our ToolCall type.
func FromLangChainToolCalls(tcs []llms.ToolCall) []ToolCall {
	result := make([]ToolCall, 0, len(tcs))
	for _, tc := range tcs {
		funcName := ""
		funcArgs := ""
		if tc.FunctionCall != nil {
			funcName = tc.FunctionCall.Name
			funcArgs = tc.FunctionCall.Arguments
		}
		result = append(result, ToolCall{
			ID:           tc.ID,
			Type:         tc.Type,
			FunctionName: funcName,
			Arguments:    funcArgs,
		})
	}
	return result
}

// FromLangChainRole converts langchaingo ChatMessageType to our MessageRole.
func FromLangChainRole(role llms.ChatMessageType) MessageRole {
	switch role {
	case llms.ChatMessageTypeSystem:
		return RoleSystem
	case llms.ChatMessageTypeHuman:
		return RoleUser
	case llms.ChatMessageTypeAI:
		return RoleAssistant
	case llms.ChatMessageTypeTool:
		return RoleTool
	default:
		return RoleUser
	}
}
