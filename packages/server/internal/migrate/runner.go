package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Config controls runner behaviour.
type Config struct {
	DatabasePath  string
	BackupDir     string
	RetainBackups int
	BusyTimeout   time.Duration
	ForceBackup   bool
}

// Runner applies registered migrations against the database.
type Runner struct {
	db      *sql.DB
	store   *Store
	logger  *slog.Logger
	cfg     Config
	backup  *BackupManager
	nowFunc func() time.Time
}

// NewRunner constructs a Runner.
func NewRunner(db *sql.DB, cfg Config, logger *slog.Logger) (*Runner, error) {
	if db == nil {
		return nil, errors.New("migrate: db handle is required")
	}
	if cfg.DatabasePath == "" {
		return nil, errors.New("migrate: database path is required in config")
	}
	if cfg.RetainBackups <= 0 {
		cfg.RetainBackups = 3
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	store := NewStore(db)
	b := &BackupManager{
		DatabasePath: cfg.DatabasePath,
		BackupDir:    cfg.BackupDir,
		Retain:       cfg.RetainBackups,
	}

	return &Runner{
		db:      db,
		store:   store,
		logger:  logger,
		cfg:     cfg,
		backup:  b,
		nowFunc: time.Now,
	}, nil
}

// ApplyAll runs every registered migration in order.
func (r *Runner) ApplyAll(ctx context.Context) error {
	return r.apply(ctx, "")
}

// ApplyTo runs migrations up to and including targetID (if provided).
func (r *Runner) ApplyTo(ctx context.Context, targetID string) error {
	return r.apply(ctx, targetID)
}

func (r *Runner) apply(ctx context.Context, targetID string) error {
	unlock := lockProcess()
	defer unlock()

	if err := r.store.EnsureSchema(ctx); err != nil {
		return err
	}
	if err := r.prepareConnection(ctx); err != nil {
		return err
	}

	migrations := List()
	for _, mig := range migrations {
		if targetID != "" && mig.ID > targetID {
			break
		}
		checksum := mig.Checksum

		rec, err := r.store.GetRecord(ctx, mig.ID)
		if err == nil {
			if rec.Status == StatusFinished {
				if rec.Checksum != checksum {
					return fmt.Errorf("%w: stored=%s new=%s", ErrChecksumMismatch, rec.Checksum, checksum)
				}
				continue
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		if err := r.runMigration(ctx, mig); err != nil {
			return err
		}
	}

	return nil
}

var processMutex sync.Mutex

func lockProcess() func() {
	processMutex.Lock()
	return func() {
		processMutex.Unlock()
	}
}

func (r *Runner) runMigration(ctx context.Context, mig Migration) error {
	if mig.Precheck != nil {
		if err := mig.Precheck(ctx, r.db); err != nil {
			return fmt.Errorf("migrate: precheck %s: %w", mig.ID, err)
		}
	}

	var backupPath *string
	if r.cfg.ForceBackup || mig.RequiresBackup {
		path, err := r.backup.Create(ctx, mig.ID, r.nowFunc())
		if err != nil {
			return fmt.Errorf("migrate: backup %s: %w", mig.ID, err)
		}
		backupPath = &path
	}

	startedAt := r.nowFunc()
	metaTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate: begin metadata tx for %s: %w", mig.ID, err)
	}
	defer rollbackIgnore(metaTx)

	record, err := r.store.MarkStarted(ctx, metaTx, StartParams{ID: mig.ID, Checksum: mig.Checksum, StartedAt: startedAt, BackupPath: backupPath})
	if err != nil {
		return err
	}
	state, err := record.Cursor()
	if err != nil {
		return err
	}
	if err := metaTx.Commit(); err != nil {
		return fmt.Errorf("migrate: commit metadata start %s: %w", mig.ID, err)
	}

	r.logger.InfoContext(ctx, "migration started",
		slog.String("migration_id", mig.ID),
		slog.Int("attempt", record.Attempts),
	)
	if backupPath != nil {
		r.logger.InfoContext(ctx, "migration backup created",
			slog.String("migration_id", mig.ID),
			slog.String("backup_path", *backupPath),
		)
	}

	execStart := r.nowFunc()

	tx, err := r.beginTx(ctx)
	if err != nil {
		return fmt.Errorf("migrate: begin tx for %s: %w", mig.ID, err)
	}
	defer rollbackIgnore(tx)

	applyCtx := withCursorManager(ctx, cursorManager{state: state, store: r.store, id: mig.ID})

	if err := mig.Apply(applyCtx, tx); err != nil {
		rollbackIgnore(tx)
		_ = r.recordPostCommitError(ctx, mig.ID, err)
		r.logger.ErrorContext(ctx, "migration apply failed",
			slog.String("migration_id", mig.ID),
			slog.Int("attempt", record.Attempts),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("migrate: apply %s: %w", mig.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate: commit %s: %w", mig.ID, err)
	}

	if mig.Validate != nil {
		if err := mig.Validate(ctx, r.db); err != nil {
			_ = r.recordPostCommitError(ctx, mig.ID, err)
			r.logger.ErrorContext(ctx, "migration validate failed",
				slog.String("migration_id", mig.ID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("migrate: validate %s: %w", mig.ID, err)
		}
	}

	if mig.After != nil {
		if err := mig.After(ctx, r.db); err != nil {
			_ = r.recordPostCommitError(ctx, mig.ID, err)
			r.logger.ErrorContext(ctx, "migration after hook failed",
				slog.String("migration_id", mig.ID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("migrate: after hook %s: %w", mig.ID, err)
		}
	}

	if mig.RequiresCheckpoint {
		if err := RunCheckpoint(ctx, r.db); err != nil {
			_ = r.recordPostCommitError(ctx, mig.ID, err)
			r.logger.ErrorContext(ctx, "migration checkpoint failed",
				slog.String("migration_id", mig.ID),
				slog.String("error", err.Error()),
			)
			return fmt.Errorf("migrate: checkpoint %s: %w", mig.ID, err)
		}
	}

	finishTx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate: begin finish tx %s: %w", mig.ID, err)
	}
	defer rollbackIgnore(finishTx)

	finishRec, err := r.store.MarkFinished(ctx, finishTx, FinishParams{ID: mig.ID, FinishedAt: r.nowFunc()})
	if err != nil {
		return err
	}
	if err := finishTx.Commit(); err != nil {
		return fmt.Errorf("migrate: commit finish %s: %w", mig.ID, err)
	}

	if backupPath != nil {
		if err := r.backup.Trim(); err != nil {
			r.logger.WarnContext(ctx, "failed to trim backups", slog.String("error", err.Error()))
		}
	}

	r.logger.InfoContext(ctx, "migration applied",
		slog.String("migration_id", mig.ID),
		slog.Int("attempt", finishRec.Attempts),
		slog.Duration("duration", r.nowFunc().Sub(execStart)),
	)
	return nil
}

func (r *Runner) prepareConnection(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return fmt.Errorf("migrate: enable foreign_keys: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		return fmt.Errorf("migrate: set journal_mode WAL: %w", err)
	}
	if r.cfg.BusyTimeout > 0 {
		if _, err := r.db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout=%d", int(r.cfg.BusyTimeout.Milliseconds()))); err != nil {
			return fmt.Errorf("migrate: set busy_timeout: %w", err)
		}
	}
	return nil
}

func (r *Runner) beginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

func (r *Runner) recordPostCommitError(ctx context.Context, id string, cause error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollbackIgnore(tx)

	if err := r.store.SetError(ctx, tx, ErrorParams{ID: id, LastError: cause.Error()}); err != nil {
		return err
	}
	return tx.Commit()
}

func rollbackIgnore(tx *sql.Tx) {
	if tx == nil {
		return
	}
	_ = tx.Rollback()
}
