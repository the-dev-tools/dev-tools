package ioworkspace

import (
	"context"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mcredential"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/scredential"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExport_AINodes(t *testing.T) {
	ctx := context.Background()

	db, _, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)

	queries := gen.New(db)
	wsID := idwrap.NewNow()

	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:      wsID,
		Name:    "Test WS",
		Updated: 0,
	})
	require.NoError(t, err)

	// Create credential for AI provider node
	credID := idwrap.NewNow()
	credWriter := scredential.NewCredentialWriterFromQueries(queries)
	err = credWriter.CreateCredential(ctx, &mcredential.Credential{
		ID:          credID,
		WorkspaceID: wsID,
		Name:        "OpenAI Key",
		Kind:        mcredential.CREDENTIAL_KIND_OPENAI,
	})
	require.NoError(t, err)

	// Build bundle with AI nodes
	flowID := idwrap.NewNow()
	aiNodeID := idwrap.NewNow()
	aiProviderNodeID := idwrap.NewNow()
	aiMemoryNodeID := idwrap.NewNow()

	temp := float32(0.7)

	bundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: flowID, WorkspaceID: wsID, Name: "AI Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: aiNodeID, FlowID: flowID, NodeKind: mflow.NODE_KIND_AI, Name: "AI Agent"},
			{ID: aiProviderNodeID, FlowID: flowID, NodeKind: mflow.NODE_KIND_AI_PROVIDER, Name: "AI Provider"},
			{ID: aiMemoryNodeID, FlowID: flowID, NodeKind: mflow.NODE_KIND_AI_MEMORY, Name: "AI Memory"},
		},
		FlowAINodes: []mflow.NodeAI{
			{FlowNodeID: aiNodeID, Prompt: "You are a helpful assistant", MaxIterations: 5},
		},
		FlowAIProviderNodes: []mflow.NodeAiProvider{
			{FlowNodeID: aiProviderNodeID, CredentialID: &credID, Model: mflow.AiModelGpt52, Temperature: &temp},
		},
		FlowAIMemoryNodes: []mflow.NodeMemory{
			{FlowNodeID: aiMemoryNodeID, MemoryType: mflow.AiMemoryTypeWindowBuffer, WindowSize: 10},
		},
	}

	// Import into DB
	svc := New(queries, nil)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = svc.Import(ctx, tx, bundle, ImportOptions{
		WorkspaceID: wsID,
		PreserveIDs: true,
		ImportFlows: true,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// Export and verify AI node data is present
	exported, err := svc.Export(ctx, ExportOptions{
		WorkspaceID:  wsID,
		IncludeFlows: true,
		ExportFormat: "json",
	})
	require.NoError(t, err)

	// Verify AI nodes
	assert.Len(t, exported.FlowAINodes, 1)
	assert.Equal(t, "You are a helpful assistant", exported.FlowAINodes[0].Prompt)
	assert.Equal(t, int32(5), exported.FlowAINodes[0].MaxIterations)

	// Verify AI provider nodes
	assert.Len(t, exported.FlowAIProviderNodes, 1)
	assert.Equal(t, mflow.AiModelGpt52, exported.FlowAIProviderNodes[0].Model)
	require.NotNil(t, exported.FlowAIProviderNodes[0].Temperature)
	assert.InDelta(t, 0.7, float64(*exported.FlowAIProviderNodes[0].Temperature), 0.001)
	require.NotNil(t, exported.FlowAIProviderNodes[0].CredentialID)
	assert.Equal(t, credID, *exported.FlowAIProviderNodes[0].CredentialID)

	// Verify AI memory nodes
	assert.Len(t, exported.FlowAIMemoryNodes, 1)
	assert.Equal(t, mflow.AiMemoryTypeWindowBuffer, exported.FlowAIMemoryNodes[0].MemoryType)
	assert.Equal(t, int32(10), exported.FlowAIMemoryNodes[0].WindowSize)

	// Verify credentials exported
	assert.Len(t, exported.Credentials, 1)
	assert.Equal(t, "OpenAI Key", exported.Credentials[0].Name)
	assert.Equal(t, mcredential.CREDENTIAL_KIND_OPENAI, exported.Credentials[0].Kind)
}

func TestExportImport_AINodes_RoundTrip(t *testing.T) {
	ctx := context.Background()

	db, _, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)

	queries := gen.New(db)
	wsID := idwrap.NewNow()

	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:      wsID,
		Name:    "Test WS",
		Updated: 0,
	})
	require.NoError(t, err)

	// Create credential
	credID := idwrap.NewNow()
	credWriter := scredential.NewCredentialWriterFromQueries(queries)
	err = credWriter.CreateCredential(ctx, &mcredential.Credential{
		ID:          credID,
		WorkspaceID: wsID,
		Name:        "Anthropic Key",
		Kind:        mcredential.CREDENTIAL_KIND_ANTHROPIC,
	})
	require.NoError(t, err)

	// Build original bundle
	flowID := idwrap.NewNow()
	aiNodeID := idwrap.NewNow()
	aiProviderNodeID := idwrap.NewNow()
	aiMemoryNodeID := idwrap.NewNow()

	temp := float32(0.9)
	maxTokens := int32(4096)

	originalBundle := &WorkspaceBundle{
		Flows: []mflow.Flow{
			{ID: flowID, WorkspaceID: wsID, Name: "AI Round Trip Flow"},
		},
		FlowNodes: []mflow.Node{
			{ID: aiNodeID, FlowID: flowID, NodeKind: mflow.NODE_KIND_AI, Name: "Agent"},
			{ID: aiProviderNodeID, FlowID: flowID, NodeKind: mflow.NODE_KIND_AI_PROVIDER, Name: "Provider"},
			{ID: aiMemoryNodeID, FlowID: flowID, NodeKind: mflow.NODE_KIND_AI_MEMORY, Name: "Memory"},
		},
		FlowAINodes: []mflow.NodeAI{
			{FlowNodeID: aiNodeID, Prompt: "Analyze the data", MaxIterations: 3},
		},
		FlowAIProviderNodes: []mflow.NodeAiProvider{
			{FlowNodeID: aiProviderNodeID, CredentialID: &credID, Model: mflow.AiModelClaudeSonnet45, Temperature: &temp, MaxTokens: &maxTokens},
		},
		FlowAIMemoryNodes: []mflow.NodeMemory{
			{FlowNodeID: aiMemoryNodeID, MemoryType: mflow.AiMemoryTypeWindowBuffer, WindowSize: 20},
		},
	}

	svc := New(queries, nil)

	// Import original bundle
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = svc.Import(ctx, tx, originalBundle, ImportOptions{
		WorkspaceID: wsID,
		PreserveIDs: true,
		ImportFlows: true,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// Export from DB
	exported, err := svc.Export(ctx, ExportOptions{
		WorkspaceID:  wsID,
		IncludeFlows: true,
		ExportFormat: "json",
	})
	require.NoError(t, err)

	// Re-import into a fresh workspace
	wsID2 := idwrap.NewNow()
	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:      wsID2,
		Name:    "Test WS 2",
		Updated: 0,
	})
	require.NoError(t, err)

	tx2, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	result2, err := svc.Import(ctx, tx2, exported, ImportOptions{
		WorkspaceID: wsID2,
		PreserveIDs: false,
		ImportFlows: true,
	})
	require.NoError(t, err)
	require.NoError(t, tx2.Commit())

	assert.Equal(t, 1, result2.FlowAINodesCreated)
	assert.Equal(t, 1, result2.FlowAIProviderNodesCreated)
	assert.Equal(t, 1, result2.FlowAIMemoryNodesCreated)

	// Export again and verify data survived the round-trip
	reExported, err := svc.Export(ctx, ExportOptions{
		WorkspaceID:  wsID2,
		IncludeFlows: true,
		ExportFormat: "json",
	})
	require.NoError(t, err)

	// Verify AI node data
	require.Len(t, reExported.FlowAINodes, 1)
	assert.Equal(t, "Analyze the data", reExported.FlowAINodes[0].Prompt)
	assert.Equal(t, int32(3), reExported.FlowAINodes[0].MaxIterations)

	// Verify AI provider node data
	require.Len(t, reExported.FlowAIProviderNodes, 1)
	assert.Equal(t, mflow.AiModelClaudeSonnet45, reExported.FlowAIProviderNodes[0].Model)
	require.NotNil(t, reExported.FlowAIProviderNodes[0].Temperature)
	assert.InDelta(t, 0.9, float64(*reExported.FlowAIProviderNodes[0].Temperature), 0.001)
	require.NotNil(t, reExported.FlowAIProviderNodes[0].MaxTokens)
	assert.Equal(t, int32(4096), *reExported.FlowAIProviderNodes[0].MaxTokens)

	// Verify AI memory node data
	require.Len(t, reExported.FlowAIMemoryNodes, 1)
	assert.Equal(t, mflow.AiMemoryTypeWindowBuffer, reExported.FlowAIMemoryNodes[0].MemoryType)
	assert.Equal(t, int32(20), reExported.FlowAIMemoryNodes[0].WindowSize)
}
