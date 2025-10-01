# Go Function Migration Harness

The desktop application now uses a Go-based migration runner (`packages/server/internal/migrate`) for the bundled SQLite database. This runner executes ordered Go functions during application start, applies WAL/foreign-key safety pragmas, and keeps crash-resilient metadata in a `schema_migrations` table.

## Authoring Migrations

- Call `migrate.Register` from package init or an explicit boot hook.
- Required fields: ULID `ID`, deterministic `Checksum`, and an `Apply` function that mutates the DB using the provided transaction.
- Optional hooks:
  - `Precheck`: validate environment before opening a transaction (disk space, feature flags, etc.).
  - `Validate`: post-commit assertions; failures keep the migration in `started` status.
  - `After`: non-transactional work; failures are recorded like validation errors.
  - `RequiresBackup`: force a physical backup before applying (`Runner` needs `BackupDir`).
  - `RequiresCheckpoint`: trigger `PRAGMA wal_checkpoint(TRUNCATE)` after success.
  - `SaveCursorState`/`CursorStateFromContext`: persist and resume chunked progress.

`Checksum` should be a stable hash string (e.g., SHA256 of the migration source). Changing the migration logic requires changing both the code and the checksum.

```go
migrate.Register(migrate.Migration{
    ID:       idwrap.NewNow().String(),
    Checksum: "sha256:...",
    Apply: func(ctx context.Context, tx *sql.Tx) error {
        _, err := tx.ExecContext(ctx, "ALTER TABLE foo ADD COLUMN bar TEXT")
        return err
    },
})
```

## Running Migrations

Create a `Runner` early in startup and call `ApplyAll` before other services touch the DB.

```go
cfg := migrate.Config{
    DatabasePath:  dbPath,
    BackupDir:     filepath.Join(dataDir, "migrations", "backups"),
    RetainBackups: 3,
    BusyTimeout:   5 * time.Second,
}
runner, _ := migrate.NewRunner(db, cfg, logger)
if err := runner.ApplyAll(ctx); err != nil {
    // surface to UI / fail fast
}
```

`ApplyTo` accepts an optional target ID for partial upgrades. The runner serializes concurrent calls inside the process, ensures WAL + foreign keys are enabled, and records attempts/last errors for diagnostics.

## Metadata & Recovery

`schema_migrations` now tracks:

- `status` (`started`/`finished`)
- `checksum`
- `attempts` counter and timestamps
- `last_error`
- `cursor` JSON payload for resumable migrations
- `backup_path` pointing at the staged copy

When a migration fails (precheck, apply, validate, checkpoint, after), the runner stores the error, leaves the migration in `started`, and surfaces the failure to the caller. Next boot will re-run the migration after incrementing `attempts`.

## Backups & Checkpoints

For migrations flagged with `RequiresBackup` (or when `Config.ForceBackup` is true), the runner copies the SQLite database plus WAL/SHM companions into a timestamped directory under `BackupDir`. Successful runs trim backups to `RetainBackups`; failures leave the copy in place. `RunCheckpoint` issues `PRAGMA wal_checkpoint(TRUNCATE)` to keep WAL growth in check.

## Cursor Utilities

Inside `Apply`, call:

```go
state, ok := migrate.CursorStateFromContext(ctx)
if ok {
    // resume using state
}
if err := migrate.SaveCursorState(ctx, tx, migrate.CursorState{"offset": float64(n)}); err != nil {
    return err
}
```

State is stored in the metadata table and cleared automatically once `MarkFinished` succeeds.

## Testing

Use `sqlitemem.NewSQLiteMem` with the helper tests under `packages/server/internal/migrate`. The package ships table-driven tests for metadata, backups, cursor replay, and concurrency serialization. When adding migrations, create targeted tests that exercise `Apply`, `Validate`, and resume behaviour.

## Diagnostics

`Runner` emits structured slog events:

- `migration started`
- `migration backup created`
- `migration applied` (with duration/attempt)
- `migration apply/validate/after/checkpoint failed`

These logs feed desktop diagnostics and can be extended with metrics later.

## Follow-up

- Decide how to derive canonical checksums (build manifest vs. literal hash).
- Wire the runner into the desktop bootstrap sequence (feature flag if needed).
- Extend telemetry sinks if additional dashboards are required.
