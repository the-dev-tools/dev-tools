package migrate

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
)

func TestStoreMarkStartedInsertsAndUpdates(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	store := NewStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	id := newID()
	checksum := "checksum-1"
	startedAt := time.Now()
	backupPath := "backup-1"

	tx := beginTx(ctx, t, db)
	rec, err := store.MarkStarted(ctx, tx, StartParams{
		ID:         id,
		Checksum:   checksum,
		StartedAt:  startedAt,
		BackupPath: &backupPath,
	})
	if err != nil {
		t.Fatalf("mark started: %v", err)
	}
	commitTx(t, tx)

	if rec.Status != StatusStarted {
		t.Fatalf("expected status started, got %s", rec.Status)
	}
	if rec.Attempts != 1 {
		t.Fatalf("expected attempts 1, got %d", rec.Attempts)
	}
	if rec.BackupPath.String != backupPath {
		t.Fatalf("expected backup path %s, got %s", backupPath, rec.BackupPath.String)
	}

	tx = beginTx(ctx, t, db)
	rec, err = store.MarkStarted(ctx, tx, StartParams{
		ID:        id,
		Checksum:  checksum,
		StartedAt: startedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("mark started second time: %v", err)
	}
	commitTx(t, tx)

	if rec.Attempts != 2 {
		t.Fatalf("expected attempts 2, got %d", rec.Attempts)
	}
	if rec.BackupPath.Valid {
		t.Fatalf("expected backup path cleared, got %v", rec.BackupPath.String)
	}
}

func TestStoreMarkFinishedClearsCursorAndError(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	store := NewStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	id := newID()
	tx := beginTx(ctx, t, db)
	if _, err := store.MarkStarted(ctx, tx, StartParams{ID: id, Checksum: "sum", StartedAt: time.Now()}); err != nil {
		t.Fatalf("mark started: %v", err)
	}
	if err := store.SetError(ctx, tx, ErrorParams{ID: id, LastError: "boom"}); err != nil {
		t.Fatalf("set error: %v", err)
	}
	if err := store.SaveCursor(ctx, tx, CursorParams{ID: id, Cursor: CursorState{"step": float64(1)}}); err != nil {
		t.Fatalf("save cursor: %v", err)
	}
	commitTx(t, tx)

	tx = beginTx(ctx, t, db)
	rec, err := store.MarkFinished(ctx, tx, FinishParams{ID: id, FinishedAt: time.Now()})
	if err != nil {
		t.Fatalf("mark finished: %v", err)
	}
	commitTx(t, tx)

	if rec.Status != StatusFinished {
		t.Fatalf("expected finished status, got %s", rec.Status)
	}
	if rec.LastError.Valid {
		t.Fatalf("expected last error cleared, got %s", rec.LastError.String)
	}
	if rec.CursorJSON.Valid {
		t.Fatalf("expected cursor cleared, got %s", rec.CursorJSON.String)
	}
}

func TestStoreChecksumMismatch(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	store := NewStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	id := newID()
	tx := beginTx(ctx, t, db)
	if _, err := store.MarkStarted(ctx, tx, StartParams{ID: id, Checksum: "sum1", StartedAt: time.Now()}); err != nil {
		t.Fatalf("mark started: %v", err)
	}
	if _, err := store.MarkFinished(ctx, tx, FinishParams{ID: id, FinishedAt: time.Now()}); err != nil {
		t.Fatalf("mark finished: %v", err)
	}
	commitTx(t, tx)

	tx = beginTx(ctx, t, db)
	_, err = store.MarkStarted(ctx, tx, StartParams{ID: id, Checksum: "sum2", StartedAt: time.Now()})
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	rollbackTx(t, tx)
}

func TestStoreCursorRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	store := NewStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	id := newID()
	tx := beginTx(ctx, t, db)
	if _, err := store.MarkStarted(ctx, tx, StartParams{ID: id, Checksum: "sum", StartedAt: time.Now()}); err != nil {
		t.Fatalf("mark started: %v", err)
	}
	if err := store.SaveCursor(ctx, tx, CursorParams{ID: id, Cursor: CursorState{"offset": float64(42)}}); err != nil {
		t.Fatalf("save cursor: %v", err)
	}
	state, err := store.LoadCursor(ctx, tx, id)
	if err != nil {
		t.Fatalf("load cursor: %v", err)
	}
	commitTx(t, tx)

	if state == nil {
		t.Fatalf("expected cursor state")
	}
	if got := state["offset"]; got != float64(42) {
		t.Fatalf("expected offset 42, got %v", got)
	}
}

func TestStoreSaveCursorRequiresExistingRow(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	store := NewStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	tx := beginTx(ctx, t, db)
	err = store.SaveCursor(ctx, tx, CursorParams{ID: newID(), Cursor: CursorState{"offset": float64(1)}})
	if err == nil {
		t.Fatalf("expected error when saving cursor without record")
	}
	rollbackTx(t, tx)
}

func TestStoreEnsureSchemaIdempotent(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	store := NewStore(db)
	for i := 0; i < 3; i++ {
		if err := store.EnsureSchema(ctx); err != nil {
			t.Fatalf("ensure schema iteration %d: %v", i, err)
		}
	}
}

func FuzzCursorRoundTrip(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(42))

	f.Fuzz(func(t *testing.T, seed uint64) {
		ctx := context.Background()
		db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
		if err != nil {
			t.Fatalf("new sqlite: %v", err)
		}
		t.Cleanup(cleanup)

		store := NewStore(db)
		if err := store.EnsureSchema(ctx); err != nil {
			t.Fatalf("ensure schema: %v", err)
		}

		id := newID()
		tx := beginTx(ctx, t, db)
		if _, err := store.MarkStarted(ctx, tx, StartParams{ID: id, Checksum: "sum", StartedAt: time.Now()}); err != nil {
			t.Fatalf("mark started: %v", err)
		}

		cursor := CursorState{
			"seed":         float64(seed),
			"timestamp_ms": float64(time.Now().UnixMilli()),
		}

		if err := store.SaveCursor(ctx, tx, CursorParams{ID: id, Cursor: cursor}); err != nil {
			t.Fatalf("save cursor: %v", err)
		}

		loaded, err := store.LoadCursor(ctx, tx, id)
		if err != nil {
			t.Fatalf("load cursor: %v", err)
		}
		if got := loaded["seed"]; got != cursor["seed"] {
			t.Fatalf("seed mismatch: got %v want %v", got, cursor["seed"])
		}
		commitTx(t, tx)
	})
}

func beginTx(ctx context.Context, t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	return tx
}

func commitTx(t *testing.T, tx *sql.Tx) {
	t.Helper()
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
}

func rollbackTx(t *testing.T, tx *sql.Tx) {
	t.Helper()
	if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
		t.Fatalf("rollback tx: %v", err)
	}
}
