//go:build ai_integration

package nai

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"
)

// mockCredentialService implements scredential.ICredentialService
type mockCredentialService struct {
	mock.Mock
}

func (m *mockCredentialService) GetCredential(ctx context.Context, id idwrap.IDWrap) (*mcredential.Credential, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mcredential.Credential), args.Error(1)
}

func (m *mockCredentialService) GetCredentialOpenAI(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialOpenAI, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*mcredential.CredentialOpenAI), args.Error(1)
}

func (m *mockCredentialService) GetCredentialGemini(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialGemini, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*mcredential.CredentialGemini), args.Error(1)
}

func (m *mockCredentialService) GetCredentialAnthropic(ctx context.Context, id idwrap.IDWrap) (*mcredential.CredentialAnthropic, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*mcredential.CredentialAnthropic), args.Error(1)
}

func TestNodeAI_LiveCustom_Generic(t *testing.T) {
	// This generic test verifies that the "Custom Model" feature works with ANY provider
	// that is compatible with OpenAI or Anthropic SDKs, using standard env vars.

	// Priority: Anthropic -> OpenAI -> Skip
	var apiKey, baseUrl, modelName string
	var credKind mcredential.CredentialKind

	if os.Getenv("ANTHROPIC_BASE_URL") != "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_AUTH_TOKEN")
		}
		baseUrl = os.Getenv("ANTHROPIC_BASE_URL")
		modelName = os.Getenv("ANTHROPIC_MODEL")
		credKind = mcredential.CREDENTIAL_KIND_ANTHROPIC
	} else if os.Getenv("OPENAI_BASE_URL") != "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
		baseUrl = os.Getenv("OPENAI_BASE_URL")
		modelName = os.Getenv("OPENAI_MODEL")
		credKind = mcredential.CREDENTIAL_KIND_OPENAI
	} else {
		// Fallback/Legacy logic for direct tests
		apiKey = os.Getenv("MINIMAX_API_KEY")
		if apiKey != "" {
			baseUrl = "https://api.minimax.io/v1"
			modelName = "MiniMax-M2.1"
			credKind = mcredential.CREDENTIAL_KIND_OPENAI
		}
	}

	if apiKey == "" || baseUrl == "" {
		t.Skip("Skipping custom model test: API Key or Base URL not set (check ANTHROPIC_BASE_URL or OPENAI_BASE_URL)")
	}
	if modelName == "" {
		modelName = "custom-model" // Default fallback
	}

	ctx := context.Background()
	credID := idwrap.NewNow()
	
	// 1. Setup Mock Credential Service
	mockService := new(mockCredentialService)
	
	mockService.On("GetCredential", mock.Anything, credID).Return(&mcredential.Credential{
		ID:   credID,
		Name: "CustomProvider",
		Kind: credKind,
	}, nil)

	// Mock specific credential details based on kind
	if credKind == mcredential.CREDENTIAL_KIND_ANTHROPIC {
		mockService.On("GetCredentialAnthropic", mock.Anything, credID).Return(&mcredential.CredentialAnthropic{
			CredentialID: credID,
			ApiKey:       apiKey,
			BaseUrl:      &baseUrl,
		}, nil)
	} else if credKind == mcredential.CREDENTIAL_KIND_OPENAI {
		mockService.On("GetCredentialOpenAI", mock.Anything, credID).Return(&mcredential.CredentialOpenAI{
			CredentialID: credID,
			Token:        apiKey,
			BaseUrl:      &baseUrl,
		}, nil)
	}

	// 2. Create Factory
	factory := scredential.NewLLMProviderFactory(mockService)

	// 3. Create Node with Custom Model
	n := New(idwrap.NewNow(), "CUSTOM_AI_NODE", mflow.AiModelCustom, modelName, credID, "Say 'Hello' and state your model name.", 0, factory)

	req := &node.FlowNodeRequest{
		VarMap:        make(map[string]any),
		ReadWriteLock: &sync.RWMutex{},
	}

	// 4. Run
	t.Logf("Running AI Node with Custom Model: %s (Provider: %v) at %s", modelName, credKind, baseUrl)
	res := n.RunSync(ctx, req)
	
	if res.Err != nil {
		t.Fatalf("Node execution failed: %v", res.Err)
	}

	val, err := node.ReadNodeVar(req, "CUSTOM_AI_NODE", "text")
	assert.NoError(t, err)
	t.Logf("Response: %v", val)
	assert.NotEmpty(t, val)
}