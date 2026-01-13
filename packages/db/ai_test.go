package devtoolsdb

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

func TestAiAndCredentials(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("failed to create in-memory db: %v", err)
	}
	defer cleanup()

	queries := gen.New(db)

	workspaceID := idwrap.NewNow()
	// Insert Workspace
	if _, err := db.ExecContext(ctx, "INSERT INTO workspaces (id, name) VALUES (?, ?)", workspaceID.Bytes(), "Test WS"); err != nil {
		t.Fatalf("failed to insert workspace: %v", err)
	}

	t.Run("CredentialCRUD", func(t *testing.T) {
		credID := idwrap.NewNow()

		// Create
		err := queries.CreateCredential(ctx, gen.CreateCredentialParams{
			ID:          credID,
			WorkspaceID: workspaceID,
			Name:        "My OpenAI",
			Kind:        0, // OpenAI
		})
		if err != nil {
			t.Fatalf("failed to create credential: %v", err)
		}

		err = queries.CreateCredentialOpenAI(ctx, gen.CreateCredentialOpenAIParams{
			CredentialID:   credID,
			Token:          []byte("sk-12345"),
			BaseUrl:        sql.NullString{Valid: false},
			EncryptionType: 0, // No encryption for tests
		})
		if err != nil {
			t.Fatalf("failed to create credential openai: %v", err)
		}

		// Read
		cred, err := queries.GetCredential(ctx, credID)
		if err != nil {
			t.Fatalf("failed to get credential: %v", err)
		}
		if cred.Name != "My OpenAI" {
			t.Errorf("expected name 'My OpenAI', got '%s'", cred.Name)
		}

		credOpenAI, err := queries.GetCredentialOpenAI(ctx, credID)
		if err != nil {
			t.Fatalf("failed to get credential openai: %v", err)
		}
		if string(credOpenAI.Token) != "sk-12345" {
			t.Errorf("expected token 'sk-12345', got '%s'", string(credOpenAI.Token))
		}

		// Update
		err = queries.UpdateCredentialOpenAI(ctx, gen.UpdateCredentialOpenAIParams{
			CredentialID:   credID,
			Token:          []byte("sk-updated"),
			BaseUrl:        sql.NullString{String: "https://api.openai.com/v1", Valid: true},
			EncryptionType: 0,
		})
		if err != nil {
			t.Fatalf("failed to update credential openai: %v", err)
		}

		credOpenAI, _ = queries.GetCredentialOpenAI(ctx, credID)
		if string(credOpenAI.Token) != "sk-updated" || credOpenAI.BaseUrl.String != "https://api.openai.com/v1" {
			t.Errorf("update failed")
		}

		// Delete
		err = queries.DeleteCredential(ctx, credID)
		if err != nil {
			t.Fatalf("failed to delete credential: %v", err)
		}

		_, err = queries.GetCredential(ctx, credID)
		if err == nil {
			t.Errorf("expected error after delete, got nil")
		}
	})

	t.Run("GeminiCredentialCRUD", func(t *testing.T) {
		credID := idwrap.NewNow()

		// Create
		err := queries.CreateCredential(ctx, gen.CreateCredentialParams{
			ID:          credID,
			WorkspaceID: workspaceID,
			Name:        "My Gemini",
			Kind:        1, // Gemini
		})
		if err != nil {
			t.Fatalf("failed to create credential: %v", err)
		}

		err = queries.CreateCredentialGemini(ctx, gen.CreateCredentialGeminiParams{
			CredentialID:   credID,
			ApiKey:         []byte("gemini-123"),
			BaseUrl:        sql.NullString{Valid: false},
			EncryptionType: 0,
		})
		if err != nil {
			t.Fatalf("failed to create credential gemini: %v", err)
		}

		// Read
		cred, err := queries.GetCredential(ctx, credID)
		if err != nil {
			t.Fatalf("failed to get credential: %v", err)
		}
		assert.Equal(t, int8(1), cred.Kind)

		credGemini, err := queries.GetCredentialGemini(ctx, credID)
		if err != nil {
			t.Fatalf("failed to get credential gemini: %v", err)
		}
		assert.Equal(t, "gemini-123", string(credGemini.ApiKey))

		// Update
		err = queries.UpdateCredentialGemini(ctx, gen.UpdateCredentialGeminiParams{
			CredentialID:   credID,
			ApiKey:         []byte("gemini-updated"),
			BaseUrl:        sql.NullString{String: "https://gemini.api", Valid: true},
			EncryptionType: 0,
		})
		if err != nil {
			t.Fatalf("failed to update credential gemini: %v", err)
		}

		credGemini, _ = queries.GetCredentialGemini(ctx, credID)
		assert.Equal(t, "gemini-updated", string(credGemini.ApiKey))
		assert.Equal(t, "https://gemini.api", credGemini.BaseUrl.String)

		// Delete
		err = queries.DeleteCredential(ctx, credID)
		if err != nil {
			t.Fatalf("failed to delete credential: %v", err)
		}
	})

	t.Run("AnthropicCredentialCRUD", func(t *testing.T) {
		credID := idwrap.NewNow()

		// Create
		err := queries.CreateCredential(ctx, gen.CreateCredentialParams{
			ID:          credID,
			WorkspaceID: workspaceID,
			Name:        "My Anthropic",
			Kind:        2, // Anthropic
		})
		if err != nil {
			t.Fatalf("failed to create credential: %v", err)
		}

		err = queries.CreateCredentialAnthropic(ctx, gen.CreateCredentialAnthropicParams{
			CredentialID:   credID,
			ApiKey:         []byte("claude-123"),
			BaseUrl:        sql.NullString{Valid: false},
			EncryptionType: 0,
		})
		if err != nil {
			t.Fatalf("failed to create credential anthropic: %v", err)
		}

		// Read
		cred, err := queries.GetCredential(ctx, credID)
		if err != nil {
			t.Fatalf("failed to get credential: %v", err)
		}
		assert.Equal(t, int8(2), cred.Kind)

		credAnthropic, err := queries.GetCredentialAnthropic(ctx, credID)
		if err != nil {
			t.Fatalf("failed to get credential anthropic: %v", err)
		}
		assert.Equal(t, "claude-123", string(credAnthropic.ApiKey))

		// Update
		err = queries.UpdateCredentialAnthropic(ctx, gen.UpdateCredentialAnthropicParams{
			CredentialID:   credID,
			ApiKey:         []byte("claude-updated"),
			BaseUrl:        sql.NullString{String: "https://anthropic.api", Valid: true},
			EncryptionType: 0,
		})
		if err != nil {
			t.Fatalf("failed to update credential anthropic: %v", err)
		}

		credAnthropic, _ = queries.GetCredentialAnthropic(ctx, credID)
		assert.Equal(t, "claude-updated", string(credAnthropic.ApiKey))
		assert.Equal(t, "https://anthropic.api", credAnthropic.BaseUrl.String)

		// Delete
		err = queries.DeleteCredential(ctx, credID)
		if err != nil {
			t.Fatalf("failed to delete credential: %v", err)
		}
	})

	t.Run("FlowNodeAiCRUD", func(t *testing.T) {
		flowID := idwrap.NewNow()
		nodeID := idwrap.NewNow()
		credID := idwrap.NewNow()

		// Create Credential for FK
		err := queries.CreateCredential(ctx, gen.CreateCredentialParams{
			ID:          credID,
			WorkspaceID: workspaceID,
			Name:        "Test Cred",
			Kind:        0,
		})
		if err != nil {
			t.Fatalf("failed to create credential: %v", err)
		}

		// Setup Flow and Node
		if _, err := db.ExecContext(ctx, "INSERT INTO flow (id, workspace_id, name) VALUES (?, ?, ?)", flowID.Bytes(), workspaceID.Bytes(), "Flow"); err != nil {
			t.Fatalf("failed to insert flow: %v", err)
		}

		err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
			ID:        nodeID,
			FlowID:    flowID,
			Name:      "AI Task",
			NodeKind:  6, // Ai
			PositionX: 100,
			PositionY: 200,
		})
		if err != nil {
			t.Fatalf("failed to create flow node: %v", err)
		}

		// Create FlowNodeAI
		err = queries.CreateFlowNodeAI(ctx, gen.CreateFlowNodeAIParams{
			FlowNodeID:    nodeID,
			Model:         0,
			CredentialID:  credID.Bytes(),
			Prompt:        "Summarize this: {{input}}",
			MaxIterations: 5,
		})
		if err != nil {
			t.Fatalf("failed to create flow node ai: %v", err)
		}

		// Read
		nodeAi, err := queries.GetFlowNodeAI(ctx, nodeID)
		if err != nil {
			t.Fatalf("failed to get flow node ai: %v", err)
		}
		if nodeAi.Prompt != "Summarize this: {{input}}" {
			t.Errorf("unexpected prompt")
		}
		if nodeAi.MaxIterations != 5 {
			t.Errorf("unexpected max iterations")
		}

		// Update
		err = queries.UpdateFlowNodeAI(ctx, gen.UpdateFlowNodeAIParams{
			FlowNodeID:    nodeID,
			Model:         0,
			CredentialID:  credID.Bytes(),
			Prompt:        "Updated prompt",
			MaxIterations: 10,
		})
		if err != nil {
			t.Fatalf("failed to update flow node ai: %v", err)
		}

		nodeAi, _ = queries.GetFlowNodeAI(ctx, nodeID)
		if nodeAi.Prompt != "Updated prompt" {
			t.Errorf("update failed")
		}
		if nodeAi.MaxIterations != 10 {
			t.Errorf("update failed max iterations")
		}

		// Delete
		err = queries.DeleteFlowNodeAI(ctx, nodeID)
		if err != nil {
			t.Fatalf("failed to delete flow node ai: %v", err)
		}
	})
}
