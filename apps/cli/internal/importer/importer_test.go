package importer_test

import (
	"context"
	"log/slog"
	"testing"

	"the-dev-tools/cli/internal/common"
	"the-dev-tools/cli/internal/importer"
	"the-dev-tools/server/pkg/idwrap"
)

func TestRunImport_FailsOnMissingWorkspace(t *testing.T) {
	// Since RunImport creates a fresh in-memory DB and checks for workspace *before* callback,
	// it is guaranteed to fail with "workspace not found" (or "sql: no rows").
	// This test confirms that behavior, which matches the code structure.

	err := importer.RunImport(context.Background(), slog.Default(), idwrap.NewNow().String(), "", func(ctx context.Context, s *common.Services, w idwrap.IDWrap, f *idwrap.IDWrap) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error due to missing workspace in empty DB, got nil")
	}

	// We expect "workspace not found" or "no rows"
	// Check error string content
	if err.Error() == "" {
		t.Fatal("got empty error")
	}
}

// Ensure the code compiles.
