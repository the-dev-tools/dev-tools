package sqlc

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// CreateLocalTables creates all tables defined in schema/*.sql
// This is used for testing and local development
func CreateLocalTables(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	entries, err := schemaFS.ReadDir("schema")
	if err != nil {
		return fmt.Errorf("failed to read schema directory: %w", err)
	}

	// Ensure files are sorted (ReadDir normally returns them sorted, but being explicit is safe)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		content, err := schemaFS.ReadFile("schema/" + entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read schema file %s: %w", entry.Name(), err)
		}

		// Execute the schema file
		_, err = db.ExecContext(ctx, string(content))
		if err != nil {
			return fmt.Errorf("failed to execute schema file %s: %w", entry.Name(), err)
		}
	}

	return nil
}
