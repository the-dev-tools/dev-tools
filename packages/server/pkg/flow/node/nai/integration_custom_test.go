//go:build ai_integration

package nai

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func TestNodeAI_LiveCustom_Generic(t *testing.T) {
	// This generic test verifies that custom model configurations work with ANY provider
	// that is compatible with OpenAI or Anthropic SDKs, using standard env vars.
	RequireIntegration(t)

	// Priority: Anthropic with custom base -> OpenAI with custom base -> Skip
	var apiKey, baseUrl, modelName string
	var providerType string

	if os.Getenv("ANTHROPIC_BASE_URL") != "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_AUTH_TOKEN")
		}
		baseUrl = os.Getenv("ANTHROPIC_BASE_URL")
		modelName = os.Getenv("ANTHROPIC_MODEL")
		providerType = "anthropic"
	} else if os.Getenv("OPENAI_BASE_URL") != "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		baseUrl = os.Getenv("OPENAI_BASE_URL")
		modelName = os.Getenv("OPENAI_MODEL")
		providerType = "openai"
	} else {
		// Fallback/Legacy logic for direct tests
		apiKey = os.Getenv("MINIMAX_API_KEY")
		if apiKey != "" {
			baseUrl = "https://api.minimax.io/v1"
			modelName = "MiniMax-M2.1"
			providerType = "openai"
		}
	}

	if apiKey == "" || baseUrl == "" {
		t.Skip("Skipping custom model test: API Key or Base URL not set (check ANTHROPIC_BASE_URL or OPENAI_BASE_URL)")
	}
	if modelName == "" {
		modelName = "custom-model" // Default fallback
	}

	ctx := context.Background()

	aiNodeID := idwrap.NewNow()
	providerNodeID := idwrap.NewNow()

	// Create AI nodes
	n := New(aiNodeID, "CUSTOM_AI_NODE", "Say 'Hello' and state your model name.", 5, nil)
	providerNode := CreateTestAiProviderNode(providerNodeID)

	edgeMap := mflow.EdgesMap{
		aiNodeID: {
			mflow.HandleAiProvider: []idwrap.IDWrap{providerNodeID},
		},
	}

	nodeMap := map[idwrap.IDWrap]node.FlowNode{
		aiNodeID:       n, // AI node must be in nodeMap for provider to find via reverse lookup
		providerNodeID: providerNode,
	}

	if providerType == "anthropic" {
		opts := []anthropic.Option{
			anthropic.WithToken(apiKey),
			anthropic.WithBaseURL(baseUrl),
		}
		if modelName != "" {
			opts = append(opts, anthropic.WithModel(modelName))
		}
		llm, err := anthropic.New(opts...)
		if err != nil {
			t.Fatalf("Failed to create Anthropic client: %v", err)
		}
		providerNode.LLM = llm // Set LLM on provider, not AI node
	} else {
		// OpenAI-compatible provider
		opts := []openai.Option{
			openai.WithToken(apiKey),
			openai.WithBaseURL(baseUrl),
		}
		if modelName != "" {
			opts = append(opts, openai.WithModel(modelName))
		}
		llm, err := openai.New(opts...)
		if err != nil {
			t.Fatalf("Failed to create OpenAI client: %v", err)
		}
		providerNode.LLM = llm // Set LLM on provider, not AI node
	}

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
		EdgeSourceMap: edgeMap,
		NodeMap:       nodeMap,
	}

	t.Logf("Running AI Node with Custom Model: %s (Provider: %s) at %s", modelName, providerType, baseUrl)
	res := n.RunSync(ctx, req)

	if res.Err != nil {
		t.Fatalf("Node execution failed: %v", res.Err)
	}

	val, err := node.ReadNodeVar(req, "CUSTOM_AI_NODE", "text")
	assert.NoError(t, err)
	t.Logf("Response: %v", val)
	assert.NotEmpty(t, val)
}
