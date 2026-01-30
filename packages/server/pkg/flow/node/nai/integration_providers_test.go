//go:build ai_integration

package nai

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func TestNodeAI_LiveOpenAI(t *testing.T) {
	apiKey := RequireEnv(t, "OPENAI_API_KEY")
	baseUrl := os.Getenv("OPENAI_BASE_URL")
	modelName := os.Getenv("OPENAI_MODEL")

	ctx := context.Background()

	opts := []openai.Option{
		openai.WithToken(apiKey),
	}
	if baseUrl != "" {
		opts = append(opts, openai.WithBaseURL(baseUrl))
	}
	if modelName != "" {
		opts = append(opts, openai.WithModel(modelName))
	}

	llm, err := openai.New(opts...)
	assert.NoError(t, err)

	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()

	n := New(aiNodeID, "OPENAI_AGENT", "What is the value of 'test_var'? Use get_variable.", 5, nil)

	providerNode := CreateTestAiProviderNode(providerNodeID)
	providerNode.LLM = llm // Set LLM on provider, not AI node

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       n, // AI node must be in nodeMap for provider to find via reverse lookup
		providerNodeID: providerNode,
	}

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"test_var": "Hello from DevTools!",
		},
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	val, err := node.ReadNodeVar(req, "OPENAI_AGENT", "text")
	assert.NoError(t, err)
	t.Logf("OpenAI Response: %v", val)
	assert.Contains(t, val, "Hello from DevTools!")
}

func TestNodeAI_LiveGemini(t *testing.T) {
	apiKey := RequireEnv(t, "GEMINI_API_KEY")
	ctx := context.Background()

	llm, err := googleai.New(ctx,
		googleai.WithAPIKey(apiKey),
		googleai.WithDefaultModel("gemini-2.5-flash"), // Use Gemini 2.5 Flash (stable)
	)
	assert.NoError(t, err)

	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()

	n := New(aiNodeID, "GEMINI_AGENT", "Greet the user {{user_name}}. Then tell me what is in 'secret_code' variable.", 5, nil)

	providerNode := CreateTestAiProviderNode(providerNodeID)
	providerNode.LLM = llm // Set LLM on provider, not AI node

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       n, // AI node must be in nodeMap for provider to find via reverse lookup
		providerNodeID: providerNode,
	}

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"user_name":   "Dev",
			"secret_code": "INTEGRATION_SUCCESS",
		},
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	val, err := node.ReadNodeVar(req, "GEMINI_AGENT", "text")
	assert.NoError(t, err)
	t.Logf("Gemini Response: %v", val)
	assert.Contains(t, val, "Dev")
	assert.Contains(t, val, "INTEGRATION_SUCCESS")
}

func TestNodeAI_LiveAnthropic(t *testing.T) {
	apiKey := RequireEnv(t, "ANTHROPIC_API_KEY")
	ctx := context.Background()

	llm, err := anthropic.New(
		anthropic.WithToken(apiKey),
		anthropic.WithModel("claude-sonnet-4-20250514"), // Use Claude Sonnet 4
	)
	assert.NoError(t, err)

	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()

	n := New(aiNodeID, "ANTHROPIC_AGENT", "Say 'Claude is here'.", 5, nil)

	providerNode := CreateTestAiProviderNode(providerNodeID)
	providerNode.LLM = llm // Set LLM on provider, not AI node

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       n, // AI node must be in nodeMap for provider to find via reverse lookup
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

	val, err := node.ReadNodeVar(req, "ANTHROPIC_AGENT", "text")
	assert.NoError(t, err)
	t.Logf("Anthropic Response: %v", val)
	assert.Contains(t, val, "Claude")
}
