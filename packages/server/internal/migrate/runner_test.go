package migrate

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
)

func TestRunnerApplyAll(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	ResetForTesting()

	id := newID()
	table := "migration_apply_all"
	if err := Register(Migration{
		ID:       id,
		Checksum: "test-checksum-apply",
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+table+" (id INTEGER PRIMARY KEY)")
			return err
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := Config{DatabasePath: filepath.Join(t.TempDir(), "db.sqlite")}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	if err := runner.ApplyAll(ctx); err != nil {
		t.Fatalf("apply all: %v", err)
	}

	var name string
	if err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name); err != nil {
		t.Fatalf("table missing: %v", err)
	}

	store := NewStore(db)
	rec, err := store.GetRecord(ctx, id)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if rec.Status != StatusFinished {
		t.Fatalf("expected finished status, got %s", rec.Status)
	}
	if rec.Attempts != 1 {
		t.Fatalf("expected attempts 1, got %d", rec.Attempts)
	}
}

func TestRunnerSkipsFinishedMigrations(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	ResetForTesting()

	id := newID()
	var applies int
	if err := Register(Migration{
		ID:       id,
		Checksum: "test-checksum-skip",
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			applies++
			_, err := tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS skip_table (id INTEGER PRIMARY KEY)")
			return err
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := Config{DatabasePath: filepath.Join(t.TempDir(), "db.sqlite")}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	if err := runner.ApplyAll(ctx); err != nil {
		t.Fatalf("apply all first: %v", err)
	}
	if err := runner.ApplyAll(ctx); err != nil {
		t.Fatalf("apply all second: %v", err)
	}

	if applies != 1 {
		t.Fatalf("expected apply once, got %d", applies)
	}
}

func TestRunnerRecordsErrors(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	ResetForTesting()

	id := newID()
	if err := Register(Migration{
		ID:       id,
		Checksum: "test-checksum-error",
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			return errors.New("boom")
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := Config{DatabasePath: filepath.Join(t.TempDir(), "db.sqlite")}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	err = runner.ApplyAll(ctx)
	if err == nil {
		t.Fatalf("expected error from apply all")
	}

	store := NewStore(db)
	rec, err := store.GetRecord(ctx, id)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if rec.Status != StatusStarted {
		t.Fatalf("expected status started, got %s", rec.Status)
	}
	if !rec.LastError.Valid {
		t.Fatalf("expected last_error to be set")
	}
}

func TestRunnerCreatesBackup(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "file.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open file db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sqlc.CreateLocalTables(ctx, db); err != nil {
		t.Fatalf("create tables: %v", err)
	}

	ResetForTesting()

	id := newID()
	if err := Register(Migration{
		ID:             id,
		Checksum:       "test-checksum-backup",
		RequiresBackup: true,
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS backup_table (id INTEGER PRIMARY KEY)")
			return err
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "backups")
	cfg := Config{
		DatabasePath:  dbPath,
		BackupDir:     backupDir,
		RetainBackups: 2,
	}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	if err := runner.ApplyAll(ctx); err != nil {
		t.Fatalf("apply all: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 backup dir, got %d", len(entries))
	}
	backupPath := filepath.Join(backupDir, entries[0].Name(), filepath.Base(dbPath))
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}

	store := NewStore(db)
	rec, err := store.GetRecord(ctx, id)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if !rec.BackupPath.Valid {
		t.Fatalf("expected backup path stored")
	}
}

func TestRunnerCursorStateLifecycle(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	ResetForTesting()

	id := newID()
	var validateCalls int
	var applyCalls int
	if err := Register(Migration{
		ID:       id,
		Checksum: "test-checksum-cursor",
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			state, has := CursorStateFromContext(ctx)
			if applyCalls == 1 {
				if !has {
					t.Fatalf("expected cursor state on retry")
				}
				if got := state["offset"]; got != float64(10) {
					t.Fatalf("expected offset 10, got %v", got)
				}
			}
			applyCalls++
			return SaveCursorState(ctx, tx, CursorState{"offset": float64(10)})
		},
		Validate: func(ctx context.Context, db *sql.DB) error {
			validateCalls++
			if validateCalls == 1 {
				return errors.New("validation failed")
			}
			return nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := Config{DatabasePath: filepath.Join(t.TempDir(), "db.sqlite")}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	if err := runner.ApplyAll(ctx); err == nil {
		t.Fatalf("expected validation error on first run")
	}

	store := NewStore(db)
	rec, err := store.GetRecord(ctx, id)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if !rec.LastError.Valid {
		t.Fatalf("expected last_error set after validation failure")
	}

	// second attempt should see persisted cursor and finish successfully
	if err := runner.ApplyAll(ctx); err != nil {
		t.Fatalf("apply all second: %v", err)
	}

	rec, err = store.GetRecord(ctx, id)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if rec.CursorJSON.Valid {
		t.Fatalf("expected cursor cleared after success")
	}
	if rec.Attempts != 2 {
		t.Fatalf("expected attempts 2, got %d", rec.Attempts)
	}
}

func TestRunnerAfterFailure(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	ResetForTesting()

	id := newID()
	if err := Register(Migration{
		ID:       id,
		Checksum: "test-checksum-after",
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "CREATE TABLE after_table (id INTEGER PRIMARY KEY)")
			return err
		},
		After: func(ctx context.Context, db *sql.DB) error {
			return errors.New("after failed")
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := Config{DatabasePath: filepath.Join(t.TempDir(), "db.sqlite")}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	if err := runner.ApplyAll(ctx); err == nil {
		t.Fatalf("expected after failure error")
	}

	store := NewStore(db)
	rec, err := store.GetRecord(ctx, id)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if rec.Status != StatusStarted {
		t.Fatalf("expected status started after after failure, got %s", rec.Status)
	}
	if !rec.LastError.Valid {
		t.Fatalf("expected last_error to be recorded")
	}
}

func TestRunnerBackupFailure(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	ResetForTesting()

	id := newID()
	if err := Register(Migration{
		ID:             id,
		Checksum:       "test-checksum-backup-fail",
		RequiresBackup: true,
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			return nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := Config{DatabasePath: filepath.Join(t.TempDir(), "db.sqlite")}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	if err := runner.ApplyAll(ctx); err == nil {
		t.Fatalf("expected backup failure error")
	}
}

func TestRunnerRetryAfterCrash(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	ResetForTesting()

	id := newID()
	var attempt int
	if err := Register(Migration{
		ID:       id,
		Checksum: "test-checksum-retry",
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			attempt++
			if attempt == 1 {
				return errors.New("first attempt fails")
			}
			_, err := tx.ExecContext(ctx, "CREATE TABLE retry_table (id INTEGER PRIMARY KEY)")
			return err
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := Config{DatabasePath: filepath.Join(t.TempDir(), "db.sqlite")}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	if err := runner.ApplyAll(ctx); err == nil {
		t.Fatalf("expected first run failure")
	}

	if err := runner.ApplyAll(ctx); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	store := NewStore(db)
	rec, err := store.GetRecord(ctx, id)
	if err != nil {
		t.Fatalf("get record: %v", err)
	}
	if rec.Attempts != 2 {
		t.Fatalf("expected attempts 2, got %d", rec.Attempts)
	}
	if rec.Status != StatusFinished {
		t.Fatalf("expected finished after retry")
	}
}

func TestRunnerSerializesConcurrentCalls(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	if err != nil {
		t.Fatalf("new sqlite: %v", err)
	}
	t.Cleanup(cleanup)

	ResetForTesting()

	ready := make(chan struct{})
	release := make(chan struct{})

	id := newID()
	if err := Register(Migration{
		ID:       id,
		Checksum: "test-checksum-concurrency",
		Apply: func(ctx context.Context, tx *sql.Tx) error {
			close(ready)
			<-release
			return nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	cfg := Config{DatabasePath: filepath.Join(t.TempDir(), "db.sqlite")}
	runner, err := NewRunner(db, cfg, slogDiscard())
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- runner.ApplyAll(ctx)
	}()

	<-ready

	secondDone := make(chan error, 1)
	go func() {
		secondDone <- runner.ApplyAll(ctx)
	}()

	select {
	case err := <-secondDone:
		t.Fatalf("second runner returned early: %v", err)
	case <-time.After(50 * time.Millisecond):
		// expected: still waiting for lock release
	}

	close(release)

	if err := <-done; err != nil {
		t.Fatalf("first runner err: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second runner err: %v", err)
	}
}

func slogDiscard() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}
