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

func TestCredentialService(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create in-memory db: %v", err)
	}
	defer cleanup()

	queries := gen.New(db)
	service := NewCredentialService(queries) // Uses default logger

	workspaceID := idwrap.NewNow()
	// Seed workspace
	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "WS",
	})
	if err != nil {
		t.Fatalf("failed to seed workspace: %v", err)
	}

	t.Run("Full lifecycle", func(t *testing.T) {
		id := idwrap.NewNow()
		cred := &mcredential.Credential{
			ID:          id,
			WorkspaceID: workspaceID,
			Name:        "Service Test",
			Kind:        mcredential.CREDENTIAL_KIND_OPENAI,
		}

		// Create
		err := service.CreateCredential(ctx, cred)
		assert.NoError(t, err)

		openai := &mcredential.CredentialOpenAI{
			CredentialID: id,
			Token:        "secret",
		}
		err = service.CreateCredentialOpenAI(ctx, openai)
		assert.NoError(t, err)

		// Read
		got, err := service.GetCredential(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, "Service Test", got.Name)

		gotOpenAI, err := service.GetCredentialOpenAI(ctx, id)
		assert.NoError(t, err)
		assert.Equal(t, "secret", gotOpenAI.Token)

		// Update
		cred.Name = "Updated Name"
		err = service.UpdateCredential(ctx, cred)
		assert.NoError(t, err)

		baseUrl := "https://proxy.com"
		openai.BaseUrl = &baseUrl
		err = service.UpdateCredentialOpenAI(ctx, openai)
		assert.NoError(t, err)

		// Verify Update
		got, _ = service.GetCredential(ctx, id)
		assert.Equal(t, "Updated Name", got.Name)
		gotOpenAI, _ = service.GetCredentialOpenAI(ctx, id)
		assert.Equal(t, "https://proxy.com", *gotOpenAI.BaseUrl)

		// List
		list, err := service.ListCredentials(ctx, workspaceID)
		assert.NoError(t, err)
		assert.Len(t, list, 1)

		// Delete
		err = service.DeleteCredential(ctx, id)
		assert.NoError(t, err)

		_, err = service.GetCredential(ctx, id)
		assert.Error(t, err)
	})
}
