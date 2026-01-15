// Package nmemory provides the Memory node implementation for flow execution.
// Memory nodes are passive configuration providers that supply conversation
// history to connected AI Agent nodes via HandleAiMemory edges.
package nmemory

import (
	"context"
	"sync"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// Message represents a single message in the conversation history.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// NodeMemory represents a Memory node that provides conversation history to AI Agent nodes.
// It is a passive node - it does not execute but provides and manages conversation
// memory when discovered by AI nodes via HandleAiMemory edges.
type NodeMemory struct {
	FlowNodeID idwrap.IDWrap
	Name       string
	MemoryType mflow.AiMemoryType
	WindowSize int32

	// Runtime state - conversation history (protected by mutex).
	// Messages stores conversation history in-memory for the current flow execution.
	// TODO(persistent-kv): Messages will be persisted when key-value store is implemented,
	// enabling cross-execution conversation continuity.
	mu       sync.RWMutex
	Messages []Message
}

// New creates a new NodeMemory with the given configuration.
func New(
	id idwrap.IDWrap,
	name string,
	memoryType mflow.AiMemoryType,
	windowSize int32,
) *NodeMemory {
	return &NodeMemory{
		FlowNodeID: id,
		Name:       name,
		MemoryType: memoryType,
		WindowSize: windowSize,
		Messages:   make([]Message, 0),
	}
}

// GetID returns the node's unique identifier.
func (n *NodeMemory) GetID() idwrap.IDWrap { return n.FlowNodeID }

// GetName returns the node's display name.
func (n *NodeMemory) GetName() string { return n.Name }

// RunSync is a no-op for Memory nodes. Memory nodes are passive state containers
// and do not execute directly. They are discovered by AI Agent nodes
// via HandleAiMemory edges.
func (n *NodeMemory) RunSync(_ context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	// Memory nodes are passive - they don't produce output or trigger next nodes.
	// They are read/written by AI Agent nodes via edge connections.
	next := mflow.GetNextNodeID(req.EdgeSourceMap, n.FlowNodeID, mflow.HandleUnspecified)
	return node.FlowNodeResult{
		NextNodeID: next,
		Err:        nil,
	}
}

// RunAsync runs the node asynchronously by calling RunSync and sending the result.
func (n *NodeMemory) RunAsync(ctx context.Context, req *node.FlowNodeRequest, resultChan chan node.FlowNodeResult) {
	resultChan <- n.RunSync(ctx, req)
}

// AddMessage appends a message to the conversation history.
// For WindowBuffer memory type, it enforces the window size limit by
// removing the oldest messages when the limit is exceeded.
func (n *NodeMemory) AddMessage(role, content string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.Messages = append(n.Messages, Message{Role: role, Content: content})

	// Enforce window size for WindowBuffer type
	//nolint:gosec // G115: Window size is bounded by flow configuration, no realistic overflow
	if n.MemoryType == mflow.AiMemoryTypeWindowBuffer && int32(len(n.Messages)) > n.WindowSize {
		// Keep only the last WindowSize messages
		excess := len(n.Messages) - int(n.WindowSize)
		n.Messages = n.Messages[excess:]
	}
}

// GetMessages returns a copy of the current conversation history.
// Returns a copy to prevent concurrent modification issues.
func (n *NodeMemory) GetMessages() []Message {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Return a copy to prevent external modification
	messages := make([]Message, len(n.Messages))
	copy(messages, n.Messages)
	return messages
}

// Clear resets the conversation history.
func (n *NodeMemory) Clear() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Messages = make([]Message, 0)
}

// Len returns the current number of messages in the history.
func (n *NodeMemory) Len() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.Messages)
}
