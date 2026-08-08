package ioworkspace

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mload"
)

// importWithLogger runs a bundle through Import against a fresh in-memory
// database, capturing everything the service logged.
func importWithLogger(t *testing.T, bundle *WorkspaceBundle) string {
	t.Helper()

	ctx := context.Background()

	db, _, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)

	queries := gen.New(db)
	wsID := idwrap.NewNow()
	require.NoError(t, queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:      wsID,
		Name:    "Load Warning WS",
		Updated: 0,
	}))

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelWarn}))

	for i := range bundle.Flows {
		bundle.Flows[i].WorkspaceID = wsID
	}

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, err = New(queries, logger).Import(ctx, tx, bundle, ImportOptions{
		WorkspaceID: wsID,
		PreserveIDs: true,
		ImportFlows: true,
	})
	require.NoError(t, err, "import must still succeed - load scenarios are dropped, not fatal")
	require.NoError(t, tx.Commit())

	return logs.String()
}

func loadScenarioBundle(scenarios ...mload.Scenario) *WorkspaceBundle {
	flowID := idwrap.NewNow()
	return &WorkspaceBundle{
		Flows:         []mflow.Flow{{ID: flowID, Name: "Checkout"}},
		LoadScenarios: scenarios,
	}
}

// TestImportWarnsThatLoadScenariosAreNotStored covers the one part of a bundle
// Import cannot persist. There is no schema for load scenarios yet, so a
// workspace imported from a file with a load: block will not export one back -
// which has to be said out loud rather than discovered from a diff.
func TestImportWarnsThatLoadScenariosAreNotStored(t *testing.T) {
	logs := importWithLogger(t, loadScenarioBundle(
		mload.Scenario{
			Name: "checkout-baseline", FlowName: "Checkout",
			Executor: mload.ExecutorConstantVUs, VUs: 10, Duration: 30 * time.Second,
		},
		mload.Scenario{
			Name: "browse-smoke", FlowName: "Checkout",
			Executor: mload.ExecutorConstantVUs, VUs: 1, MaxIterations: 25,
		},
	))

	if !strings.Contains(logs, LoadScenariosNotStoredMessage) {
		t.Errorf("import did not warn about unstored load scenarios; logs:\n%s", logs)
	}
	if !strings.Contains(logs, "count=2") {
		t.Errorf("warning does not name how many scenarios were dropped; logs:\n%s", logs)
	}
	for _, name := range []string{"checkout-baseline", "browse-smoke"} {
		if !strings.Contains(logs, name) {
			t.Errorf("warning does not name scenario %q; logs:\n%s", name, logs)
		}
	}
	if !strings.Contains(logs, "level=WARN") {
		t.Errorf("expected the message at warn level; logs:\n%s", logs)
	}
}

// TestImportSilentWithoutLoadScenarios keeps the warning from becoming noise
// on the overwhelmingly common import that has no load: block at all.
func TestImportSilentWithoutLoadScenarios(t *testing.T) {
	logs := importWithLogger(t, loadScenarioBundle())

	if strings.Contains(logs, LoadScenariosNotStoredMessage) {
		t.Errorf("warned about load scenarios on a bundle that has none; logs:\n%s", logs)
	}
}
