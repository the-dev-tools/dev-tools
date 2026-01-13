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

	n := New(idwrap.NewNow(), "OPENAI_AGENT", mflow.AiModelGpt52Pro, "", idwrap.IDWrap{}, "What is the value of 'test_var'? Use get_variable.", 0, nil)
	n.LLM = llm

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"test_var": "Hello from DevTools!",
		},
		ReadWriteLock: &sync.RWMutex{},
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

	llm, err := googleai.New(ctx, googleai.WithAPIKey(apiKey))
	assert.NoError(t, err)

	n := New(idwrap.NewNow(), "GEMINI_AGENT", mflow.AiModelGemini3Pro, "", idwrap.IDWrap{}, "Greet the user {{user_name}}. Then tell me what is in 'secret_code' variable.", 0, nil)
	n.LLM = llm

	req := &node.FlowNodeRequest{
		VarMap: map[string]any{
			"user_name":   "Dev",
			"secret_code": "INTEGRATION_SUCCESS",
		},
		ReadWriteLock: &sync.RWMutex{},
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

	llm, err := anthropic.New(anthropic.WithToken(apiKey))
	assert.NoError(t, err)

	n := New(idwrap.NewNow(), "ANTHROPIC_AGENT", mflow.AiModelClaudeOpus45, "", idwrap.IDWrap{}, "Say 'Claude is here'.", 0, nil)
	n.LLM = llm

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	res := n.RunSync(ctx, req)
	assert.NoError(t, res.Err)

	val, err := node.ReadNodeVar(req, "ANTHROPIC_AGENT", "text")
	assert.NoError(t, err)
	t.Logf("Anthropic Response: %v", val)
	assert.Contains(t, val, "Claude")
}
