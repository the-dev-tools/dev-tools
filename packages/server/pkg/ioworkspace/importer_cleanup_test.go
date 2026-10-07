package ioworkspace

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
)

// cleanupBundle builds a bundle with one flow that owns a one-step cleanup
// block (a wait node, which needs no HTTP entities).
func cleanupBundle(wsID idwrap.IDWrap) (*WorkspaceBundle, FlowCleanup) {
	flowID := idwrap.NewNow()
	cleanupFlowID := idwrap.NewNow()
	nodeID := idwrap.NewNow()

	cleanup := FlowCleanup{
		FlowID:        flowID,
		CleanupFlowID: cleanupFlowID,
		Steps:         []FlowCleanupStep{{NodeID: nodeID, Name: "Pause"}},
		Bundle: &WorkspaceBundle{
			Flows:         []mflow.Flow{{ID: cleanupFlowID, WorkspaceID: wsID, Name: "Checkout cleanup"}},
			FlowNodes:     []mflow.Node{{ID: nodeID, FlowID: cleanupFlowID, Name: "Pause", NodeKind: mflow.NODE_KIND_WAIT}},
			FlowWaitNodes: []mflow.NodeWait{{FlowNodeID: nodeID, DurationMs: 1}},
		},
	}

	return &WorkspaceBundle{
		Flows:        []mflow.Flow{{ID: flowID, WorkspaceID: wsID, Name: "Checkout"}},
		FlowCleanups: []FlowCleanup{cleanup},
	}, cleanup
}

func importCleanupBundle(t *testing.T, importCleanups bool) (string, *gen.Queries, FlowCleanup, idwrap.IDWrap) {
	t.Helper()

	ctx := context.Background()
	db, _, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)

	queries := gen.New(db)
	wsID := idwrap.NewNow()
	require.NoError(t, queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{ID: wsID, Name: "Cleanup WS"}))

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))

	bundle, cleanup := cleanupBundle(wsID)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = New(queries, logger).Import(ctx, tx, bundle, ImportOptions{
		WorkspaceID:        wsID,
		PreserveIDs:        true,
		ImportFlows:        true,
		ImportFlowCleanups: importCleanups,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	return logs.String(), queries, cleanup, wsID
}

func TestImportStoresFlowCleanupsWhenAsked(t *testing.T) {
	logs, queries, cleanup, _ := importCleanupBundle(t, true)
	require.NotContains(t, logs, FlowCleanupsNotStoredMessage)

	ctx := context.Background()
	nodes, err := sflow.NewNodeService(queries).GetNodesByFlowID(ctx, cleanup.CleanupFlowID)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	require.Equal(t, "Pause", nodes[0].Name)
}

func TestImportDropsFlowCleanupsWithWarningByDefault(t *testing.T) {
	logs, queries, cleanup, wsID := importCleanupBundle(t, false)
	require.Contains(t, logs, FlowCleanupsNotStoredMessage)
	require.Contains(t, logs, "Checkout")

	ctx := context.Background()
	flowService := sflow.NewFlowService(queries)
	flows, err := flowService.GetFlowsByWorkspaceID(ctx, wsID)
	require.NoError(t, err)
	for _, f := range flows {
		require.NotEqual(t, cleanup.CleanupFlowID, f.ID, "hidden cleanup flow must not be stored")
	}
}

func TestImportFlowCleanupsRequiresPreservedIDs(t *testing.T) {
	err := ImportOptions{
		WorkspaceID:        idwrap.NewNow(),
		ImportFlowCleanups: true,
	}.Validate()
	require.ErrorIs(t, err, ErrFlowCleanupsNeedPreservedIDs)
}
