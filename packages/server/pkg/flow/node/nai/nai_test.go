package nai

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/mocknode"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nmemory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/naiprovider"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type mockModel struct {
	mock.Mock
}

func (m *mockModel) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	args := m.Called(ctx, prompt, options)
	return args.String(0), args.Error(1)
}

func (m *mockModel) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	args := m.Called(ctx, messages, options)
	return args.Get(0).(*llms.ContentResponse), args.Error(1)
}

// createTestAiProviderNode creates a AI Provider node for testing
func createTestAiProviderNode(id, credentialID idwrap.IDWrap) *naiprovider.NodeAiProvider {
	return &naiprovider.NodeAiProvider{
		FlowNodeID:   id,
		Name:         "Test Model",
		CredentialID: &credentialID,
		Model:        mflow.AiModelGpt52Instant,
	}
}

func TestNodeAIRun(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// Mock 1: Assistant requests get_variable
	resp1 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				ToolCalls: []llms.ToolCall{
					{
						ID: "call_1",
						FunctionCall: &llms.FunctionCall{
							Name:      "get_variable",
							Arguments: `{"key": "my_var"}`,
						},
					},
				},
			},
		},
	}

	// Mock 2: Assistant provides final answer
	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason: "stop",
				Content:    "final answer",
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msg []llms.MessageContent) bool {
		if len(msg) != 1 {
			return false
		}
		part := msg[0].Parts[0].(llms.TextContent)
		return part.Text == "hello Alice"
	}), mock.Anything).Return(resp1, nil)

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msg []llms.MessageContent) bool {
		return len(msg) == 3 // Prompt + AssistantToolCall + ToolResponse
	}), mock.Anything).Return(resp2, nil)

	// Create AI Provider node
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	// Setup edge map with AI Provider node
	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID: providerNode,
	}

	n := New(nodeID, "AI_NODE", "hello {{user_name}}", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"my_var":    "secret_data",
			"user_name": "Alice",
		},
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// Verify variable write
	val, err := node.ReadNodeVar(req, "AI_NODE", "text")
	assert.NoError(t, err)
	assert.Equal(t, "final answer", val)

	mm.AssertExpectations(t)
}

func TestNodeAI_MissingModelNode(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()

	// No AI Provider node connected - should error
	n := New(nodeID, "AI_NODE", "hello", 0, nil)

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: mflow.EdgesMap{},
		NodeMap:       map[idwrap.IDWrap]node.FlowNode{},
	}

	res := n.RunSync(ctx, req)
	assert.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "AI Agent requires a connected AI Provider node")
}

func TestNodeAI_MissingProviderFactory(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	// AI Provider node connected but no LLM override and no ProviderFactory
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID: providerNode,
	}

	n := New(nodeID, "AI_NODE", "hello", 0, nil)

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "AI Agent node requires LLM provider factory")
}

func TestNodeAI_WithConnectedTools(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()
	httpNodeID := idwrap.NewNow()

	mm := new(mockModel)

	// Mock HTTP node that writes output to VarMap
	httpNode := &mocknode.MockNode{
		ID:    httpNodeID,
		OnRun: func() {},
	}
	// Override GetName to return a specific name
	httpNodeName := "GetUsers"

	// Create a custom mock that returns proper name
	customHttpNode := &namedMockNode{
		MockNode: httpNode,
		name:     httpNodeName,
	}

	// Create AI Provider node
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	// Setup edge map: AI node -> HTTP node via HandleAiTools, Model via HandleAiProvider
	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiTools: []idwrap.IDWrap{httpNodeID},
		},
	}

	// Setup node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		httpNodeID:  customHttpNode,
		providerNodeID: providerNode,
	}

	// Mock: LLM calls the GetUsers tool
	resp1 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				ToolCalls: []llms.ToolCall{
					{
						ID: "call_http",
						FunctionCall: &llms.FunctionCall{
							Name:      httpNodeName,
							Arguments: `{}`,
						},
					},
				},
			},
		},
	}

	// Mock: LLM returns final answer after seeing tool result
	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason: "stop",
				Content:    "Found 2 users: alice and bob",
			},
		},
	}

	// First call returns tool call, second returns final answer
	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		return len(msgs) == 1 // Initial prompt only
	}), mock.Anything).Return(resp1, nil).Once()

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		return len(msgs) > 1 // Has tool response
	}), mock.Anything).Return(resp2, nil).Once()

	n := New(nodeID, "AI_NODE", "Get all users", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	// Simulate HTTP node writing output (this happens when the tool executes)
	customHttpNode.OnRun = func() {
		req.VarMap[httpNodeName] = map[string]any{
			"response": map[string]any{
				"status": float64(200),
				"body":   map[string]any{"users": []string{"alice", "bob"}},
			},
		}
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// Verify AI node wrote its result
	val, err := node.ReadNodeVar(req, "AI_NODE", "text")
	assert.NoError(t, err)
	assert.Equal(t, "Found 2 users: alice and bob", val)

	mm.AssertExpectations(t)
}

func TestNodeAI_MaxIterations(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// Mock: LLM keeps calling tools forever (should stop at 3 iterations)
	toolCallResp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				ToolCalls: []llms.ToolCall{
					{
						ID: "call_loop",
						FunctionCall: &llms.FunctionCall{
							Name:      "get_variable",
							Arguments: `{"key": "counter"}`,
						},
					},
				},
			},
		},
	}

	// Allow up to 3 calls
	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(toolCallResp, nil).Times(3)

	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID: providerNode,
	}

	n := New(nodeID, "AI_NODE", "Loop forever", 3, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"counter": 0,
		},
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	// Should complete without error (just stops after 3 iterations)
	assert.NoError(t, res.Err)

	mm.AssertNumberOfCalls(t, "GenerateContent", 3)
}

func TestNodeAI_MultipleToolCalls(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// Mock: LLM calls two tools at once
	resp1 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				ToolCalls: []llms.ToolCall{
					{
						ID: "call_1",
						FunctionCall: &llms.FunctionCall{
							Name:      "get_variable",
							Arguments: `{"key": "var_a"}`,
						},
					},
					{
						ID: "call_2",
						FunctionCall: &llms.FunctionCall{
							Name:      "get_variable",
							Arguments: `{"key": "var_b"}`,
						},
					},
				},
			},
		},
	}

	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason: "stop",
				Content:    "Got both values: A and B",
			},
		},
	}

	// First call returns multiple tool calls
	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		return len(msgs) == 1
	}), mock.Anything).Return(resp1, nil).Once()

	// Second call (after tool responses) returns final answer
	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		return len(msgs) > 1
	}), mock.Anything).Return(resp2, nil).Once()

	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID: providerNode,
	}

	n := New(nodeID, "AI_NODE", "Get both vars", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"var_a": "A",
			"var_b": "B",
		},
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	val, err := node.ReadNodeVar(req, "AI_NODE", "text")
	assert.NoError(t, err)
	assert.Equal(t, "Got both values: A and B", val)

	mm.AssertExpectations(t)
}

func TestNodeAI_ToolExecutionErrorFeedback(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// Mock: LLM calls a tool that doesn't exist
	resp1 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				ToolCalls: []llms.ToolCall{
					{
						ID: "call_bad",
						FunctionCall: &llms.FunctionCall{
							Name:      "nonexistent_tool",
							Arguments: `{}`,
						},
					},
				},
			},
		},
	}

	// Mock: LLM sees the error and decides to stop with an explanation
	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason: "stop",
				Content:    "The tool failed, so I am stopping.",
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		return len(msgs) == 1
	}), mock.Anything).Return(resp1, nil)

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		return len(msgs) > 1
	}), mock.Anything).Return(resp2, nil)

	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID: providerNode,
	}

	n := New(nodeID, "AI_NODE", "Call bad tool", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	val, _ := node.ReadNodeVar(req, "AI_NODE", "text")
	assert.Equal(t, "The tool failed, so I am stopping.", val)
}

func TestNodeAI_LLMError(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// Mock: LLM returns an error
	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(
		(*llms.ContentResponse)(nil),
		errors.New("API rate limit exceeded"),
	)

	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID: providerNode,
	}

	n := New(nodeID, "AI_NODE", "hello", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "agent error")
	assert.Contains(t, res.Err.Error(), "API rate limit exceeded")
}

// namedMockNode wraps MockNode to provide a custom name
type namedMockNode struct {
	*mocknode.MockNode
	name  string
	OnRun func()
}

func (n *namedMockNode) GetName() string {
	return n.name
}

func (n *namedMockNode) RunSync(ctx context.Context, req *node.FlowNodeRequest) node.FlowNodeResult {
	if n.OnRun != nil {
		n.OnRun()
	}
	return n.MockNode.RunSync(ctx, req)
}

// Tests for n8n-style Model and Memory node integration

func TestNodeAI_WithConnectedModelNode(t *testing.T) {
	ctx := context.Background()
	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// AI Provider node provides configuration
	temp := float32(0.3)
	maxTokens := int32(1024)
	providerNode := &naiprovider.NodeAiProvider{
		FlowNodeID:   providerNodeID,
		Name:         "OpenAI Model",
		CredentialID: &credentialID,
		Model:        mflow.AiModelGpt52Pro,
		CustomModel:  "",
		Temperature:  &temp,
		MaxTokens:    &maxTokens,
	}

	// Setup edge map: AI Provider node connects to AI node via HandleAiProvider
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID: providerNode,
	}

	// Mock LLM response
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason: "stop",
				Content:    "Hello from GPT-5.2 Pro!",
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	// AI node requires AI Provider node (no internal model config)
	n := New(aiNodeID, "AI_NODE", "Say hello", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	val, err := node.ReadNodeVar(req, "AI_NODE", "text")
	assert.NoError(t, err)
	assert.Equal(t, "Hello from GPT-5.2 Pro!", val)

	mm.AssertExpectations(t)
}

func TestNodeAI_WithConnectedMemoryNode(t *testing.T) {
	ctx := context.Background()
	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()
	memoryNodeID := idwrap.NewNow()

	mm := new(mockModel)

	// AI Provider node
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	// Memory node provides conversation history
	memoryNode := nmemory.New(memoryNodeID, "Conversation Memory", mflow.AiMemoryTypeWindowBuffer, 10)
	// Pre-populate memory with previous messages
	memoryNode.AddMessage("user", "Hi, my name is Alice")
	memoryNode.AddMessage("assistant", "Hello Alice! Nice to meet you.")

	// Setup edge map: AI Provider node and Memory node connect to AI node
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider:  []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiMemory: []idwrap.IDWrap{memoryNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID:  providerNode,
		memoryNodeID: memoryNode,
	}

	// Mock LLM response - expect messages to include history
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason: "stop",
				Content:    "Of course I remember you, Alice!",
			},
		},
	}

	// Verify that the messages include the history from memory
	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		// Should have 3 messages: 2 from history + 1 current prompt
		return len(msgs) == 3
	}), mock.Anything).Return(resp, nil)

	n := New(aiNodeID, "AI_NODE", "Do you remember my name?", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	val, err := node.ReadNodeVar(req, "AI_NODE", "text")
	assert.NoError(t, err)
	assert.Equal(t, "Of course I remember you, Alice!", val)

	// Verify memory was updated with the new exchange
	msgs := memoryNode.GetMessages()
	assert.Len(t, msgs, 4) // 2 original + 1 user + 1 assistant
	assert.Equal(t, "Do you remember my name?", msgs[2].Content)
	assert.Equal(t, "Of course I remember you, Alice!", msgs[3].Content)

	mm.AssertExpectations(t)
}

func TestNodeAI_WithBothModelAndMemory(t *testing.T) {
	ctx := context.Background()
	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	memoryNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// AI Provider node configuration
	temp := float32(0.5)
	providerNode := &naiprovider.NodeAiProvider{
		FlowNodeID:   providerNodeID,
		Name:         "Claude Model",
		CredentialID: &credentialID,
		Model:        mflow.AiModelClaudeOpus45,
		Temperature:  &temp,
	}

	// Memory node with history
	memoryNode := nmemory.New(memoryNodeID, "Memory", mflow.AiMemoryTypeWindowBuffer, 5)
	memoryNode.AddMessage("user", "Previous question")
	memoryNode.AddMessage("assistant", "Previous answer")

	// Setup edges
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider:  []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiMemory: []idwrap.IDWrap{memoryNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID:  providerNode,
		memoryNodeID: memoryNode,
	}

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason: "stop",
				Content:    "Response with history and custom model",
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		// Should have 3 messages: 2 from history + 1 current
		return len(msgs) == 3
	}), mock.Anything).Return(resp, nil)

	// AI node requires AI Provider node
	n := New(aiNodeID, "AI_NODE", "Current question", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// Verify memory updated
	msgs := memoryNode.GetMessages()
	assert.Len(t, msgs, 4)

	mm.AssertExpectations(t)
}

func TestNodeAI_MemoryWindowEnforcement(t *testing.T) {
	ctx := context.Background()
	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()
	memoryNodeID := idwrap.NewNow()

	mm := new(mockModel)

	// AI Provider node
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)

	// Memory node with small window size
	memoryNode := nmemory.New(memoryNodeID, "Memory", mflow.AiMemoryTypeWindowBuffer, 3)
	// Add 3 messages
	memoryNode.AddMessage("user", "Message 1")
	memoryNode.AddMessage("assistant", "Response 1")
	memoryNode.AddMessage("user", "Message 2")

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider:  []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiMemory: []idwrap.IDWrap{memoryNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		providerNodeID:  providerNode,
		memoryNodeID: memoryNode,
	}

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason: "stop",
				Content:    "Response 2",
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	n := New(aiNodeID, "AI_NODE", "Current message", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// Window is 3, so after adding 2 more messages (user + assistant),
	// the oldest messages should be evicted
	msgs := memoryNode.GetMessages()
	assert.Len(t, msgs, 3) // Window size enforced
	// Oldest message should be evicted
	assert.Equal(t, "Message 2", msgs[0].Content)
	assert.Equal(t, "Current message", msgs[1].Content)
	assert.Equal(t, "Response 2", msgs[2].Content)

	mm.AssertExpectations(t)
}
