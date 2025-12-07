package importer

import (
	"context"
	"fmt"
	"log/slog"

	"the-dev-tools/cli/internal/common"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
)

// ImportCallback is the function signature for the actual import logic
type ImportCallback func(ctx context.Context, services *common.Services, wsID idwrap.IDWrap, folderIDPtr *idwrap.IDWrap) error

// RunImport performs the common setup for import operations
func RunImport(ctx context.Context, logger *slog.Logger, workspaceID, folderID string, fn ImportCallback) error {
	// Parse workspace and folder IDs
	wsID, err := idwrap.NewText(workspaceID)
	if err != nil {
		return fmt.Errorf("invalid workspace ID: %w", err)
	}

	var folderIDPtr *idwrap.IDWrap
	if folderID != "" {
		fid, err := idwrap.NewText(folderID)
		if err != nil {
			return fmt.Errorf("invalid folder ID: %w", err)
		}
		folderIDPtr = &fid
	}

	// Create in-memory database and services
	db, _, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer func() { _ = db.Close() }()

	services, err := common.CreateServices(ctx, db, logger)
	if err != nil {
		return err
	}

	// Verify workspace exists
	_, err = services.Workspace.Get(ctx, wsID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	// Run specific import logic
	return fn(ctx, services, wsID, folderIDPtr)
}
