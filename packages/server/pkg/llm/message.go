package llm

// Message represents a message in an LLM conversation.
type Message struct {
	Role  MessageRole
	Parts []ContentPart
}

// ContentPart is the interface for message content parts.
// Implemented by TextContent, ToolCall, and ToolCallResponse.
type ContentPart interface {
	isContentPart() // marker method
}

// TextContent represents text content in a message.
type TextContent struct {
	Text string
}

func (TextContent) isContentPart() {}

// TextPart is a helper function to create a TextContent part.
func TextPart(text string) TextContent {
	return TextContent{Text: text}
}

// ToolCall represents a tool call requested by the assistant.
type ToolCall struct {
	ID           string
	Type         string
	FunctionName string
	Arguments    string
}

func (ToolCall) isContentPart() {}

// ToolCallResponse represents the result of a tool call.
type ToolCallResponse struct {
	ToolCallID string
	Name       string
	Content    string
}

func (ToolCallResponse) isContentPart() {}
