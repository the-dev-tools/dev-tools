//go:build ai_integration

package nai

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func checkLiveTest(t *testing.T, keyName string) string {
	if os.Getenv("RUN_AI_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping live AI integration test (RUN_AI_INTEGRATION_TESTS != true)")
	}
	key := os.Getenv(keyName)
	if key == "" {
		t.Skipf("Skipping live test: %s not set", keyName)
	}
	return key
}

func TestNodeAI_LiveOpenAI(t *testing.T) {
	apiKey := checkLiveTest(t, "OPENAI_API_KEY")
	ctx := context.Background()

	llm, err := openai.New(openai.WithToken(apiKey))
	assert.NoError(t, err)

	n := New(idwrap.NewNow(), "OPENAI_AGENT", "What is the value of 'test_var'? Use get_variable.", nil, nil)
	n.Model = llm

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
	apiKey := checkLiveTest(t, "GEMINI_API_KEY")
	ctx := context.Background()

	llm, err := googleai.New(ctx, googleai.WithAPIKey(apiKey))
	assert.NoError(t, err)

	n := New(idwrap.NewNow(), "GEMINI_AGENT", "Greet the user {{user_name}}. Then tell me what is in 'secret_code' variable.", nil, nil)
	n.Model = llm

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
	apiKey := checkLiveTest(t, "ANTHROPIC_API_KEY")
	ctx := context.Background()

	llm, err := anthropic.New(anthropic.WithToken(apiKey))
	assert.NoError(t, err)

	n := New(idwrap.NewNow(), "ANTHROPIC_AGENT", "Say 'Claude is here'.", nil, nil)
	n.Model = llm

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
