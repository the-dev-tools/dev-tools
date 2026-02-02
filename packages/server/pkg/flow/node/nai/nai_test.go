package nai_test

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
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/naiprovider"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node/nmemory"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/runner"
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
		Model:        mflow.AiModelGpt52,
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
				GenerationInfo: map[string]any{"PromptTokens": 10, "CompletionTokens": 5},
			},
		},
	}

	// Mock 2: Assistant provides final answer
	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "final answer",
				GenerationInfo: map[string]any{"PromptTokens": 20, "CompletionTokens": 10},
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

	// Create AI Provider node with mock LLM
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)
	providerNode.LLM = mm

	// Create AI orchestrator node (needed for provider to find via reverse edge lookup)
	n := nai.New(nodeID, "AI_NODE", "hello {{user_name}}", 0, nil)

	// Setup edge map with AI Provider node
	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	// NodeMap must include the AI node (orchestrator) so provider can find it via reverse edge lookup
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		providerNodeID: providerNode,
	}

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
	n := nai.New(nodeID, "AI_NODE", "hello", 0, nil)

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

	n := nai.New(nodeID, "AI_NODE", "hello", 0, nil)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		providerNodeID: providerNode,
	}

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "requires LLM provider factory")
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

	// Create AI Provider node with mock LLM
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)
	providerNode.LLM = mm

	// Create AI orchestrator node
	n := nai.New(nodeID, "AI_NODE", "Get all users", 0, nil)

	// Setup edge map: AI node -> HTTP node via HandleAiTools, Model via HandleAiProvider
	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiTools:    []idwrap.IDWrap{httpNodeID},
		},
	}

	// Setup node map (must include AI node for provider reverse lookup)
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		httpNodeID:     customHttpNode,
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
				GenerationInfo: map[string]any{"PromptTokens": 10, "CompletionTokens": 5},
			},
		},
	}

	// Mock: LLM returns final answer after seeing tool result
	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "Found 2 users: alice and bob",
				GenerationInfo: map[string]any{"PromptTokens": 20, "CompletionTokens": 15},
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
				GenerationInfo: map[string]any{"PromptTokens": 10, "CompletionTokens": 5},
			},
		},
	}

	// Allow up to 3 calls
	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(toolCallResp, nil).Times(3)

	providerNode := createTestAiProviderNode(providerNodeID, credentialID)
	providerNode.LLM = mm

	n := nai.New(nodeID, "AI_NODE", "Loop forever", 3, nil)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		providerNodeID: providerNode,
	}

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
				GenerationInfo: map[string]any{"PromptTokens": 10, "CompletionTokens": 5},
			},
		},
	}

	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "Got both values: A and B",
				GenerationInfo: map[string]any{"PromptTokens": 20, "CompletionTokens": 10},
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
	providerNode.LLM = mm

	n := nai.New(nodeID, "AI_NODE", "Get both vars", 0, nil)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		providerNodeID: providerNode,
	}

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
				GenerationInfo: map[string]any{"PromptTokens": 10, "CompletionTokens": 5},
			},
		},
	}

	// Mock: LLM sees the error and decides to stop with an explanation
	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "The tool failed, so I am stopping.",
				GenerationInfo: map[string]any{"PromptTokens": 20, "CompletionTokens": 10},
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
	providerNode.LLM = mm

	n := nai.New(nodeID, "AI_NODE", "Call bad tool", 0, nil)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		providerNodeID: providerNode,
	}

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
	providerNode.LLM = mm

	n := nai.New(nodeID, "AI_NODE", "hello", 0, nil)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		providerNodeID: providerNode,
	}

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
		Temperature:  &temp,
		MaxTokens:    &maxTokens,
		LLM:          mm, // Mock LLM injected directly into provider
	}

	// Mock LLM response
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "Hello from GPT-5.2 Pro!",
				GenerationInfo: map[string]any{"PromptTokens": 10, "CompletionTokens": 8},
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	// AI node requires AI Provider node (no internal model config)
	n := nai.New(aiNodeID, "AI_NODE", "Say hello", 0, nil)

	// Setup edge map: AI Provider node connects to AI node via HandleAiProvider
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       n,
		providerNodeID: providerNode,
	}

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

	// AI Provider node with mock LLM
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)
	providerNode.LLM = mm

	// Memory node provides conversation history
	memoryNode := nmemory.New(memoryNodeID, "Conversation Memory", mflow.AiMemoryTypeWindowBuffer, 10)
	// Pre-populate memory with previous messages
	memoryNode.AddMessage("user", "Hi, my name is Alice")
	memoryNode.AddMessage("assistant", "Hello Alice! Nice to meet you.")

	// Mock LLM response - expect messages to include history
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "Of course I remember you, Alice!",
				GenerationInfo: map[string]any{"PromptTokens": 30, "CompletionTokens": 10},
			},
		},
	}

	// Verify that the messages include the history from memory
	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		// Should have 3 messages: 2 from history + 1 current prompt
		return len(msgs) == 3
	}), mock.Anything).Return(resp, nil)

	n := nai.New(aiNodeID, "AI_NODE", "Do you remember my name?", 0, nil)

	// Setup edge map: AI Provider node and Memory node connect to AI node
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiMemory:   []idwrap.IDWrap{memoryNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       n,
		providerNodeID: providerNode,
		memoryNodeID:   memoryNode,
	}

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

	// AI Provider node configuration with mock LLM
	temp := float32(0.5)
	providerNode := &naiprovider.NodeAiProvider{
		FlowNodeID:   providerNodeID,
		Name:         "Claude Model",
		CredentialID: &credentialID,
		Model:        mflow.AiModelClaudeOpus45,
		Temperature:  &temp,
		LLM:          mm,
	}

	// Memory node with history
	memoryNode := nmemory.New(memoryNodeID, "Memory", mflow.AiMemoryTypeWindowBuffer, 5)
	memoryNode.AddMessage("user", "Previous question")
	memoryNode.AddMessage("assistant", "Previous answer")

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "Response with history and custom model",
				GenerationInfo: map[string]any{"PromptTokens": 25, "CompletionTokens": 10},
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		// Should have 3 messages: 2 from history + 1 current
		return len(msgs) == 3
	}), mock.Anything).Return(resp, nil)

	// AI node requires AI Provider node
	n := nai.New(aiNodeID, "AI_NODE", "Current question", 0, nil)

	// Setup edges
	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiMemory:   []idwrap.IDWrap{memoryNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       n,
		providerNodeID: providerNode,
		memoryNodeID:   memoryNode,
	}

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

	// AI Provider node with mock LLM
	providerNode := createTestAiProviderNode(providerNodeID, credentialID)
	providerNode.LLM = mm

	// Memory node with small window size
	memoryNode := nmemory.New(memoryNodeID, "Memory", mflow.AiMemoryTypeWindowBuffer, 3)
	// Add 3 messages
	memoryNode.AddMessage("user", "Message 1")
	memoryNode.AddMessage("assistant", "Response 1")
	memoryNode.AddMessage("user", "Message 2")

	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "Response 2",
				GenerationInfo: map[string]any{"PromptTokens": 20, "CompletionTokens": 5},
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	n := nai.New(aiNodeID, "AI_NODE", "Current message", 0, nil)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
			mflow.HandleAiMemory:   []idwrap.IDWrap{memoryNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       n,
		providerNodeID: providerNode,
		memoryNodeID:   memoryNode,
	}

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

func TestNodeAI_EmitsIterationStatus(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// Single response without tool calls
	resp := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "Done",
				GenerationInfo: map[string]any{"PromptTokens": 10, "CompletionTokens": 5},
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)

	providerNode := createTestAiProviderNode(providerNodeID, credentialID)
	providerNode.LLM = mm

	n := nai.New(nodeID, "AI_NODE", "Test", 0, nil)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		providerNodeID: providerNode,
	}

	// Capture status updates
	var statuses []runner.FlowNodeStatus
	logPush := func(s runner.FlowNodeStatus) {
		statuses = append(statuses, s)
	}

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
		LogPushFunc:   logPush,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// Should have 4 events:
	// 1. NodeAI iteration RUNNING
	// 2. NodeAiProvider RUNNING
	// 3. NodeAiProvider SUCCESS (with metrics)
	// 4. NodeAI iteration SUCCESS
	assert.Len(t, statuses, 4)

	// NodeAI iteration RUNNING
	assert.Equal(t, mflow.NODE_STATE_RUNNING, statuses[0].State)
	assert.True(t, statuses[0].IterationEvent)
	assert.Equal(t, nodeID, statuses[0].NodeID)

	// NodeAiProvider RUNNING
	assert.Equal(t, mflow.NODE_STATE_RUNNING, statuses[1].State)
	assert.False(t, statuses[1].IterationEvent)
	assert.Equal(t, providerNodeID, statuses[1].NodeID)

	// NodeAiProvider SUCCESS (with output including metrics)
	assert.Equal(t, mflow.NODE_STATE_SUCCESS, statuses[2].State)
	assert.False(t, statuses[2].IterationEvent)
	assert.Equal(t, providerNodeID, statuses[2].NodeID)
	providerOutput, ok := statuses[2].OutputData.(map[string]any)
	assert.True(t, ok)
	assert.NotNil(t, providerOutput["metrics"])
	assert.Equal(t, "Done", providerOutput["text"])

	// NodeAI iteration SUCCESS
	assert.Equal(t, mflow.NODE_STATE_SUCCESS, statuses[3].State)
	assert.True(t, statuses[3].IterationEvent)
	assert.Equal(t, nodeID, statuses[3].NodeID)
}

func TestNodeAI_AggregatesMetrics(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()
	credentialID := idwrap.NewNow()

	mm := new(mockModel)

	// First response with tool call
	resp1 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				ToolCalls: []llms.ToolCall{
					{ID: "call_1", FunctionCall: &llms.FunctionCall{Name: "get_variable", Arguments: `{"key": "x"}`}},
				},
				GenerationInfo: map[string]any{"PromptTokens": 100, "CompletionTokens": 50},
			},
		},
	}

	// Second response - final
	resp2 := &llms.ContentResponse{
		Choices: []*llms.ContentChoice{
			{
				StopReason:     "stop",
				Content:        "Done",
				GenerationInfo: map[string]any{"PromptTokens": 150, "CompletionTokens": 75},
			},
		},
	}

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		return len(msgs) == 1
	}), mock.Anything).Return(resp1, nil).Once()

	mm.On("GenerateContent", mock.Anything, mock.MatchedBy(func(msgs []llms.MessageContent) bool {
		return len(msgs) > 1
	}), mock.Anything).Return(resp2, nil).Once()

	providerNode := createTestAiProviderNode(providerNodeID, credentialID)
	providerNode.LLM = mm

	n := nai.New(nodeID, "AI_NODE", "Test", 0, nil)

	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		nodeID:         n,
		providerNodeID: providerNode,
	}

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"x": "value",
		},
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// Verify aggregated metrics
	totalMetrics, err := node.ReadNodeVar(req, "AI_NODE", "total_metrics")
	assert.NoError(t, err)

	metricsMap, ok := totalMetrics.(mflow.AITotalMetrics)
	assert.True(t, ok)
	assert.Equal(t, int32(250), metricsMap.PromptTokens)     // 100 + 150
	assert.Equal(t, int32(125), metricsMap.CompletionTokens) // 50 + 75
	assert.Equal(t, int32(375), metricsMap.TotalTokens)      // 250 + 125
	assert.Equal(t, int32(2), metricsMap.LLMCalls)
	assert.Equal(t, int32(1), metricsMap.ToolCalls)

	mm.AssertExpectations(t)
}

func TestNodeAI_GetOutputVariables(t *testing.T) {
	n := &nai.NodeAI{}
	vars := n.GetOutputVariables()

	assert.Contains(t, vars, "text")
	assert.Contains(t, vars, "total_metrics")
	// Iteration output
	assert.Contains(t, vars, "iteration")
}

func TestNodeAI_GetRequiredVariables(t *testing.T) {
	n := &nai.NodeAI{
		Prompt: "Hello {{name}}, your score is {{score}}",
	}

	vars := n.GetRequiredVariables()
	assert.Contains(t, vars, "name")
	assert.Contains(t, vars, "score")
}
