package devtoolsdb

import (
	"context"
	"database/sql"
	"log"
	"testing"
	"time"

	gen "github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	_ "modernc.org/sqlite"
)

// TestHTTPChildEntityVerification verifies that all HTTP child entity tables work correctly
func TestHTTPChildEntityVerification(t *testing.T) {
	ctx := context.Background()

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Read and execute schema
	schema := `
-- Create the essential tables for testing
CREATE TABLE users (
  id BLOB NOT NULL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB,
  provider_type INT8 NOT NULL DEFAULT 0,
  provider_id TEXT,
  status INT8 NOT NULL DEFAULT 0,
  UNIQUE (provider_type, provider_id)
);

CREATE TABLE workspaces (
  id BLOB NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  updated BIGINT NOT NULL DEFAULT (unixepoch()),
  collection_count INT NOT NULL DEFAULT 0,
  flow_count INT NOT NULL DEFAULT 0,
  active_env BLOB,
  global_env BLOB,
  display_order REAL NOT NULL DEFAULT 0
);

CREATE TABLE files (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  folder_id BLOB,
  content_id BLOB,
  content_kind INT8 NOT NULL DEFAULT 0,
  name TEXT NOT NULL,
  display_order REAL NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  CHECK (length (id) == 16),
  CHECK (content_kind IN (0, 1, 2)),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL
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
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (folder_id) REFERENCES files (id) ON DELETE SET NULL,
  FOREIGN KEY (parent_http_id) REFERENCES http (id) ON DELETE CASCADE,
  CHECK (is_delta = FALSE OR parent_http_id IS NOT NULL)
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
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_header_id) REFERENCES http_header (id) ON DELETE CASCADE,
  FOREIGN KEY (prev) REFERENCES http_header (id) ON DELETE SET NULL,
  FOREIGN KEY (next) REFERENCES http_header (id) ON DELETE SET NULL,
  CHECK (is_delta = FALSE OR parent_header_id IS NOT NULL)
);

CREATE TABLE http_search_param (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  parent_http_search_param_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_search_param_id) REFERENCES http_search_param (id) ON DELETE CASCADE
);

CREATE TABLE http_body_form (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  parent_http_body_form_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_body_form_id) REFERENCES http_body_form (id) ON DELETE CASCADE
);

CREATE TABLE http_body_urlencoded (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  parent_http_body_urlencoded_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_body_urlencoded_id) REFERENCES http_body_urlencoded (id) ON DELETE CASCADE,
  CHECK (is_delta = FALSE OR parent_http_body_urlencoded_id IS NOT NULL)
);

CREATE TABLE http_assert (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL DEFAULT '',
  "order" REAL NOT NULL DEFAULT 0,
  parent_http_assert_id BLOB,
  is_delta BOOLEAN NOT NULL DEFAULT FALSE,
  delta_key TEXT,
  delta_value TEXT,
  delta_enabled BOOLEAN,
  delta_description TEXT,
  delta_order REAL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  updated_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_http_assert_id) REFERENCES http_assert (id) ON DELETE CASCADE,
  CHECK (is_delta = FALSE OR parent_http_assert_id IS NOT NULL)
);

CREATE TABLE http_response (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  status INT32 NOT NULL,
  body BLOB,
  time DATETIME NOT NULL,
  duration INT32 NOT NULL,
  size INT32 NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE http_response_header (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);

CREATE TABLE http_response_assert (
  id BLOB NOT NULL PRIMARY KEY,
  http_id BLOB NOT NULL,
  value TEXT NOT NULL,
  success BOOLEAN NOT NULL,
  created_at BIGINT NOT NULL DEFAULT (unixepoch()),
  FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE
);
`

	_, err = db.ExecContext(ctx, schema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Enable foreign key constraints
	_, err = db.ExecContext(ctx, "PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Create SQLC queries
	_ = gen.New(db)

	// Generate test IDs
	workspaceID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	headerID := idwrap.NewNow()
	searchParamID := idwrap.NewNow()
	bodyFormID := idwrap.NewNow()
	bodyUrlencodedID := idwrap.NewNow()
	assertID := idwrap.NewNow()
	responseID := idwrap.NewNow()
	responseHeaderID := idwrap.NewNow()
	responseAssertID := idwrap.NewNow()

	// Test 1: Create base HTTP record
	t.Run("CreateHTTP", func(t *testing.T) {
		// First create workspace
		_, err := db.ExecContext(ctx, "INSERT INTO workspaces (id, name) VALUES (?, ?)", workspaceID.Bytes(), "Test Workspace")
		if err != nil {
			t.Fatalf("Failed to create workspace: %v", err)
		}

		// Create HTTP record
		_, err = db.ExecContext(ctx, `
			INSERT INTO http (id, workspace_id, name, url, method) 
			VALUES (?, ?, ?, ?, ?)`,
			httpID.Bytes(), workspaceID.Bytes(), "Test API", "https://api.example.com/test", "GET")
		if err != nil {
			t.Fatalf("Failed to create HTTP record: %v", err)
		}

		// Verify HTTP record was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http WHERE id = ?", httpID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP record: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP record, got %d", count)
		}
	})

	// Test 2: Create HTTP header
	t.Run("CreateHTTPHeader", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_header (id, http_id, header_key, header_value) 
			VALUES (?, ?, ?, ?)`,
			headerID.Bytes(), httpID.Bytes(), "Content-Type", "application/json")
		if err != nil {
			t.Fatalf("Failed to create HTTP header: %v", err)
		}

		// Verify header was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_header WHERE id = ?", headerID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP header: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP header record, got %d", count)
		}
	})

	// Test 3: Create HTTP search param
	t.Run("CreateHTTPSearchParam", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_search_param (id, http_id, key, value) 
			VALUES (?, ?, ?, ?)`,
			searchParamID.Bytes(), httpID.Bytes(), "limit", "10")
		if err != nil {
			t.Fatalf("Failed to create HTTP search param: %v", err)
		}

		// Verify search param was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_search_param WHERE id = ?", searchParamID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP search param: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP search param record, got %d", count)
		}
	})

	// Test 4: Create HTTP body form
	t.Run("CreateHTTPBodyForm", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_body_form (id, http_id, key, value) 
			VALUES (?, ?, ?, ?)`,
			bodyFormID.Bytes(), httpID.Bytes(), "username", "testuser")
		if err != nil {
			t.Fatalf("Failed to create HTTP body form: %v", err)
		}

		// Verify body form was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_body_form WHERE id = ?", bodyFormID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP body form: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP body form record, got %d", count)
		}
	})

	// Test 5: Create HTTP body urlencoded
	t.Run("CreateHTTPBodyUrlencoded", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_body_urlencoded (id, http_id, key, value) 
			VALUES (?, ?, ?, ?)`,
			bodyUrlencodedID.Bytes(), httpID.Bytes(), "param1", "value1")
		if err != nil {
			t.Fatalf("Failed to create HTTP body urlencoded: %v", err)
		}

		// Verify body urlencoded was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_body_urlencoded WHERE id = ?", bodyUrlencodedID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP body urlencoded: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP body urlencoded record, got %d", count)
		}
	})

	// Test 6: Create HTTP assert
	t.Run("CreateHTTPAssert", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_assert (id, http_id, key, value) 
			VALUES (?, ?, ?, ?)`,
			assertID.Bytes(), httpID.Bytes(), "status_code", "200")
		if err != nil {
			t.Fatalf("Failed to create HTTP assert: %v", err)
		}

		// Verify assert was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_assert WHERE id = ?", assertID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP assert: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP assert record, got %d", count)
		}
	})

	// Test 7: Create HTTP response
	t.Run("CreateHTTPResponse", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_response (id, http_id, status, time, duration, size) 
			VALUES (?, ?, ?, ?, ?, ?)`,
			responseID.Bytes(), httpID.Bytes(), 200, time.Now(), 150, 1024)
		if err != nil {
			t.Fatalf("Failed to create HTTP response: %v", err)
		}

		// Verify response was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_response WHERE id = ?", responseID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP response: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP response record, got %d", count)
		}
	})

	// Test 8: Create HTTP response header
	t.Run("CreateHTTPResponseHeader", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_response_header (id, http_id, key, value) 
			VALUES (?, ?, ?, ?)`,
			responseHeaderID.Bytes(), httpID.Bytes(), "Server", "nginx/1.18.0")
		if err != nil {
			t.Fatalf("Failed to create HTTP response header: %v", err)
		}

		// Verify response header was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_response_header WHERE id = ?", responseHeaderID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP response header: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP response header record, got %d", count)
		}
	})

	// Test 9: Create HTTP response assert
	t.Run("CreateHTTPResponseAssert", func(t *testing.T) {
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_response_assert (id, http_id, value, success) 
			VALUES (?, ?, ?, ?)`,
			responseAssertID.Bytes(), httpID.Bytes(), "Response time < 500ms", true)
		if err != nil {
			t.Fatalf("Failed to create HTTP response assert: %v", err)
		}

		// Verify response assert was created
		var count int
		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_response_assert WHERE id = ?", responseAssertID.Bytes()).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to verify HTTP response assert: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 HTTP response assert record, got %d", count)
		}
	})

	// Test 10: Verify foreign key constraints
	t.Run("VerifyForeignKeyConstraints", func(t *testing.T) {
		// Try to insert a header with invalid http_id (should fail)
		invalidHeaderID := idwrap.NewNow()
		_, err := db.ExecContext(ctx, `
			INSERT INTO http_header (id, http_id, header_key, header_value) 
			VALUES (?, ?, ?, ?)`,
			invalidHeaderID.Bytes(), []byte("invalid-http-id"), "Invalid", "Header")

		if err == nil {
			t.Errorf("Expected foreign key constraint error, but got none")
		}
	})

	// Test 11: Verify data integrity
	t.Run("VerifyDataIntegrity", func(t *testing.T) {
		// Count all related records
		var headerCount, searchParamCount, bodyFormCount, bodyUrlencodedCount, assertCount int
		var responseCount, responseHeaderCount, responseAssertCount int

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_header WHERE http_id = ?", httpID.Bytes()).Scan(&headerCount)
		if err != nil {
			t.Fatalf("Failed to count headers: %v", err)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_search_param WHERE http_id = ?", httpID.Bytes()).Scan(&searchParamCount)
		if err != nil {
			t.Fatalf("Failed to count search params: %v", err)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_body_form WHERE http_id = ?", httpID.Bytes()).Scan(&bodyFormCount)
		if err != nil {
			t.Fatalf("Failed to count body forms: %v", err)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_body_urlencoded WHERE http_id = ?", httpID.Bytes()).Scan(&bodyUrlencodedCount)
		if err != nil {
			t.Fatalf("Failed to count body urlencoded: %v", err)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_assert WHERE http_id = ?", httpID.Bytes()).Scan(&assertCount)
		if err != nil {
			t.Fatalf("Failed to count asserts: %v", err)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_response WHERE http_id = ?", httpID.Bytes()).Scan(&responseCount)
		if err != nil {
			t.Fatalf("Failed to count responses: %v", err)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_response_header WHERE http_id = ?", httpID.Bytes()).Scan(&responseHeaderCount)
		if err != nil {
			t.Fatalf("Failed to count response headers: %v", err)
		}

		err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM http_response_assert WHERE http_id = ?", httpID.Bytes()).Scan(&responseAssertCount)
		if err != nil {
			t.Fatalf("Failed to count response asserts: %v", err)
		}

		// Verify all counts are 1 (we created one of each)
		if headerCount != 1 || searchParamCount != 1 || bodyFormCount != 1 ||
			bodyUrlencodedCount != 1 || assertCount != 1 || responseCount != 1 ||
			responseHeaderCount != 1 || responseAssertCount != 1 {
			t.Errorf("Data integrity check failed. Expected all counts to be 1, got: "+
				"headers=%d, searchParams=%d, bodyForms=%d, bodyUrlencoded=%d, asserts=%d, "+
				"responses=%d, responseHeaders=%d, responseAsserts=%d",
				headerCount, searchParamCount, bodyFormCount, bodyUrlencodedCount, assertCount,
				responseCount, responseHeaderCount, responseAssertCount)
		}
	})
}

func main() {
	// Run the verification test
	t := &testing.T{}
	TestHTTPChildEntityVerification(t)

	if t.Failed() {
		log.Println("❌ HTTP Child Entity Database Verification FAILED")
	} else {
		log.Println("✅ HTTP Child Entity Database Verification PASSED")
	}
}
