package txread

import (
	"context"
	"database/sql"
)

// This file demonstrates the EXACT bug pattern that was in rimportv2/storage.go
// and verifies the linter would have caught it.

// WorkspaceService simulates the workspace service from the real codebase
type WorkspaceService struct {
	db *sql.DB
}

func (s *WorkspaceService) Get(ctx context.Context, id int) (string, error) {
	return "", nil
}

func (s *WorkspaceService) TX(tx *sql.Tx) *WorkspaceServiceTX {
	return &WorkspaceServiceTX{tx: tx}
}

type WorkspaceServiceTX struct {
	tx *sql.Tx
}

func (s *WorkspaceServiceTX) Get(ctx context.Context, id int) (string, error) {
	return "", nil
}

// DefaultImporter simulates the importer struct
type DefaultImporter struct {
	db               *sql.DB
	workspaceService *WorkspaceService
}

// OriginalBugPattern recreates the EXACT bug that was in storage.go
// The workspace read was INSIDE the transaction, causing SQLite deadlock
func (imp *DefaultImporter) OriginalBugPattern(ctx context.Context, workspaceID int) error {
	// PHASE 2: Storage (Write) - BeginTx
	tx, err := imp.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// BUG: This read was inside the transaction!
	// In file-based SQLite, this causes deadlock because:
	// - We hold a write lock (BeginTx)
	// - workspaceService.Get() tries to read via a different connection
	// - The read waits for the write lock to release
	// - But we're waiting for the read to complete
	// = DEADLOCK
	workspace, err := imp.workspaceService.Get(ctx, workspaceID) // want "non-TX-bound read Get\\(\\) inside transaction"
	if err != nil {
		return err
	}
	_ = workspace

	return tx.Commit()
}

// FixedPattern shows the correct pattern after the fix
func (imp *DefaultImporter) FixedPattern(ctx context.Context, workspaceID int) error {
	// PHASE 1: Pre-Resolution (Read-only)
	// CRITICAL: Read BEFORE starting transaction
	workspace, err := imp.workspaceService.Get(ctx, workspaceID)
	if err != nil {
		return err
	}
	_ = workspace

	// PHASE 2: Storage (Write) - BeginTx
	tx, err := imp.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Only writes inside transaction
	return tx.Commit()
}

// AlternativeFixWithTXBound shows another valid fix using TX-bound service
func (imp *DefaultImporter) AlternativeFixWithTXBound(ctx context.Context, workspaceID int) error {
	tx, err := imp.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// GOOD: Use TX-bound service - reads via same connection as write lock
	txWorkspace := imp.workspaceService.TX(tx)
	workspace, err := txWorkspace.Get(ctx, workspaceID)
	if err != nil {
		return err
	}
	_ = workspace

	return tx.Commit()
}
