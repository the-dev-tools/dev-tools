package migrate

import (
	"context"
	"database/sql"
)

// ApplyFunc mutates database state within a transaction.
type ApplyFunc func(ctx context.Context, tx *sql.Tx) error

// PrecheckFunc runs prior to opening a transaction for environmental validation.
type PrecheckFunc func(ctx context.Context, db *sql.DB) error

// ValidateFunc executes after commit to verify postconditions.
type ValidateFunc func(ctx context.Context, db *sql.DB) error

// AfterFunc runs post-validation for non-transactional work (e.g., checkpoints).
type AfterFunc func(ctx context.Context, db *sql.DB) error

// CursorState carries resumable progress for chunked migrations.
type CursorState map[string]any

// CursorFuncs manages persistence of resumable cursor state.
type CursorFuncs struct {
	Load func(ctx context.Context, tx *sql.Tx) (CursorState, error)
	Save func(ctx context.Context, tx *sql.Tx, state CursorState) error
}

// Migration describes a registered migration and its hooks.
type Migration struct {
	ID                 string
	Checksum           string
	Description        string
	Apply              ApplyFunc
	Precheck           PrecheckFunc
	Validate           ValidateFunc
	After              AfterFunc
	RequiresCheckpoint bool
	RequiresBackup     bool
	ChunkSize          int
	Cursor             *CursorFuncs
}

// HasCursor reports whether the migration provided cursor persistence helpers.
func (m Migration) HasCursor() bool {
	return m.Cursor != nil && m.Cursor.Load != nil && m.Cursor.Save != nil
}
