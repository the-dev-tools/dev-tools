package migrate

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Status represents the state of a migration record.
type Status string

const (
	// StatusStarted marks an in-flight migration.
	StatusStarted Status = "started"
	// StatusFinished marks a successfully completed migration.
	StatusFinished Status = "finished"
)

// ErrChecksumMismatch is returned when the stored checksum differs from the
// caller provided checksum for a finished migration.
var ErrChecksumMismatch = errors.New("migrate: checksum mismatch for migration")

// Record models a row in schema_migrations.
type Record struct {
	ID         string
	Status     Status
	Checksum   string
	Attempts   int
	StartedAt  time.Time
	FinishedAt sql.NullTime
	LastError  sql.NullString
	CursorJSON sql.NullString
	BackupPath sql.NullString
}

// Cursor returns the stored cursor state if present.
func (r Record) Cursor() (CursorState, error) {
	if !r.CursorJSON.Valid || len(r.CursorJSON.String) == 0 {
		return nil, nil
	}

	var state CursorState
	if err := json.Unmarshal([]byte(r.CursorJSON.String), &state); err != nil {
		return nil, err
	}
	return state, nil
}

const createSchemaMigrationsTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    id TEXT PRIMARY KEY,
    status TEXT NOT NULL CHECK (status IN ('started', 'finished')),
    checksum TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    last_error TEXT,
    cursor JSON,
    backup_path TEXT
);
`

const createSchemaMigrationsStatusIdx = `
CREATE INDEX IF NOT EXISTS idx_schema_migrations_status ON schema_migrations(status);
`

// Store provides helpers for manipulating schema_migrations metadata.
type Store struct {
	db *sql.DB
}

// NewStore constructs a Store bound to the provided database handle.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// EnsureSchema ensures the metadata table and indexes exist.
func (s *Store) EnsureSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, createSchemaMigrationsTable); err != nil {
		return fmt.Errorf("migrate: creating schema_migrations table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, createSchemaMigrationsStatusIdx); err != nil {
		return fmt.Errorf("migrate: creating schema_migrations status index: %w", err)
	}
	return nil
}

// StartParams describes the data recorded when a migration begins.
type StartParams struct {
	ID         string
	Checksum   string
	StartedAt  time.Time
	BackupPath *string
}

// MarkStarted inserts or updates the metadata row for an in-progress migration.
// It increments the attempts counter, clears last_error, and records the latest
// backup path. If the existing record is finished with a different checksum, it
// returns ErrChecksumMismatch.
func (s *Store) MarkStarted(ctx context.Context, tx *sql.Tx, params StartParams) (Record, error) {
	existing, err := getRecord(ctx, tx, params.ID)
	if err != nil && err != sql.ErrNoRows {
		return Record{}, err
	}

	if err == sql.ErrNoRows {
		if err := insertStarted(ctx, tx, params); err != nil {
			return Record{}, err
		}
	} else {
		if existing.Status == StatusFinished && existing.Checksum != params.Checksum {
			return Record{}, fmt.Errorf("%w: stored=%s new=%s", ErrChecksumMismatch, existing.Checksum, params.Checksum)
		}
		if err := updateStarted(ctx, tx, params); err != nil {
			return Record{}, err
		}
	}

	return getRecord(ctx, tx, params.ID)
}

func insertStarted(ctx context.Context, tx *sql.Tx, params StartParams) error {
	res, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (id, status, checksum, attempts, started_at, backup_path, last_error)
         VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		params.ID,
		StatusStarted,
		params.Checksum,
		1,
		params.StartedAt.UTC(),
		nullableString(params.BackupPath),
	)
	if err != nil {
		return fmt.Errorf("migrate: insert started: %w", err)
	}
	return ensureRowsAffected(res, "insert started")
}

func updateStarted(ctx context.Context, tx *sql.Tx, params StartParams) error {
	res, err := tx.ExecContext(
		ctx,
		`UPDATE schema_migrations
         SET status = ?,
             checksum = ?,
             attempts = attempts + 1,
             started_at = ?,
             backup_path = ?,
             last_error = NULL
         WHERE id = ?`,
		StatusStarted,
		params.Checksum,
		params.StartedAt.UTC(),
		nullableString(params.BackupPath),
		params.ID,
	)
	if err != nil {
		return fmt.Errorf("migrate: update started: %w", err)
	}
	if err := ensureRowsAffected(res, "update started"); err != nil {
		return err
	}
	return nil
}

// FinishParams describes the data recorded when a migration completes.
type FinishParams struct {
	ID         string
	FinishedAt time.Time
}

// MarkFinished marks a migration as finished, clearing error/cursor fields.
func (s *Store) MarkFinished(ctx context.Context, tx *sql.Tx, params FinishParams) (Record, error) {
	res, err := tx.ExecContext(
		ctx,
		`UPDATE schema_migrations
         SET status = ?,
             finished_at = ?,
             last_error = NULL,
             cursor = NULL
         WHERE id = ?`,
		StatusFinished,
		params.FinishedAt.UTC(),
		params.ID,
	)
	if err != nil {
		return Record{}, fmt.Errorf("migrate: mark finished: %w", err)
	}
	if err := ensureRowsAffected(res, "mark finished"); err != nil {
		return Record{}, err
	}
	return getRecord(ctx, tx, params.ID)
}

// SetError stores the last error message for a migration.
type ErrorParams struct {
	ID        string
	LastError string
}

func (s *Store) SetError(ctx context.Context, tx *sql.Tx, params ErrorParams) error {
	res, err := tx.ExecContext(
		ctx,
		`UPDATE schema_migrations
         SET last_error = ?
         WHERE id = ?`,
		params.LastError,
		params.ID,
	)
	if err != nil {
		return fmt.Errorf("migrate: set error: %w", err)
	}
	if err := ensureRowsAffected(res, "set error"); err != nil {
		return err
	}
	return nil
}

// CursorParams captures cursor persistence details.
type CursorParams struct {
	ID     string
	Cursor CursorState
}

// SaveCursor persists resumable cursor state.
func (s *Store) SaveCursor(ctx context.Context, tx *sql.Tx, params CursorParams) error {
	var payload sql.NullString
	if params.Cursor != nil {
		data, err := json.Marshal(params.Cursor)
		if err != nil {
			return fmt.Errorf("migrate: marshal cursor: %w", err)
		}
		payload = sql.NullString{String: string(data), Valid: true}
	}

	res, err := tx.ExecContext(
		ctx,
		`UPDATE schema_migrations
         SET cursor = ?
         WHERE id = ?`,
		payload,
		params.ID,
	)
	if err != nil {
		return fmt.Errorf("migrate: save cursor: %w", err)
	}
	if err := ensureRowsAffected(res, "save cursor"); err != nil {
		return err
	}
	return nil
}

// LoadCursor fetches the cursor for a migration, if present.
func (s *Store) LoadCursor(ctx context.Context, tx *sql.Tx, id string) (CursorState, error) {
	rec, err := getRecord(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	return rec.Cursor()
}

// Get returns the metadata record for a migration by id.
func (s *Store) Get(ctx context.Context, tx *sql.Tx, id string) (Record, error) {
	return getRecord(ctx, tx, id)
}

// GetRecord fetches the metadata entry without requiring a transaction.
func (s *Store) GetRecord(ctx context.Context, id string) (Record, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, status, checksum, attempts, started_at, finished_at, last_error, cursor, backup_path
         FROM schema_migrations
         WHERE id = ?`,
		id,
	)
	return scanRecord(row)
}

func getRecord(ctx context.Context, tx *sql.Tx, id string) (Record, error) {
	row := tx.QueryRowContext(
		ctx,
		`SELECT id, status, checksum, attempts, started_at, finished_at, last_error, cursor, backup_path
         FROM schema_migrations
         WHERE id = ?`,
		id,
	)
	return scanRecord(row)
}

func nullableString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func ensureRowsAffected(res sql.Result, op string) error {
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("migrate: %s rows affected: %w", op, err)
	}
	if affected == 0 {
		return fmt.Errorf("migrate: %s touched no rows", op)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(row rowScanner) (Record, error) {
	var rec Record
	if err := row.Scan(
		&rec.ID,
		&rec.Status,
		&rec.Checksum,
		&rec.Attempts,
		&rec.StartedAt,
		&rec.FinishedAt,
		&rec.LastError,
		&rec.CursorJSON,
		&rec.BackupPath,
	); err != nil {
		return Record{}, err
	}
	return rec, nil
}
