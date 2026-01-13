package scredential

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
)

func TestLLMProviderFactory(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create in-memory db: %v", err)
	}
	defer cleanup()

	queries := gen.New(db)
	service := NewCredentialService(queries) // Uses default logger
	factory := NewLLMProviderFactory(service)

	workspaceID := idwrap.NewNow()
	// Seed workspace
	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "WS",
	})
	if err != nil {
		t.Fatalf("failed to seed workspace: %v", err)
	}

	t.Run("Create OpenAI Model", func(t *testing.T) {
		id := idwrap.NewNow()
		_ = service.CreateCredential(ctx, &mcredential.Credential{
			ID:          id,
			WorkspaceID: workspaceID,
			Name:        "OpenAI",
			Kind:        mcredential.CREDENTIAL_KIND_OPENAI,
		})
		_ = service.CreateCredentialOpenAI(ctx, &mcredential.CredentialOpenAI{
			CredentialID: id,
			Token:        "sk-test",
		})

		model, err := factory.CreateModel(ctx, id)
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})

	t.Run("Create Gemini Model", func(t *testing.T) {
		id := idwrap.NewNow()
		_ = service.CreateCredential(ctx, &mcredential.Credential{
			ID:          id,
			WorkspaceID: workspaceID,
			Name:        "Gemini",
			Kind:        mcredential.CREDENTIAL_KIND_GEMINI,
		})
		_ = service.CreateCredentialGemini(ctx, &mcredential.CredentialGemini{
			CredentialID: id,
			ApiKey:       "test-key",
		})

		model, err := factory.CreateModel(ctx, id)
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})

	t.Run("Create Anthropic Model", func(t *testing.T) {
		id := idwrap.NewNow()
		_ = service.CreateCredential(ctx, &mcredential.Credential{
			ID:          id,
			WorkspaceID: workspaceID,
			Name:        "Anthropic",
			Kind:        mcredential.CREDENTIAL_KIND_ANTHROPIC,
		})
		_ = service.CreateCredentialAnthropic(ctx, &mcredential.CredentialAnthropic{
			CredentialID: id,
			ApiKey:       "ant-test",
		})

		model, err := factory.CreateModel(ctx, id)
		assert.NoError(t, err)
		assert.NotNil(t, model)
	})
}
