package sflow

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

func setupNodeAiProviderTest(t *testing.T) (context.Context, *sql.DB, *gen.Queries, idwrap.IDWrap, idwrap.IDWrap) {
	ctx := context.Background()
	db, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)

	queries := gen.New(db)

	// Create workspace
	workspaceID := idwrap.NewNow()
	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	require.NoError(t, err)

	// Create credential
	credentialID := idwrap.NewNow()
	err = queries.CreateCredential(ctx, gen.CreateCredentialParams{
		ID:          credentialID,
		WorkspaceID: workspaceID,
		Name:        "Test Credential",
		Kind:        1,
	})
	require.NoError(t, err)

	// Create flow
	flowID := idwrap.NewNow()
	err = queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:          flowID,
		WorkspaceID: workspaceID,
		Name:        "Test Flow",
	})
	require.NoError(t, err)

	// Create base node (AI_PROVIDER kind)
	nodeID := idwrap.NewNow()
	err = queries.CreateFlowNode(ctx, gen.CreateFlowNodeParams{
		ID:        nodeID,
		FlowID:    flowID,
		Name:      "Test AI Provider Node",
		NodeKind:  int32(mflow.NODE_KIND_AI_PROVIDER),
		PositionX: 100,
		PositionY: 200,
	})
	require.NoError(t, err)

	t.Cleanup(func() { db.Close() })

	return ctx, db, queries, nodeID, credentialID
}

func TestNodeAiProviderMapper_RoundTrip(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()
	temp := float32(0.7)
	maxTokens := int32(4096)

	mn := mflow.NodeAiProvider{
		FlowNodeID:   nodeID,
		CredentialID: &credID,
		Model:        mflow.AiModelGpt52Pro,
		Temperature:  &temp,
		MaxTokens:    &maxTokens,
	}

	dbn := ConvertNodeAiProviderToDB(mn)
	assert.Equal(t, nodeID.Bytes(), dbn.FlowNodeID)
	assert.Equal(t, credID.Bytes(), dbn.CredentialID)
	assert.Equal(t, int8(mflow.AiModelGpt52Pro), dbn.Model)
	assert.True(t, dbn.Temperature.Valid)
	assert.InDelta(t, 0.7, dbn.Temperature.Float64, 0.001)
	assert.True(t, dbn.MaxTokens.Valid)
	assert.Equal(t, int64(4096), dbn.MaxTokens.Int64)

	mn2 := ConvertDBToNodeAiProvider(dbn)
	assert.Equal(t, mn.FlowNodeID, mn2.FlowNodeID)
	require.NotNil(t, mn2.CredentialID)
	assert.Equal(t, *mn.CredentialID, *mn2.CredentialID)
	assert.Equal(t, mn.Model, mn2.Model)
	require.NotNil(t, mn2.Temperature)
	assert.InDelta(t, *mn.Temperature, *mn2.Temperature, 0.001)
	require.NotNil(t, mn2.MaxTokens)
	assert.Equal(t, *mn.MaxTokens, *mn2.MaxTokens)
}

func TestNodeAiProviderMapper_NilFields(t *testing.T) {
	nodeID := idwrap.NewNow()
	credID := idwrap.NewNow()

	mn := mflow.NodeAiProvider{
		FlowNodeID:   nodeID,
		CredentialID: &credID,
		Model:        mflow.AiModelClaudeSonnet45,
		Temperature:  nil,
		MaxTokens:    nil,
	}

	dbn := ConvertNodeAiProviderToDB(mn)
	assert.False(t, dbn.Temperature.Valid)
	assert.False(t, dbn.MaxTokens.Valid)

	mn2 := ConvertDBToNodeAiProvider(dbn)
	assert.Nil(t, mn2.Temperature)
	assert.Nil(t, mn2.MaxTokens)
}

func TestNodeAiProviderMapper_NilCredentialID(t *testing.T) {
	nodeID := idwrap.NewNow()

	mn := mflow.NodeAiProvider{
		FlowNodeID:   nodeID,
		CredentialID: nil, // No credential set
		Model:        mflow.AiModelClaudeSonnet45,
		Temperature:  nil,
		MaxTokens:    nil,
	}

	dbn := ConvertNodeAiProviderToDB(mn)
	assert.Empty(t, dbn.CredentialID)

	mn2 := ConvertDBToNodeAiProvider(dbn)
	assert.Nil(t, mn2.CredentialID)
}

func TestNodeAiProviderService_CRUD(t *testing.T) {
	ctx, db, queries, nodeID, credID := setupNodeAiProviderTest(t)

	service := NewNodeAiProviderService(queries)

	temp := float32(0.8)
	maxTokens := int32(2048)

	// Create
	provider := mflow.NodeAiProvider{
		FlowNodeID:   nodeID,
		CredentialID: &credID,
		Model:        mflow.AiModelGemini3Flash,
		Temperature:  &temp,
		MaxTokens:    &maxTokens,
	}

	// Use TX for write operations
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	writer := service.TX(tx)

	err = writer.CreateNodeAiProvider(ctx, provider)
	require.NoError(t, err)

	err = tx.Commit()
	require.NoError(t, err)

	// Read
	retrieved, err := service.GetNodeAiProvider(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, nodeID, retrieved.FlowNodeID)
	require.NotNil(t, retrieved.CredentialID)
	assert.Equal(t, credID, *retrieved.CredentialID)
	assert.Equal(t, mflow.AiModelGemini3Flash, retrieved.Model)
	require.NotNil(t, retrieved.Temperature)
	assert.InDelta(t, 0.8, *retrieved.Temperature, 0.001)
	require.NotNil(t, retrieved.MaxTokens)
	assert.Equal(t, int32(2048), *retrieved.MaxTokens)

	// Update
	newTemp := float32(0.5)
	provider.Temperature = &newTemp
	provider.Model = mflow.AiModelClaudeOpus45

	tx2, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	writer2 := service.TX(tx2)

	err = writer2.UpdateNodeAiProvider(ctx, provider)
	require.NoError(t, err)

	err = tx2.Commit()
	require.NoError(t, err)

	// Verify update
	updated, err := service.GetNodeAiProvider(ctx, nodeID)
	require.NoError(t, err)
	assert.Equal(t, mflow.AiModelClaudeOpus45, updated.Model)
	require.NotNil(t, updated.Temperature)
	assert.InDelta(t, 0.5, *updated.Temperature, 0.001)

	// Delete
	tx3, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	writer3 := service.TX(tx3)

	err = writer3.DeleteNodeAiProvider(ctx, nodeID)
	require.NoError(t, err)

	err = tx3.Commit()
	require.NoError(t, err)

	// Verify deletion
	_, err = service.GetNodeAiProvider(ctx, nodeID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestNodeAiProviderService_GetNonExistent(t *testing.T) {
	ctx, _, queries, _, _ := setupNodeAiProviderTest(t)

	service := NewNodeAiProviderService(queries)

	nonExistentID := idwrap.NewNow()
	_, err := service.GetNodeAiProvider(ctx, nonExistentID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
