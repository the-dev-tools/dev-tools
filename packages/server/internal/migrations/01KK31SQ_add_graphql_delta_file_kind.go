package migrations

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/migrate"
)

// MigrationAddGraphQLDeltaFileKindID is the ULID for the GraphQL delta file kind migration.
const MigrationAddGraphQLDeltaFileKindID = "01KK31SQD0GFP92RZ3XMN5V6BQ"

// MigrationAddGraphQLDeltaFileKindChecksum is a stable hash of this migration.
const MigrationAddGraphQLDeltaFileKindChecksum = "sha256:add-graphql-delta-file-kind-v2"

func init() {
	if err := migrate.Register(migrate.Migration{
		ID:             MigrationAddGraphQLDeltaFileKindID,
		Checksum:       MigrationAddGraphQLDeltaFileKindChecksum,
		Description:    "GraphQL delta file kind — files table constraint now handled by websocket migration",
		Apply:          applyGraphQLDeltaFileKind,
		Validate:       validateGraphQLDeltaFileKind,
		RequiresBackup: false,
	}); err != nil {
		panic("failed to register GraphQL delta file kind migration: " + err.Error())
	}
}

func applyGraphQLDeltaFileKind(_ context.Context, _ *sql.Tx) error {
	// No-op: the files table CHECK constraint update (adding content_kind 6 and 7)
	// is now consolidated into migration 01KKFQT8 (WebSocket tables) to avoid
	// the later migration accidentally dropping content_kind=7 when it recreated
	// the files table with only (0..6).
	return nil
}

func validateGraphQLDeltaFileKind(_ context.Context, _ *sql.DB) error {
	// Validation deferred to 01KKFQT8 which owns the files table recreation.
	return nil
}
