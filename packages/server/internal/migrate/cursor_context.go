package migrate

import (
	"context"
	"database/sql"
	"errors"
)

type cursorContextKey struct{}

type cursorManager struct {
	state CursorState
	store *Store
	id    string
}

func withCursorManager(ctx context.Context, mgr cursorManager) context.Context {
	return context.WithValue(ctx, cursorContextKey{}, mgr)
}

// CursorStateFromContext retrieves the persisted cursor state (if any).
func CursorStateFromContext(ctx context.Context) (CursorState, bool) {
	mgr, ok := ctx.Value(cursorContextKey{}).(cursorManager)
	if !ok {
		return nil, false
	}
	return mgr.state, mgr.state != nil
}

// SaveCursorState persists resumable state for the current migration.
func SaveCursorState(ctx context.Context, tx *sql.Tx, state CursorState) error {
	mgr, ok := ctx.Value(cursorContextKey{}).(cursorManager)
	if !ok {
		return errors.New("migrate: cursor manager not found in context")
	}
	return mgr.store.SaveCursor(ctx, tx, CursorParams{ID: mgr.id, Cursor: state})
}
