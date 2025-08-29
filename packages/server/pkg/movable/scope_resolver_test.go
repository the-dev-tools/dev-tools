package movable

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"

	_ "github.com/mattn/go-sqlite3"
)

// Test database schema setup
const testSchema = `
CREATE TABLE workspaces (
	id BLOB NOT NULL PRIMARY KEY,
	name TEXT NOT NULL
);

CREATE TABLE collections (
	id BLOB NOT NULL PRIMARY KEY,
	workspace_id BLOB NOT NULL,
	name TEXT NOT NULL,
	FOREIGN KEY (workspace_id) REFERENCES workspaces (id)
);

CREATE TABLE item_folder (
	id BLOB NOT NULL PRIMARY KEY,
	collection_id BLOB NOT NULL,
	parent_id BLOB,
	name TEXT NOT NULL,
	FOREIGN KEY (collection_id) REFERENCES collections (id),
	FOREIGN KEY (parent_id) REFERENCES item_folder (id)
);

CREATE TABLE collection_items (
	id BLOB NOT NULL PRIMARY KEY,
	collection_id BLOB NOT NULL,
	parent_folder_id BLOB,
	item_type INT8 NOT NULL,
	folder_id BLOB,
	endpoint_id BLOB,
	name TEXT NOT NULL,
	FOREIGN KEY (collection_id) REFERENCES collections (id),
	FOREIGN KEY (folder_id) REFERENCES item_folder (id)
);

CREATE TABLE item_api (
	id BLOB NOT NULL PRIMARY KEY,
	collection_id BLOB NOT NULL,
	folder_id BLOB,
	name TEXT NOT NULL,
	url TEXT NOT NULL,
	method TEXT NOT NULL,
	delta_parent_id BLOB DEFAULT NULL,
	hidden BOOLEAN NOT NULL DEFAULT FALSE,
	FOREIGN KEY (collection_id) REFERENCES collections (id),
	FOREIGN KEY (folder_id) REFERENCES item_folder (id),
	FOREIGN KEY (delta_parent_id) REFERENCES item_api (id)
);

CREATE TABLE item_api_example (
	id BLOB NOT NULL PRIMARY KEY,
	item_api_id BLOB NOT NULL,
	collection_id BLOB NOT NULL,
	is_default BOOLEAN NOT NULL DEFAULT FALSE,
	name TEXT NOT NULL,
	version_parent_id BLOB DEFAULT NULL,
	FOREIGN KEY (item_api_id) REFERENCES item_api (id),
	FOREIGN KEY (collection_id) REFERENCES collections (id),
	FOREIGN KEY (version_parent_id) REFERENCES item_api_example (id)
);

CREATE TABLE flow (
	id BLOB NOT NULL PRIMARY KEY,
	workspace_id BLOB NOT NULL,
	name TEXT NOT NULL,
	FOREIGN KEY (workspace_id) REFERENCES workspaces (id)
);

CREATE TABLE flow_node (
	id BLOB NOT NULL PRIMARY KEY,
	flow_id BLOB NOT NULL,
	name TEXT NOT NULL,
	node_kind INT NOT NULL,
	position_x REAL NOT NULL DEFAULT 0,
	position_y REAL NOT NULL DEFAULT 0,
	FOREIGN KEY (flow_id) REFERENCES flow (id)
);
`

func setupScopeTestDB(t *testing.T) (*sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	_, err = db.Exec(testSchema)
	if err != nil {
		t.Fatalf("Failed to create test schema: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func insertTestData(t *testing.T, db *sql.DB) (workspaceID, collectionID, folderID, endpointID, exampleID, flowID, nodeID idwrap.IDWrap) {
	workspaceID = idwrap.NewNow()
	collectionID = idwrap.NewNow()
	folderID = idwrap.NewNow()
	endpointID = idwrap.NewNow()
	exampleID = idwrap.NewNow()
	flowID = idwrap.NewNow()
	nodeID = idwrap.NewNow()

	// Insert workspace
	_, err := db.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`,
		workspaceID.Bytes(), "Test Workspace")
	if err != nil {
		t.Fatalf("Failed to insert workspace: %v", err)
	}

	// Insert collection
	_, err = db.Exec(`INSERT INTO collections (id, workspace_id, name) VALUES (?, ?, ?)`,
		collectionID.Bytes(), workspaceID.Bytes(), "Test Collection")
	if err != nil {
		t.Fatalf("Failed to insert collection: %v", err)
	}

	// Insert folder
	_, err = db.Exec(`INSERT INTO item_folder (id, collection_id, name) VALUES (?, ?, ?)`,
		folderID.Bytes(), collectionID.Bytes(), "Test Folder")
	if err != nil {
		t.Fatalf("Failed to insert folder: %v", err)
	}

	// Insert collection item
	collectionItemID := idwrap.NewNow()
	_, err = db.Exec(`INSERT INTO collection_items (id, collection_id, item_type, folder_id, name) VALUES (?, ?, ?, ?, ?)`,
		collectionItemID.Bytes(), collectionID.Bytes(), 0, folderID.Bytes(), "Test Item")
	if err != nil {
		t.Fatalf("Failed to insert collection item: %v", err)
	}

	// Insert endpoint
	_, err = db.Exec(`INSERT INTO item_api (id, collection_id, folder_id, name, url, method) VALUES (?, ?, ?, ?, ?, ?)`,
		endpointID.Bytes(), collectionID.Bytes(), folderID.Bytes(), "Test Endpoint", "/test", "GET")
	if err != nil {
		t.Fatalf("Failed to insert endpoint: %v", err)
	}

	// Insert example
	_, err = db.Exec(`INSERT INTO item_api_example (id, item_api_id, collection_id, name) VALUES (?, ?, ?, ?)`,
		exampleID.Bytes(), endpointID.Bytes(), collectionID.Bytes(), "Test Example")
	if err != nil {
		t.Fatalf("Failed to insert example: %v", err)
	}

	// Insert flow
	_, err = db.Exec(`INSERT INTO flow (id, workspace_id, name) VALUES (?, ?, ?)`,
		flowID.Bytes(), workspaceID.Bytes(), "Test Flow")
	if err != nil {
		t.Fatalf("Failed to insert flow: %v", err)
	}

	// Insert flow node
	_, err = db.Exec(`INSERT INTO flow_node (id, flow_id, name, node_kind) VALUES (?, ?, ?, ?)`,
		nodeID.Bytes(), flowID.Bytes(), "Test Node", 1)
	if err != nil {
		t.Fatalf("Failed to insert flow node: %v", err)
	}

	return workspaceID, collectionID, folderID, endpointID, exampleID, flowID, nodeID
}

func TestNewDefaultScopeResolver(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	if resolver == nil {
		t.Fatal("Expected non-nil resolver")
	}
}

func TestResolveContext(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	workspaceID, _, _, endpointID, exampleID, _, nodeID := insertTestData(t, db)

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	tests := []struct {
		name        string
		itemID      idwrap.IDWrap
		expected    MovableContext
		shouldError bool
	}{
		{
			name:     "resolve workspace context",
			itemID:   workspaceID,
			expected: ContextWorkspace,
		},
		{
			name:     "resolve endpoint context",
			itemID:   endpointID,
			expected: ContextEndpoint,
		},
		{
			name:     "resolve example context",
			itemID:   exampleID,
			expected: ContextRequest,
		},
		{
			name:     "resolve flow node context",
			itemID:   nodeID,
			expected: ContextFlow,
		},
		{
			name:        "resolve non-existent item",
			itemID:      idwrap.NewNow(),
			shouldError: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, err := resolver.ResolveContext(ctx, tt.itemID)
			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if context != tt.expected {
				t.Errorf("Expected context %v, got %v", tt.expected, context)
			}
		})
	}
}

func TestResolveContextCaching(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	_, _, _, endpointID, _, _, _ := insertTestData(t, db)

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	ctx := context.Background()

	// First call should query database
	context1, err := resolver.ResolveContext(ctx, endpointID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Second call should use cache
	context2, err := resolver.ResolveContext(ctx, endpointID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if context1 != context2 {
		t.Errorf("Cached context should match original: %v != %v", context1, context2)
	}

	// Verify cache contains the item
	if metadata, found := cache.GetContext(endpointID); !found {
		t.Error("Expected item to be cached")
	} else if metadata.Type != ContextEndpoint {
		t.Errorf("Expected cached context %v, got %v", ContextEndpoint, metadata.Type)
	}
}

func TestResolveScopeID(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	workspaceID, collectionID, _, endpointID, exampleID, flowID, nodeID := insertTestData(t, db)

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	tests := []struct {
		name        string
		itemID      idwrap.IDWrap
		contextType MovableContext
		expected    idwrap.IDWrap
		shouldError bool
	}{
		{
			name:        "resolve endpoint scope",
			itemID:      endpointID,
			contextType: ContextEndpoint,
			expected:    collectionID,
		},
		{
			name:        "resolve example scope",
			itemID:      exampleID,
			contextType: ContextRequest,
			expected:    endpointID,
		},
		{
			name:        "resolve flow node scope",
			itemID:      nodeID,
			contextType: ContextFlow,
			expected:    flowID,
		},
		{
			name:        "resolve workspace scope",
			itemID:      workspaceID,
			contextType: ContextWorkspace,
			expected:    workspaceID,
		},
		{
			name:        "resolve with wrong context type",
			itemID:      endpointID,
			contextType: ContextFlow,
			shouldError: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopeID, err := resolver.ResolveScopeID(ctx, tt.itemID, tt.contextType)
			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if scopeID.Compare(tt.expected) != 0 {
				t.Errorf("Expected scope ID %v, got %v", tt.expected.String(), scopeID.String())
			}
		})
	}
}

func TestValidateScope(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	_, collectionID, _, endpointID, _, _, _ := insertTestData(t, db)

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	ctx := context.Background()

	// Valid scope validation
	err = resolver.ValidateScope(ctx, endpointID, collectionID)
	if err != nil {
		t.Errorf("Unexpected error for valid scope: %v", err)
	}

	// Invalid scope validation
	wrongScope := idwrap.NewNow()
	err = resolver.ValidateScope(ctx, endpointID, wrongScope)
	if err == nil {
		t.Error("Expected error for invalid scope")
	}
}

func TestGetScopeHierarchy(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	workspaceID, _, _, endpointID, exampleID, _, nodeID := insertTestData(t, db)

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	tests := []struct {
		name           string
		itemID         idwrap.IDWrap
		expectedLevels int
		expectedTop    MovableContext
	}{
		{
			name:           "endpoint hierarchy",
			itemID:         endpointID,
			expectedLevels: 2, // workspace + collection
			expectedTop:    ContextWorkspace,
		},
		{
			name:           "example hierarchy",
			itemID:         exampleID,
			expectedLevels: 3, // workspace + collection + endpoint
			expectedTop:    ContextWorkspace,
		},
		{
			name:           "flow node hierarchy",
			itemID:         nodeID,
			expectedLevels: 2, // workspace + flow
			expectedTop:    ContextWorkspace,
		},
		{
			name:           "workspace hierarchy",
			itemID:         workspaceID,
			expectedLevels: 1, // workspace only
			expectedTop:    ContextWorkspace,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hierarchy, err := resolver.GetScopeHierarchy(ctx, tt.itemID)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(hierarchy) != tt.expectedLevels {
				t.Errorf("Expected %d levels, got %d", tt.expectedLevels, len(hierarchy))
			}

			if len(hierarchy) > 0 {
				if hierarchy[0].Context != tt.expectedTop {
					t.Errorf("Expected top context %v, got %v", tt.expectedTop, hierarchy[0].Context)
				}
				
				// Verify levels are in ascending order
				for i := 1; i < len(hierarchy); i++ {
					if hierarchy[i].Level != hierarchy[i-1].Level+1 {
						t.Errorf("Hierarchy levels not in order: level %d followed by level %d",
							hierarchy[i-1].Level, hierarchy[i].Level)
					}
				}
			}
		})
	}
}

func TestGetContextBoundaries(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	ctx := context.Background()

	tests := []struct {
		name        string
		contextType MovableContext
		shouldError bool
	}{
		{name: "collection boundaries", contextType: ContextCollection},
		{name: "flow boundaries", contextType: ContextFlow},
		{name: "endpoint boundaries", contextType: ContextEndpoint},
		{name: "request boundaries", contextType: ContextRequest},
		{name: "workspace boundaries", contextType: ContextWorkspace},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boundaries, err := resolver.GetContextBoundaries(ctx, tt.contextType)
			if tt.shouldError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if boundaries == nil {
				t.Error("Expected non-nil boundaries")
			}
			if len(boundaries) == 0 {
				t.Error("Expected non-empty boundaries")
			}
		})
	}
}

func TestBatchResolveContexts(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	workspaceID, _, _, endpointID, exampleID, _, nodeID := insertTestData(t, db)

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	itemIDs := []idwrap.IDWrap{workspaceID, endpointID, exampleID, nodeID}
	expected := map[idwrap.IDWrap]MovableContext{
		workspaceID: ContextWorkspace,
		endpointID:  ContextEndpoint,
		exampleID:   ContextRequest,
		nodeID:      ContextFlow,
	}

	ctx := context.Background()
	results, err := resolver.BatchResolveContexts(ctx, itemIDs)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(results) != len(expected) {
		t.Errorf("Expected %d results, got %d", len(expected), len(results))
	}

	for itemID, expectedContext := range expected {
		if actualContext, found := results[itemID]; !found {
			t.Errorf("Missing result for item %v", itemID.String())
		} else if actualContext != expectedContext {
			t.Errorf("Expected context %v for item %v, got %v",
				expectedContext, itemID.String(), actualContext)
		}
	}
}

func TestScopeResolverPerformance(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	// Insert many test items for performance testing
	_, collectionID, folderID, _, _, _, _ := insertTestData(t, db)

	// Create many endpoints for testing
	endpointIDs := make([]idwrap.IDWrap, 100)
	for i := 0; i < 100; i++ {
		endpointID := idwrap.NewNow()
		endpointIDs[i] = endpointID
		_, err := db.Exec(`INSERT INTO item_api (id, collection_id, folder_id, name, url, method) VALUES (?, ?, ?, ?, ?, ?)`,
			endpointID.Bytes(), collectionID.Bytes(), folderID.Bytes(), "Test Endpoint", "/test", "GET")
		if err != nil {
			t.Fatalf("Failed to insert endpoint: %v", err)
		}
	}

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	ctx := context.Background()
	
	// Test single resolution performance
	start := time.Now()
	_, err = resolver.ResolveContext(ctx, endpointIDs[0])
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	singleDuration := time.Since(start)

	// Test batch resolution performance
	start = time.Now()
	results, err := resolver.BatchResolveContexts(ctx, endpointIDs)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	batchDuration := time.Since(start)

	if len(results) != len(endpointIDs) {
		t.Errorf("Expected %d results, got %d", len(endpointIDs), len(results))
	}

	// Test cached resolution performance (should be much faster)
	start = time.Now()
	_, err = resolver.ResolveContext(ctx, endpointIDs[0])
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	cachedDuration := time.Since(start)

	t.Logf("Single resolution: %v", singleDuration)
	t.Logf("Batch resolution (100 items): %v", batchDuration)
	t.Logf("Cached resolution: %v", cachedDuration)

	// Cached resolution should be much faster than database query
	if cachedDuration > singleDuration/2 {
		t.Logf("Warning: Cached resolution (%v) not significantly faster than uncached (%v)", 
			cachedDuration, singleDuration)
	}

	// Verify performance requirements
	if cachedDuration > time.Microsecond {
		t.Errorf("Cached resolution took %v, expected < 1µs", cachedDuration)
	}
}

func TestScopeResolverThreadSafety(t *testing.T) {
	db, cleanup := setupScopeTestDB(t)
	defer cleanup()

	workspaceID, _, _, endpointID, exampleID, _, nodeID := insertTestData(t, db)

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		t.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	itemIDs := []idwrap.IDWrap{workspaceID, endpointID, exampleID, nodeID}

	// Run concurrent operations
	ctx := context.Background()
	done := make(chan bool)
	errors := make(chan error, 20)

	// Start 10 goroutines doing concurrent resolutions
	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()
			
			for _, itemID := range itemIDs {
				_, err := resolver.ResolveContext(ctx, itemID)
				if err != nil {
					errors <- err
					return
				}
				
				contextType, err := resolver.ResolveContext(ctx, itemID)
				if err != nil {
					errors <- err
					return
				}
				
				_, err = resolver.ResolveScopeID(ctx, itemID, contextType)
				if err != nil {
					errors <- err
					return
				}
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Goroutine completed successfully
		case err := <-errors:
			t.Fatalf("Concurrent operation failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Test timed out - possible deadlock")
		}
	}

	// Check if there were any errors
	select {
	case err := <-errors:
		t.Fatalf("Concurrent operation failed: %v", err)
	default:
		// No errors
	}
}

func TestInMemoryContextCache(t *testing.T) {
	cache := NewInMemoryContextCache(100 * time.Millisecond)
	itemID := idwrap.NewNow()
	
	metadata := &ContextMetadata{
		Type:      ContextCollection,
		ScopeID:   idwrap.NewNow().Bytes(),
		IsHidden:  false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test set and get
	cache.SetContext(itemID, metadata)
	retrieved, found := cache.GetContext(itemID)
	if !found {
		t.Error("Expected to find cached item")
	}
	if retrieved.Type != metadata.Type {
		t.Errorf("Expected type %v, got %v", metadata.Type, retrieved.Type)
	}

	// Test TTL expiration
	time.Sleep(150 * time.Millisecond)
	_, found = cache.GetContext(itemID)
	if found {
		t.Error("Expected cache item to be expired")
	}

	// Test invalidation
	cache.SetContext(itemID, metadata)
	cache.InvalidateContext(itemID)
	_, found = cache.GetContext(itemID)
	if found {
		t.Error("Expected cache item to be invalidated")
	}

	// Test clear
	cache.SetContext(itemID, metadata)
	cache.Clear()
	_, found = cache.GetContext(itemID)
	if found {
		t.Error("Expected cache to be cleared")
	}
}

func BenchmarkResolveContext(b *testing.B) {
	db, cleanup := setupScopeTestDB(b)
	defer cleanup()

	_, _, _, endpointID, _, _, _ := insertTestData(b, db)

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		b.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.ResolveContext(ctx, endpointID)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}

func BenchmarkBatchResolveContexts(b *testing.B) {
	db, cleanup := setupScopeTestDB(b)
	defer cleanup()

	_, collectionID, folderID, _, _, _, _ := insertTestData(b, db)

	// Create 100 endpoints for batch testing
	endpointIDs := make([]idwrap.IDWrap, 100)
	for i := 0; i < 100; i++ {
		endpointID := idwrap.NewNow()
		endpointIDs[i] = endpointID
		_, err := db.Exec(`INSERT INTO item_api (id, collection_id, folder_id, name, url, method) VALUES (?, ?, ?, ?, ?, ?)`,
			endpointID.Bytes(), collectionID.Bytes(), folderID.Bytes(), "Test Endpoint", "/test", "GET")
		if err != nil {
			b.Fatalf("Failed to insert endpoint: %v", err)
		}
	}

	cache := NewInMemoryContextCache(5 * time.Minute)
	resolver, err := NewDefaultScopeResolver(db, cache)
	if err != nil {
		b.Fatalf("Failed to create scope resolver: %v", err)
	}
	defer resolver.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.BatchResolveContexts(ctx, endpointIDs)
		if err != nil {
			b.Fatalf("Unexpected error: %v", err)
		}
	}
}