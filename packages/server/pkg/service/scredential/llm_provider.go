package scredential

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// WorkspaceCredentials holds API keys for all providers at workspace level
type WorkspaceCredentials struct {
	OpenAIKey    string
	AnthropicKey string
	GeminiKey    string
}

type LLMProviderFactory struct {
	service     *CredentialService
	credentials *WorkspaceCredentials
}

func NewLLMProviderFactory(service CredentialService) *LLMProviderFactory {
	return &LLMProviderFactory{
		service: &service,
	}
}

// NewLLMProviderFactoryWithCredentials creates a factory with workspace-level credentials
func NewLLMProviderFactoryWithCredentials(creds *WorkspaceCredentials) *LLMProviderFactory {
	return &LLMProviderFactory{
		credentials: creds,
	}
}

// CreateModelWithCredential creates an LLM client using the specified model and credential
func (f *LLMProviderFactory) CreateModelWithCredential(ctx context.Context, aiModel mflow.AiModel, credentialID idwrap.IDWrap) (llms.Model, error) {
	if f.service == nil {
		return nil, fmt.Errorf("credential service not configured")
	}

	cred, err := f.service.GetCredential(ctx, credentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	modelStr := aiModel.ModelString()
	provider := aiModel.Provider()

	// Validate that credential matches the model's provider
	credProvider := ""
	switch cred.Kind {
	case mcredential.CREDENTIAL_KIND_OPENAI:
		credProvider = "openai"
	case mcredential.CREDENTIAL_KIND_GEMINI:
		credProvider = "google"
	case mcredential.CREDENTIAL_KIND_ANTHROPIC:
		credProvider = "anthropic"
	}

	if credProvider != provider {
		return nil, fmt.Errorf("credential type (%s) does not match model provider (%s)", credProvider, provider)
	}

	switch cred.Kind {
	case mcredential.CREDENTIAL_KIND_OPENAI:
		openaiCred, err := f.service.GetCredentialOpenAI(ctx, credentialID)
		if err != nil {
			return nil, fmt.Errorf("failed to get openai details: %w", err)
		}
		opts := []openai.Option{
			openai.WithToken(openaiCred.Token),
			openai.WithModel(modelStr),
		}
		if openaiCred.BaseUrl != nil {
			opts = append(opts, openai.WithBaseURL(*openaiCred.BaseUrl))
		}
		return openai.New(opts...)

	case mcredential.CREDENTIAL_KIND_GEMINI:
		geminiCred, err := f.service.GetCredentialGemini(ctx, credentialID)
		if err != nil {
			return nil, fmt.Errorf("failed to get gemini details: %w", err)
		}
		opts := []googleai.Option{
			googleai.WithAPIKey(geminiCred.ApiKey),
			googleai.WithDefaultModel(modelStr),
		}
		return googleai.New(ctx, opts...)

	case mcredential.CREDENTIAL_KIND_ANTHROPIC:
		anthropicCred, err := f.service.GetCredentialAnthropic(ctx, credentialID)
		if err != nil {
			return nil, fmt.Errorf("failed to get anthropic details: %w", err)
		}
		opts := []anthropic.Option{
			anthropic.WithToken(anthropicCred.ApiKey),
			anthropic.WithModel(modelStr),
		}
		if anthropicCred.BaseUrl != nil {
			opts = append(opts, anthropic.WithBaseURL(*anthropicCred.BaseUrl))
		}
		return anthropic.New(opts...)

	default:
		return nil, fmt.Errorf("unsupported credential kind: %v", cred.Kind)
	}
}

func (f *LLMProviderFactory) CreateModel(ctx context.Context, credentialID idwrap.IDWrap) (llms.Model, error) {
	cred, err := f.service.GetCredential(ctx, credentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	switch cred.Kind {
	case mcredential.CREDENTIAL_KIND_OPENAI:
		openaiCred, err := f.service.GetCredentialOpenAI(ctx, credentialID)
		if err != nil {
			return nil, fmt.Errorf("failed to get openai details: %w", err)
		}

		opts := []openai.Option{
			openai.WithToken(openaiCred.Token),
		}
		if openaiCred.BaseUrl != nil {
			opts = append(opts, openai.WithBaseURL(*openaiCred.BaseUrl))
		}

		return openai.New(opts...)
	case mcredential.CREDENTIAL_KIND_GEMINI:
		geminiCred, err := f.service.GetCredentialGemini(ctx, credentialID)
		if err != nil {
			return nil, fmt.Errorf("failed to get gemini details: %w", err)
		}

		opts := []googleai.Option{
			googleai.WithAPIKey(geminiCred.ApiKey),
		}
		if geminiCred.BaseUrl != nil {
			// Note: LangChain Go googleai might not support custom base URL yet, 
			// checking common option names.
			// opts = append(opts, googleai.WithBaseURL(*geminiCred.BaseUrl))
		}

		return googleai.New(ctx, opts...)
	case mcredential.CREDENTIAL_KIND_ANTHROPIC:
		anthropicCred, err := f.service.GetCredentialAnthropic(ctx, credentialID)
		if err != nil {
			return nil, fmt.Errorf("failed to get anthropic details: %w", err)
		}

		opts := []anthropic.Option{
			anthropic.WithToken(anthropicCred.ApiKey),
		}
		if anthropicCred.BaseUrl != nil {
			opts = append(opts, anthropic.WithBaseURL(*anthropicCred.BaseUrl))
		}

		return anthropic.New(opts...)
	default:
		return nil, fmt.Errorf("unsupported credential kind: %v", cred.Kind)
	}
}
