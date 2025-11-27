package devtoolsdb

import (
	"context"
	"database/sql"
	"log"
	"testing"

	_ "modernc.org/sqlite"
	gen "the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
)

func TestUpdateFlowNodeHTTPUpsert(t *testing.T) {
	ctx := context.Background()

	// Create in-memory SQLite database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Minimal schema to support flow_node_http
	schema := `
	CREATE TABLE workspaces (
		id BLOB NOT NULL PRIMARY KEY,
		name TEXT NOT NULL
	);

	CREATE TABLE flow (
		id BLOB NOT NULL PRIMARY KEY,
		workspace_id BLOB NOT NULL,
		version_parent_id BLOB DEFAULT NULL,
		name TEXT NOT NULL,
		duration INT NOT NULL DEFAULT 0,
		running BOOLEAN NOT NULL DEFAULT FALSE,
		FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
	);

	CREATE TABLE http (
		id BLOB NOT NULL PRIMARY KEY,
		workspace_id BLOB NOT NULL,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		method TEXT NOT NULL
	);

	CREATE TABLE flow_node (
		id BLOB NOT NULL PRIMARY KEY,
		flow_id BLOB NOT NULL,
		name TEXT NOT NULL,
		node_kind INT NOT NULL,
		position_x REAL NOT NULL,
		position_y REAL NOT NULL,
		FOREIGN KEY (flow_id) REFERENCES flow (id) ON DELETE CASCADE
	);

	CREATE TABLE flow_node_http (
		flow_node_id BLOB NOT NULL PRIMARY KEY,
		http_id BLOB NOT NULL,
		delta_http_id BLOB,
		FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE,
		FOREIGN KEY (http_id) REFERENCES http (id) ON DELETE CASCADE,
		FOREIGN KEY (delta_http_id) REFERENCES http (id) ON DELETE SET NULL
	);
	`

	_, err = db.ExecContext(ctx, schema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	queries := gen.New(db)

	// 1. Setup data
	workspaceID := idwrap.NewNow()
	flowID := idwrap.NewNow()
	flowNodeID := idwrap.NewNow()
	httpID1 := idwrap.NewNow()
	httpID2 := idwrap.NewNow()

	// Insert Workspace
	if _, err := db.ExecContext(ctx, "INSERT INTO workspaces (id, name) VALUES (?, ?)", workspaceID.Bytes(), "WS"); err != nil {
		t.Fatalf("Failed to insert workspace: %v", err)
	}

	// Insert Flow
	if _, err := db.ExecContext(ctx, "INSERT INTO flow (id, workspace_id, name) VALUES (?, ?, ?)", flowID.Bytes(), workspaceID.Bytes(), "Flow"); err != nil {
		t.Fatalf("Failed to insert flow: %v", err)
	}

	// Insert Flow Node
	if _, err := db.ExecContext(ctx, "INSERT INTO flow_node (id, flow_id, name, node_kind, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?)", flowNodeID.Bytes(), flowID.Bytes(), "Node", 1, 0, 0); err != nil {
		t.Fatalf("Failed to insert flow node: %v", err)
	}

	// Insert HTTPs
	if _, err := db.ExecContext(ctx, "INSERT INTO http (id, workspace_id, name, url, method) VALUES (?, ?, ?, ?, ?)", httpID1.Bytes(), workspaceID.Bytes(), "HTTP1", "url1", "GET"); err != nil {
		t.Fatalf("Failed to insert http1: %v", err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO http (id, workspace_id, name, url, method) VALUES (?, ?, ?, ?, ?)", httpID2.Bytes(), workspaceID.Bytes(), "HTTP2", "url2", "POST"); err != nil {
		t.Fatalf("Failed to insert http2: %v", err)
	}

	// 2. Verify Upsert Behavior (Insert case)
	// UpdateFlowNodeHTTP should insert if record doesn't exist
	err = queries.UpdateFlowNodeHTTP(ctx, gen.UpdateFlowNodeHTTPParams{
		FlowNodeID:  flowNodeID,
		HttpID:      httpID1,
		DeltaHttpID: nil,
	})
	if err != nil {
		t.Fatalf("UpdateFlowNodeHTTP (Insert case) failed: %v", err)
	}

	// Verify insertion
	nodeHTTP, err := queries.GetFlowNodeHTTP(ctx, flowNodeID)
	if err != nil {
		t.Fatalf("GetFlowNodeHTTP failed: %v", err)
	}
	if nodeHTTP.HttpID != httpID1 {
		t.Errorf("Expected HttpID %v, got %v", httpID1, nodeHTTP.HttpID)
	}

	// 3. Verify Upsert Behavior (Update case)
	// UpdateFlowNodeHTTP should update if record exists
	err = queries.UpdateFlowNodeHTTP(ctx, gen.UpdateFlowNodeHTTPParams{
		FlowNodeID:  flowNodeID,
		HttpID:      httpID2,
		DeltaHttpID: nil,
	})
	if err != nil {
		t.Fatalf("UpdateFlowNodeHTTP (Update case) failed: %v", err)
	}

	// Verify update
	nodeHTTP, err = queries.GetFlowNodeHTTP(ctx, flowNodeID)
	if err != nil {
		t.Fatalf("GetFlowNodeHTTP failed: %v", err)
	}
	if nodeHTTP.HttpID != httpID2 {
		t.Errorf("Expected HttpID %v, got %v", httpID2, nodeHTTP.HttpID)
	}

	log.Println("✅ UpdateFlowNodeHTTP Upsert Test PASSED")
}
