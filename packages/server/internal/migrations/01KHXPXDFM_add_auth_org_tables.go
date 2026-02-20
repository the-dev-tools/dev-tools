package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

const MigrationAddAuthOrgTablesID = "01KHXPXDFMZ29DZ5S2SEP7ZQ1Y"

const MigrationAddAuthOrgTablesChecksum = "sha256:add-auth-org-tables-v1"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:          MigrationAddAuthOrgTablesID,
		Checksum:    MigrationAddAuthOrgTablesChecksum,
		Description: "Add BetterAuth organization, member, and invitation tables",
		Apply:       applyAddAuthOrgTables,
		Validate:    validateAddAuthOrgTables,
	}); err != nil {
		panic("failed to register auth org tables migration: " + err.Error())
	}
}

func applyAddAuthOrgTables(ctx context.Context, tx *sql.Tx) error {
	tables := []struct {
		name string
		ddl  string
	}{
		{
			name: "auth_organization",
			ddl: `CREATE TABLE IF NOT EXISTS auth_organization (
				id BLOB NOT NULL PRIMARY KEY,
				name TEXT NOT NULL,
				slug TEXT UNIQUE,
				logo TEXT,
				metadata TEXT,
				created_at INTEGER NOT NULL,
				CHECK (length(id) = 16)
			)`,
		},
		{
			name: "auth_member",
			ddl: `CREATE TABLE IF NOT EXISTS auth_member (
				id BLOB NOT NULL PRIMARY KEY,
				user_id BLOB NOT NULL,
				organization_id BLOB NOT NULL,
				role TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				CHECK (length(id) = 16),
				FOREIGN KEY (user_id) REFERENCES auth_user (id) ON DELETE CASCADE,
				FOREIGN KEY (organization_id) REFERENCES auth_organization (id) ON DELETE CASCADE
			)`,
		},
		{
			name: "auth_invitation",
			ddl: `CREATE TABLE IF NOT EXISTS auth_invitation (
				id BLOB NOT NULL PRIMARY KEY,
				email TEXT NOT NULL,
				inviter_id BLOB NOT NULL,
				organization_id BLOB NOT NULL,
				role TEXT NOT NULL,
				status TEXT NOT NULL,
				created_at INTEGER NOT NULL,
				expires_at INTEGER NOT NULL,
				CHECK (length(id) = 16),
				FOREIGN KEY (inviter_id) REFERENCES auth_user (id) ON DELETE CASCADE,
				FOREIGN KEY (organization_id) REFERENCES auth_organization (id) ON DELETE CASCADE
			)`,
		},
	}

	for _, tbl := range tables {
		var count int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name = ?
		`, tbl.name).Scan(&count); err != nil {
			return fmt.Errorf("check %s table: %w", tbl.name, err)
		}
		if count > 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, tbl.ddl); err != nil {
			return fmt.Errorf("create %s table: %w", tbl.name, err)
		}
	}

	return nil
}

func validateAddAuthOrgTables(ctx context.Context, db *sql.DB) error {
	for _, name := range []string{"auth_organization", "auth_member", "auth_invitation"} {
		var count int
		if err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM sqlite_master
			WHERE type = 'table' AND name = ?
		`, name).Scan(&count); err != nil {
			return fmt.Errorf("check %s table: %w", name, err)
		}
		if count == 0 {
			return fmt.Errorf("%s table not found", name)
		}
	}
	return nil
}
