package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddCloudLinkTableID is the ULID for the cloud_link table migration.
const MigrationAddCloudLinkTableID = "01M4NC48Y637C4R1H8DH05BG2Y"

// MigrationAddCloudLinkTableChecksum is a stable hash of this migration.
const MigrationAddCloudLinkTableChecksum = "sha256:add-cloud-link-table-v1"

// cloudLinkDDL creates the table that links Files rows to Stresseur Cloud repos and flow files.
var cloudLinkDDL = []string{
	`CREATE TABLE IF NOT EXISTS cloud_link (
		file_id BLOB NOT NULL PRIMARY KEY,
		workspace_id BLOB NOT NULL,
		kind TEXT NOT NULL,
		api_url TEXT NOT NULL,
		cloud_workspace_slug TEXT NOT NULL,
		repo_slug TEXT NOT NULL,
		path TEXT NOT NULL DEFAULT '',
		base_sha TEXT NOT NULL DEFAULT '',
		local_dir TEXT NOT NULL DEFAULT '',
		dirty BOOLEAN NOT NULL DEFAULT FALSE,
		source_text TEXT NOT NULL DEFAULT '',
		clean_yaml TEXT NOT NULL DEFAULT '',
		updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
		CHECK (length (file_id) == 16),
		CHECK (kind IN ('repo', 'flow_file')),
		CHECK ((kind = 'repo' AND path = '') OR (kind = 'flow_file' AND path != '')),
		FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
		FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
	)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS cloud_link_target_idx ON cloud_link (
		workspace_id, api_url, cloud_workspace_slug, repo_slug, path
	)`,
}

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddCloudLinkTableID,
		Checksum:       MigrationAddCloudLinkTableChecksum,
		Description:    "Add cloud_link table for Stresseur Cloud repos and flow files",
		Apply:          applyCloudLinkTable,
		Validate:       validateCloudLinkTable,
		RequiresBackup: false,
	}); err != nil {
		panic("failed to register cloud_link table migration: " + err.Error())
	}
}

func applyCloudLinkTable(ctx context.Context, tx *sql.Tx) error {
	for _, ddl := range cloudLinkDDL {
		if _, err := tx.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("create cloud_link: %w", err)
		}
	}
	return nil
}

func validateCloudLinkTable(ctx context.Context, db *sql.DB) error {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='cloud_link'`).Scan(&name); err != nil {
		return fmt.Errorf("cloud_link table not found: %w", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='index' AND name='cloud_link_target_idx'`).Scan(&name); err != nil {
		return fmt.Errorf("cloud_link_target_idx index not found: %w", err)
	}
	return nil
}
