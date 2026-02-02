package nmemory

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func TestNewNodeMemory(t *testing.T) {
	id := idwrap.NewNow()
	n := New(id, "TestMemory", mflow.AiMemoryTypeWindowBuffer, 10)

	assert.Equal(t, id, n.GetID())
	assert.Equal(t, "TestMemory", n.GetName())
	assert.Equal(t, mflow.AiMemoryTypeWindowBuffer, n.MemoryType)
	assert.Equal(t, int32(10), n.WindowSize)
	assert.Empty(t, n.Messages)
}

func TestNodeMemory_AddMessage(t *testing.T) {
	n := New(idwrap.NewNow(), "Memory", mflow.AiMemoryTypeWindowBuffer, 100)

	n.AddMessage("user", "Hello")
	n.AddMessage("assistant", "Hi there!")
	n.AddMessage("user", "How are you?")

	messages := n.GetMessages()
	require.Len(t, messages, 3)
	assert.Equal(t, "user", messages[0].Role)
	assert.Equal(t, "Hello", messages[0].Content)
	assert.Equal(t, "assistant", messages[1].Role)
	assert.Equal(t, "Hi there!", messages[1].Content)
	assert.Equal(t, "user", messages[2].Role)
	assert.Equal(t, "How are you?", messages[2].Content)
}

func TestNodeMemory_WindowBuffer(t *testing.T) {
	// Create memory with window size of 3
	n := New(idwrap.NewNow(), "Memory", mflow.AiMemoryTypeWindowBuffer, 3)

	// Add 5 messages
	n.AddMessage("user", "Message 1")
	n.AddMessage("assistant", "Message 2")
	n.AddMessage("user", "Message 3")
	n.AddMessage("assistant", "Message 4")
	n.AddMessage("user", "Message 5")

	// Should only keep the last 3 messages
	messages := n.GetMessages()
	require.Len(t, messages, 3)
	assert.Equal(t, "Message 3", messages[0].Content)
	assert.Equal(t, "Message 4", messages[1].Content)
	assert.Equal(t, "Message 5", messages[2].Content)
}

func TestNodeMemory_WindowBuffer_ExactSize(t *testing.T) {
	n := New(idwrap.NewNow(), "Memory", mflow.AiMemoryTypeWindowBuffer, 2)

	n.AddMessage("user", "First")
	n.AddMessage("assistant", "Second")

	messages := n.GetMessages()
	require.Len(t, messages, 2)
	assert.Equal(t, "First", messages[0].Content)
	assert.Equal(t, "Second", messages[1].Content)

	// Add one more - should evict the first
	n.AddMessage("user", "Third")

	messages = n.GetMessages()
	require.Len(t, messages, 2)
	assert.Equal(t, "Second", messages[0].Content)
	assert.Equal(t, "Third", messages[1].Content)
}

func TestNodeMemory_Clear(t *testing.T) {
	n := New(idwrap.NewNow(), "Memory", mflow.AiMemoryTypeWindowBuffer, 10)

	n.AddMessage("user", "Hello")
	n.AddMessage("assistant", "Hi")

	require.Len(t, n.GetMessages(), 2)

	n.Clear()

	assert.Empty(t, n.GetMessages())
	assert.Equal(t, 0, n.Len())
}

func TestNodeMemory_Len(t *testing.T) {
	n := New(idwrap.NewNow(), "Memory", mflow.AiMemoryTypeWindowBuffer, 10)

	assert.Equal(t, 0, n.Len())

	n.AddMessage("user", "Hello")
	assert.Equal(t, 1, n.Len())

	n.AddMessage("assistant", "Hi")
	assert.Equal(t, 2, n.Len())

	n.Clear()
	assert.Equal(t, 0, n.Len())
}

func TestNodeMemory_GetMessages_ReturnsCopy(t *testing.T) {
	n := New(idwrap.NewNow(), "Memory", mflow.AiMemoryTypeWindowBuffer, 10)

	n.AddMessage("user", "Hello")

	messages := n.GetMessages()
	messages[0].Content = "Modified"

	// Original should not be affected
	originalMessages := n.GetMessages()
	assert.Equal(t, "Hello", originalMessages[0].Content)
}

func TestNodeMemory_ConcurrentAccess(t *testing.T) {
	n := New(idwrap.NewNow(), "Memory", mflow.AiMemoryTypeWindowBuffer, 100)

	var wg sync.WaitGroup

	// Spawn multiple goroutines adding messages
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				n.AddMessage("user", "message")
			}
		}(i)
	}

	// Spawn readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = n.GetMessages()
				_ = n.Len()
			}
		}()
	}

	wg.Wait()

	// Should have 100 messages (10 goroutines * 10 messages each)
	assert.Equal(t, 100, n.Len())
}

func TestNodeMemory_RunSync_PassesThrough(t *testing.T) {
	nodeID := idwrap.NewNow()
	nextID := idwrap.NewNow()

	n := New(nodeID, "Memory", mflow.AiMemoryTypeWindowBuffer, 10)

	// Setup edge map for pass-through
	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleUnspecified: []idwrap.IDWrap{nextID},
		},
	}

	req := &node.FlowNodeRequest{
		EdgeSourceMap: edgeMap,
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	result := n.RunSync(context.Background(), req)

	assert.NoError(t, result.Err)
	require.Len(t, result.NextNodeID, 1)
	assert.Equal(t, nextID, result.NextNodeID[0])
}

func TestNodeMemory_RunSync_NoNextNode(t *testing.T) {
	nodeID := idwrap.NewNow()
	n := New(nodeID, "Memory", mflow.AiMemoryTypeWindowBuffer, 10)

	req := &node.FlowNodeRequest{
		EdgeSourceMap: mflow.EdgesMap{},
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	result := n.RunSync(context.Background(), req)

	assert.NoError(t, result.Err)
	assert.Empty(t, result.NextNodeID)
}

func TestNodeMemory_RunAsync(t *testing.T) {
	nodeID := idwrap.NewNow()
	n := New(nodeID, "Memory", mflow.AiMemoryTypeWindowBuffer, 10)

	req := &node.FlowNodeRequest{
		EdgeSourceMap: mflow.EdgesMap{},
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	resultChan := make(chan node.FlowNodeResult, 1)
	n.RunAsync(context.Background(), req, resultChan)

	result := <-resultChan
	assert.NoError(t, result.Err)
}
