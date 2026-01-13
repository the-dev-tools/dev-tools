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

func TestNodeAIRun(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	
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

	n := New(nodeID, "AI_NODE", mflow.AiModelGpt52Instant, "", idwrap.IDWrap{}, "hello {{user_name}}", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"my_var":    "secret_data",
			"user_name": "Alice",
		},
		ReadWriteLock: &sync.RWMutex{},
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	// Verify variable write
	val, err := node.ReadNodeVar(req, "AI_NODE", "text")
	assert.NoError(t, err)
	assert.Equal(t, "final answer", val)

	mm.AssertExpectations(t)
}

func TestNodeAI_MissingProviderFactory(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()

	// No LLM override, no ProviderFactory
	n := New(nodeID, "AI_NODE", mflow.AiModelGpt52Instant, "", idwrap.IDWrap{}, "hello", 0, nil)

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	res := n.RunSync(ctx, req)
	assert.Error(t, res.Err)
	assert.Contains(t, res.Err.Error(), "LLM provider factory is missing")
}

func TestNodeAI_WithConnectedTools(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()
	httpNodeID := idwrap.NewNow()

	mm := new(mockModel)

	// Mock HTTP node that writes output to VarMap
	httpNode := &mocknode.MockNode{
		ID: httpNodeID,
		OnRun: func() {},
	}
	// Override GetName to return a specific name
	httpNodeName := "GetUsers"

	// Create a custom mock that returns proper name
	customHttpNode := &namedMockNode{
		MockNode: httpNode,
		name:     httpNodeName,
	}

	// Setup edge map: AI node -> HTTP node via HandleAiTools
	edgeMap := mflow.EdgesMap{
		nodeID: {
			mflow.HandleAiTools: []idwrap.IDWrap{httpNodeID},
		},
	}

	// Setup node map
	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		httpNodeID: customHttpNode,
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

	n := New(nodeID, "AI_NODE", mflow.AiModelClaudeOpus45, "", idwrap.IDWrap{}, "Get all users", 0, nil)
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

	mm := new(mockModel)

	// Mock: LLM keeps calling tools forever (should stop at 5 iterations)
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

	n := New(nodeID, "AI_NODE", mflow.AiModelGemini3Flash, "", idwrap.IDWrap{}, "Loop forever", 3, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"counter": 0,
		},
		ReadWriteLock: &sync.RWMutex{},
	}

	res := n.RunSync(ctx, req)
	// Should complete without error (just stops after 3 iterations)
	assert.NoError(t, res.Err)

	mm.AssertNumberOfCalls(t, "GenerateContent", 3)
}

func TestNodeAI_MultipleToolCalls(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()

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

	n := New(nodeID, "AI_NODE", mflow.AiModelO3, "", idwrap.IDWrap{}, "Get both vars", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"var_a": "A",
			"var_b": "B",
		},
		ReadWriteLock: &sync.RWMutex{},
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

	n := New(nodeID, "AI_NODE", mflow.AiModelGpt52Pro, "", idwrap.IDWrap{}, "Call bad tool", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	val, _ := node.ReadNodeVar(req, "AI_NODE", "text")
	assert.Equal(t, "The tool failed, so I am stopping.", val)
}

func TestNodeAI_LLMError(t *testing.T) {
	ctx := context.Background()
	nodeID := idwrap.NewNow()

	mm := new(mockModel)

	// Mock: LLM returns an error
	mm.On("GenerateContent", mock.Anything, mock.Anything, mock.Anything).Return(
		(*llms.ContentResponse)(nil),
		errors.New("API rate limit exceeded"),
	)

	n := New(nodeID, "AI_NODE", mflow.AiModelClaudeSonnet45, "", idwrap.IDWrap{}, "hello", 0, nil)
	n.LLM = mm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
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
