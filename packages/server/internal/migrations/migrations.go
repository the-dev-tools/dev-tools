// Package migrations registers all database migrations for the application.
// Migrations are executed in ULID order during application startup.
//
// To add a new migration:
//  1. Create a new file named `ULID_description.go` (e.g., `01JFGH12ABC_add_branch_table.go`)
//  2. Generate a ULID using idwrap.NewNow().String()
//  3. Implement the migration following the pattern in existing files
//  4. Import the migration file in this package (side-effect import via init)
//
// Migration checksums should be stable hashes of the migration logic.
// If you change migration code, you MUST change its checksum.
package migrations

import (
	"context"
	"database/sql"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// Config holds configuration for running migrations.
type Config struct {
	// DatabasePath is the absolute path to the SQLite database file.
	DatabasePath string
	// DataDir is the base data directory (backups stored under DataDir/migrations/backups).
	DataDir string
	// Logger for migration events.
	Logger *slog.Logger
}

// Run executes all registered migrations against the database.
// This should be called early in application startup, before services access the DB.
func Run(ctx context.Context, db *sql.DB, cfg Config) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	backupDir := filepath.Join(cfg.DataDir, "migrations", "backups")

	runnerCfg := migrate.Config{
		DatabasePath:  cfg.DatabasePath,
		BackupDir:     backupDir,
		RetainBackups: 3,
		BusyTimeout:   10 * time.Second,
	}

	runner, err := migrate.NewRunner(db, runnerCfg, cfg.Logger)
	if err != nil {
		return err
	}

	return runner.ApplyAll(ctx)
}

// RunTo executes migrations up to and including the specified migration ID.
// Useful for testing specific migration states.
func RunTo(ctx context.Context, db *sql.DB, cfg Config, targetID string) error {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	backupDir := filepath.Join(cfg.DataDir, "migrations", "backups")

	runnerCfg := migrate.Config{
		DatabasePath:  cfg.DatabasePath,
		BackupDir:     backupDir,
		RetainBackups: 3,
		BusyTimeout:   10 * time.Second,
	}

	runner, err := migrate.NewRunner(db, runnerCfg, cfg.Logger)
	if err != nil {
		return err
	}

	return runner.ApplyTo(ctx, targetID)
}
