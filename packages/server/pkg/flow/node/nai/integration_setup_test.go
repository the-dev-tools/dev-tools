//go:build ai_integration

package nai

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
)

// RequireIntegration checks if the integration test flag is set.
// It skips the test if RUN_AI_INTEGRATION_TESTS is not "true".
func RequireIntegration(t *testing.T) {
	if os.Getenv("RUN_AI_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping AI integration test: RUN_AI_INTEGRATION_TESTS != true")
	}
}

// RequireEnv checks for a specific environment variable and returns it.
// Skips the test if the variable is missing.
func RequireEnv(t *testing.T, key string) string {
	RequireIntegration(t)
	val := os.Getenv(key)
	if val == "" {
		t.Skipf("Skipping test: %s not set", key)
	}
	return val
}

// SetupGenericIntegrationTest creates an LLM client based on available environment variables.
// It checks providers in this order: OpenAI -> Anthropic -> Gemini.
func SetupGenericIntegrationTest(t *testing.T) llms.Model {
	RequireIntegration(t)
	ctx := context.Background()

	// 1. OpenAI (or Compatible like MiniMax)
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		opts := []openai.Option{openai.WithToken(apiKey)}
		if base := os.Getenv("OPENAI_BASE_URL"); base != "" {
			opts = append(opts, openai.WithBaseURL(base))
		}
		if model := os.Getenv("OPENAI_MODEL"); model != "" {
			opts = append(opts, openai.WithModel(model))
		}
		llm, err := openai.New(opts...)
		assert.NoError(t, err)
		return llm
	}

	// 2. Anthropic
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		opts := []anthropic.Option{anthropic.WithToken(apiKey)}
		if base := os.Getenv("ANTHROPIC_BASE_URL"); base != "" {
			opts = append(opts, anthropic.WithBaseURL(base))
		}
		if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
			opts = append(opts, anthropic.WithModel(model))
		}
		llm, err := anthropic.New(opts...)
		assert.NoError(t, err)
		return llm
	}

	// 3. Gemini
	if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
		opts := []googleai.Option{googleai.WithAPIKey(apiKey)}
		llm, err := googleai.New(ctx, opts...)
		assert.NoError(t, err)
		return llm
	}

	t.Skip("No valid API keys found (OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY)")
	return nil
}
