package gen

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/oklog/ulid/v2"
	"the-dev-tools/server/pkg/idwrap"
)

// BenchmarkHTTPStreamingQueries benchmarks new HTTP streaming queries
// These tests validate Priority 1 streaming optimizations

func BenchmarkHTTPStreamingQueries(b *testing.B) {
	// Setup in-memory database for benchmarking
	db := setupBenchmarkDB(b)
	defer db.Close()

	// Insert test data
	workspaceID := insertTestWorkspace(b, db)
	httpIDs := insertTestHTTPData(b, db, workspaceID, 1000) // 1000 HTTP records
	insertTestChildData(b, db, httpIDs, 5)                  // 5 child records per HTTP

	b.ResetTimer()

	b.Run("GetHTTPSnapshotPage", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			queries := New(db)
			_, err := queries.GetHTTPSnapshotPage(context.Background(), GetHTTPSnapshotPageParams{
				WorkspaceID: workspaceID,
				UpdatedAt:   time.Now().Unix(),
				Limit:       50,
			})
			if err != nil {
				b.Fatalf("GetHTTPSnapshotPage failed: %v", err)
			}
		}
	})

	b.Run("GetHTTPIncrementalUpdates", func(b *testing.B) {
		queries := New(db)
		cutoffTime := time.Now().Add(-time.Hour).Unix()

		for i := 0; i < b.N; i++ {
			_, err := queries.GetHTTPIncrementalUpdates(context.Background(), GetHTTPIncrementalUpdatesParams{
				WorkspaceID: workspaceID,
				UpdatedAt:   cutoffTime,
				UpdatedAt_2: time.Now().Unix(),
			})
			if err != nil {
				b.Fatalf("GetHTTPIncrementalUpdates failed: %v", err)
			}
		}
	})

	b.Run("GetHTTPHeadersStreaming", func(b *testing.B) {
		queries := New(db)

		for i := 0; i < b.N; i++ {
			_, err := queries.GetHTTPHeadersStreaming(context.Background(), GetHTTPHeadersStreamingParams{
				HttpIds:   httpIDs[:100], // Test with 100 HTTP IDs
				UpdatedAt: time.Now().Unix(),
			})
			if err != nil {
				b.Fatalf("GetHTTPHeadersStreaming failed: %v", err)
			}
		}
	})

	b.Run("GetHTTPStreamingMetrics", func(b *testing.B) {
		queries := New(db)
		since := time.Now().Add(-24 * time.Hour).Unix()

		for i := 0; i < b.N; i++ {
			_, err := queries.GetHTTPStreamingMetrics(context.Background(), GetHTTPStreamingMetricsParams{
				UpdatedAt:   since,
				WorkspaceID: workspaceID,
			})
			if err != nil {
				b.Fatalf("GetHTTPStreamingMetrics failed: %v", err)
			}
		}
	})
}

// BenchmarkDeltaResolution benchmarks delta resolution performance
func BenchmarkDeltaResolution(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer db.Close()

	workspaceID := insertTestWorkspace(b, db)
	parentHTTPID := insertTestHTTPRecord(b, db, workspaceID, "parent")
	deltaIDs := insertTestDeltas(b, db, parentHTTPID, 10) // 10 delta records
	_ = deltaIDs                                          // Use the delta IDs to avoid unused variable warning

	b.ResetTimer()

	queries := New(db)
	cutoffTime := time.Now().Unix()

	b.Run("ResolveHTTPWithDeltas", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := queries.ResolveHTTPWithDeltas(context.Background(), ResolveHTTPWithDeltasParams{
				ID:        parentHTTPID,
				UpdatedAt: cutoffTime,
			})
			if err != nil {
				b.Fatalf("ResolveHTTPWithDeltas failed: %v", err)
			}
		}
	})

	b.Run("GetHTTPDeltasSince", func(b *testing.B) {
		parentIDs := []*idwrap.IDWrap{&parentHTTPID}
		for i := 0; i < b.N; i++ {
			_, err := queries.GetHTTPDeltasSince(context.Background(), GetHTTPDeltasSinceParams{
				ParentIds:   parentIDs,
				UpdatedAt:   cutoffTime,
				UpdatedAt_2: cutoffTime,
			})
			if err != nil {
				b.Fatalf("GetHTTPDeltasSince failed: %v", err)
			}
		}
	})
}

// BenchmarkConcurrentStreaming tests concurrent access patterns
func BenchmarkConcurrentStreaming(b *testing.B) {
	db := setupBenchmarkDB(b)
	defer db.Close()

	workspaceID := insertTestWorkspace(b, db)
	httpIDs := insertTestHTTPData(b, db, workspaceID, 500)
	_ = httpIDs // Use the HTTP IDs to avoid unused variable warning

	b.ResetTimer()

	b.Run("ConcurrentSnapshotQueries", func(b *testing.B) {
		// Test concurrent access with separate connections
		b.RunParallel(func(pb *testing.PB) {
			// Create a new connection for each parallel worker
			workerDB, err := sql.Open("sqlite3", ":memory:")
			if err != nil {
				b.Errorf("Failed to open worker database: %v", err)
				return
			}
			defer workerDB.Close()

			// Load schema for this worker
			schema := loadSchema(b)
			_, err = workerDB.Exec(schema)
			if err != nil {
				b.Errorf("Failed to load schema for worker: %v", err)
				return
			}

			// Insert test data for this worker
			workerWorkspaceID := insertTestWorkspace(b, workerDB)
			workerHTTPIDs := insertTestHTTPData(b, workerDB, workerWorkspaceID, 100)

			queries := New(workerDB)
			for pb.Next() {
				_, err := queries.GetHTTPSnapshotPage(context.Background(), GetHTTPSnapshotPageParams{
					WorkspaceID: workerWorkspaceID,
					UpdatedAt:   time.Now().Unix(),
					Limit:       25,
				})
				if err != nil {
					b.Errorf("Concurrent snapshot query failed: %v", err)
				}
			}
			_ = workerHTTPIDs
		})
	})

	b.Run("ConcurrentIncrementalQueries", func(b *testing.B) {
		// Test concurrent access with separate connections
		b.RunParallel(func(pb *testing.PB) {
			// Create a new connection for each parallel worker
			workerDB, err := sql.Open("sqlite3", ":memory:")
			if err != nil {
				b.Errorf("Failed to open worker database: %v", err)
				return
			}
			defer workerDB.Close()

			// Load schema for this worker
			schema := loadSchema(b)
			_, err = workerDB.Exec(schema)
			if err != nil {
				b.Errorf("Failed to load schema for worker: %v", err)
				return
			}

			// Insert test data for this worker
			workerWorkspaceID := insertTestWorkspace(b, workerDB)
			workerHTTPIDs := insertTestHTTPData(b, workerDB, workerWorkspaceID, 100)

			queries := New(workerDB)
			cutoffTime := time.Now().Add(-time.Hour).Unix()
			for pb.Next() {
				_, err := queries.GetHTTPIncrementalUpdates(context.Background(), GetHTTPIncrementalUpdatesParams{
					WorkspaceID: workerWorkspaceID,
					UpdatedAt:   cutoffTime,
					UpdatedAt_2: time.Now().Unix(),
				})
				if err != nil {
					b.Errorf("Concurrent incremental query failed: %v", err)
				}
			}
			_ = workerHTTPIDs
		})
	})
}

// Helper functions for benchmark setup

func setupBenchmarkDB(b *testing.B) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("Failed to open in-memory database: %v", err)
	}

	// Load schema
	schema := loadSchema(b)
	_, err = db.Exec(schema)
	if err != nil {
		b.Fatalf("Failed to load schema: %v", err)
	}

	return db
}

func insertTestWorkspace(b *testing.B, db *sql.DB) idwrap.IDWrap {
	workspaceID := idwrap.New(ulid.Make())
	_, err := db.Exec(`
		INSERT INTO workspaces (id, name, updated, collection_count, flow_count) 
		VALUES (?, ?, ?, ?, ?)`,
		workspaceID, "test-workspace", time.Now().Unix(), 0, 0)
	if err != nil {
		b.Fatalf("Failed to insert test workspace: %v", err)
	}
	return workspaceID
}

func insertTestHTTPRecord(b *testing.B, db *sql.DB, workspaceID idwrap.IDWrap, name string) idwrap.IDWrap {
	httpID := idwrap.New(ulid.Make())
	_, err := db.Exec(`
		INSERT INTO http (id, workspace_id, name, url, method, description, is_delta) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		httpID, workspaceID, name, "https://api.example.com/test", "GET", "Test HTTP record", false)
	if err != nil {
		b.Fatalf("Failed to insert test HTTP record: %v", err)
	}
	return httpID
}

func insertTestHTTPData(b *testing.B, db *sql.DB, workspaceID idwrap.IDWrap, count int) []idwrap.IDWrap {
	httpIDs := make([]idwrap.IDWrap, count)

	for i := 0; i < count; i++ {
		httpID := idwrap.New(ulid.Make())
		httpIDs[i] = httpID

		_, err := db.Exec(`
			INSERT INTO http (id, workspace_id, name, url, method, description, is_delta, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			httpID, workspaceID,
			"test-http-"+string(rune(i)),
			"https://api.example.com/test",
			"GET",
			"Test HTTP record",
			false,
			time.Now().Add(-time.Duration(i)*time.Minute).Unix())
		if err != nil {
			b.Fatalf("Failed to insert test HTTP data: %v", err)
		}
	}

	return httpIDs
}

func insertTestDeltas(b *testing.B, db *sql.DB, parentHTTPID idwrap.IDWrap, count int) []idwrap.IDWrap {
	deltaIDs := make([]idwrap.IDWrap, count)

	for i := 0; i < count; i++ {
		deltaID := idwrap.New(ulid.Make())
		deltaIDs[i] = deltaID

		_, err := db.Exec(`
			INSERT INTO http (id, workspace_id, parent_http_id, name, url, method, description, is_delta, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			deltaID, parentHTTPID, parentHTTPID,
			"delta-http-"+string(rune(i)),
			"https://api.example.com/delta",
			"POST",
			"Delta HTTP record",
			true,
			time.Now().Add(-time.Duration(i)*time.Minute).Unix())
		if err != nil {
			b.Fatalf("Failed to insert test delta data: %v", err)
		}
	}

	return deltaIDs
}

func insertTestChildData(b *testing.B, db *sql.DB, httpIDs []idwrap.IDWrap, childCount int) {
	for _, httpID := range httpIDs {
		for i := 0; i < childCount; i++ {
			// Insert headers
			headerID := idwrap.New(ulid.Make())
			_, err := db.Exec(`
				INSERT INTO http_header (id, http_id, header_key, header_value, description, enabled, updated_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				headerID, httpID, "X-Test-Header", "test-value", "Test header", true, time.Now().Unix())
			if err != nil {
				b.Fatalf("Failed to insert test header data: %v", err)
			}

			// Insert search params
			paramID := idwrap.New(ulid.Make())
			_, err = db.Exec(`
				INSERT INTO http_search_param (id, http_id, param_key, param_value, description, enabled, updated_at) 
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				paramID, httpID, "test_param", "test_value", "Test param", true, time.Now().Unix())
			if err != nil {
				b.Fatalf("Failed to insert test search param data: %v", err)
			}
		}
	}
}

func loadSchema(b *testing.B) string {
	// This would typically load from schema.sql file
	// For benchmark purposes, we'll create a minimal schema
	return `
	CREATE TABLE workspaces (
		id BLOB NOT NULL PRIMARY KEY,
		name TEXT NOT NULL,
		updated BIGINT NOT NULL DEFAULT (unixepoch()),
		collection_count INT NOT NULL DEFAULT 0,
		flow_count INT NOT NULL DEFAULT 0
	);

	CREATE TABLE http (
		id BLOB NOT NULL PRIMARY KEY,
		workspace_id BLOB NOT NULL,
		folder_id BLOB,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		method TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		parent_http_id BLOB DEFAULT NULL,
		is_delta BOOLEAN NOT NULL DEFAULT FALSE,
		delta_name TEXT NULL,
		delta_url TEXT NULL,
		delta_method TEXT NULL,
		delta_description TEXT NULL,
		created_at BIGINT NOT NULL DEFAULT (unixepoch()),
		updated_at BIGINT NOT NULL DEFAULT (unixepoch())
	);

	CREATE TABLE http_header (
		id BLOB NOT NULL PRIMARY KEY,
		http_id BLOB NOT NULL,
		header_key TEXT NOT NULL,
		header_value TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		enabled BOOLEAN NOT NULL DEFAULT TRUE,
		parent_header_id BLOB DEFAULT NULL,
		is_delta BOOLEAN NOT NULL DEFAULT FALSE,
		delta_header_key TEXT NULL,
		delta_header_value TEXT NULL,
		delta_description TEXT NULL,
		delta_enabled BOOLEAN NULL,
		prev BLOB DEFAULT NULL,
		next BLOB DEFAULT NULL,
		created_at BIGINT NOT NULL DEFAULT (unixepoch()),
		updated_at BIGINT NOT NULL DEFAULT (unixepoch())
	);

	CREATE TABLE http_search_param (
		id BLOB NOT NULL PRIMARY KEY,
		http_id BLOB NOT NULL,
		param_key TEXT NOT NULL,
		param_value TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		enabled BOOLEAN NOT NULL DEFAULT TRUE,
		parent_search_param_id BLOB DEFAULT NULL,
		is_delta BOOLEAN NOT NULL DEFAULT FALSE,
		delta_param_key TEXT NULL,
		delta_param_value TEXT NULL,
		delta_description TEXT NULL,
		delta_enabled BOOLEAN NULL,
		prev BLOB DEFAULT NULL,
		next BLOB DEFAULT NULL,
		created_at BIGINT NOT NULL DEFAULT (unixepoch()),
		updated_at BIGINT NOT NULL DEFAULT (unixepoch())
	);

	-- Add streaming performance indexes
	CREATE INDEX http_workspace_streaming_idx ON http (workspace_id, updated_at DESC);
	CREATE INDEX http_delta_resolution_idx ON http (parent_http_id, is_delta, updated_at DESC);
	CREATE INDEX http_workspace_method_streaming_idx ON http (workspace_id, method, updated_at DESC);
	CREATE INDEX http_active_streaming_idx ON http (workspace_id, updated_at DESC) WHERE is_delta = FALSE;
	CREATE INDEX http_header_streaming_idx ON http_header (http_id, enabled, updated_at DESC) WHERE enabled = TRUE;
	CREATE INDEX http_search_param_streaming_idx ON http_search_param (http_id, enabled, updated_at DESC) WHERE enabled = TRUE;
	`
}
