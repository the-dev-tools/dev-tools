package scollectionitem_test

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/service/scollectionitem"

	_ "github.com/mattn/go-sqlite3"
)

// BenchmarkCrossCollectionMoveOperations benchmarks the performance of cross-collection move operations
func BenchmarkCrossCollectionMoveOperations(b *testing.B) {
	ctx := context.Background()
	
	// Create in-memory database for benchmarking
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize schema (this would normally be done via migrations)
	if err := initializeSchema(db); err != nil {
		b.Fatalf("Failed to initialize schema: %v", err)
	}

	// Create test data
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		b.Fatalf("Failed to prepare queries: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn}))
	service := scollectionitem.New(queries, logger)

	// Setup test data
	workspaceID := idwrap.NewNow()
	sourceCollectionID, targetCollectionID, itemIDs := setupBenchmarkData(ctx, b, queries, workspaceID)

	b.ResetTimer()

	// Benchmark cross-collection moves
	b.Run("CrossCollectionMove", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Use modulo to cycle through available items
			itemIdx := i % len(itemIDs)
			targetCollectionIdx := i % 2
			
			var targetCollection idwrap.IDWrap
			if targetCollectionIdx == 0 {
				targetCollection = targetCollectionID
			} else {
				targetCollection = sourceCollectionID
			}

			err := service.MoveCollectionItemCrossCollection(
				ctx,
				itemIDs[itemIdx],
				targetCollection,
				nil, // no target parent folder
				nil, // no target item
				movable.MovePositionUnspecified,
			)
			if err != nil {
				b.Errorf("Failed to move item %d: %v", i, err)
			}
		}
	})

	// Benchmark validation queries specifically
	b.Run("ValidationQueries", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Test ValidateCollectionsInSameWorkspace
			_, err := queries.ValidateCollectionsInSameWorkspace(ctx, gen.ValidateCollectionsInSameWorkspaceParams{
				ID:   sourceCollectionID,
				ID_2: targetCollectionID,
			})
			if err != nil {
				b.Errorf("Validation failed: %v", err)
			}
		}
	})

	// Benchmark item workspace resolution  
	b.Run("ItemWorkspaceResolution", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			itemIdx := i % len(itemIDs)
			_, err := queries.GetCollectionWorkspaceByItemId(ctx, itemIDs[itemIdx])
			if err != nil {
				b.Errorf("Workspace resolution failed: %v", err)
			}
		}
	})
}

// BenchmarkLargeDatasetOperations tests performance with larger datasets
func BenchmarkLargeDatasetOperations(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping large dataset benchmark in short mode")
	}

	ctx := context.Background()
	
	// Create in-memory database
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := initializeSchema(db); err != nil {
		b.Fatalf("Failed to initialize schema: %v", err)
	}

	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		b.Fatalf("Failed to prepare queries: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	service := scollectionitem.New(queries, logger)

	// Create larger dataset
	workspaceID := idwrap.NewNow()
	_, targetCollectionID, itemIDs := setupLargeBenchmarkData(ctx, b, queries, workspaceID, 10000)

	b.ResetTimer()

	b.Run("LargeDatasetCrossCollectionMove", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			itemIdx := i % len(itemIDs)
			targetCollection := targetCollectionID
			
			start := time.Now()
			err := service.MoveCollectionItemCrossCollection(
				ctx,
				itemIDs[itemIdx],
				targetCollection,
				nil, // no target parent folder
				nil, // no target item
				movable.MovePositionUnspecified,
			)
			duration := time.Since(start)
			
			if err != nil {
				b.Errorf("Failed to move item %d: %v", i, err)
			}
			
			// Log slow operations for analysis
			if duration > 50*time.Millisecond {
				b.Logf("Slow operation detected: %v for item %d", duration, itemIdx)
			}
		}
	})
}

// setupBenchmarkData creates test data for benchmarking
func setupBenchmarkData(ctx context.Context, b *testing.B, queries *gen.Queries, workspaceID idwrap.IDWrap) (sourceCollectionID, targetCollectionID idwrap.IDWrap, itemIDs []idwrap.IDWrap) {
	// Create workspace
	userID := idwrap.NewNow()
	
	err := queries.CreateUser(ctx, gen.CreateUserParams{
		ID:           userID,
		Email:        "benchmark@test.com",
		ProviderType: 0,
	})
	if err != nil {
		b.Fatalf("Failed to create user: %v", err)
	}

	err = queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "Benchmark Workspace",
	})
	if err != nil {
		b.Fatalf("Failed to create workspace: %v", err)
	}

	err = queries.CreateWorkspaceUser(ctx, gen.CreateWorkspaceUserParams{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        1,
	})
	if err != nil {
		b.Fatalf("Failed to create workspace user: %v", err)
	}

	// Create collections
	sourceCollectionID = idwrap.NewNow()
	targetCollectionID = idwrap.NewNow()

	err = queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          sourceCollectionID,
		WorkspaceID: workspaceID,
		Name:        "Source Collection",
	})
	if err != nil {
		b.Fatalf("Failed to create source collection: %v", err)
	}

	err = queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          targetCollectionID,
		WorkspaceID: workspaceID,
		Name:        "Target Collection",
	})
	if err != nil {
		b.Fatalf("Failed to create target collection: %v", err)
	}

	// Create test items (mix of folders and endpoints)
	itemCount := 100
	itemIDs = make([]idwrap.IDWrap, 0, itemCount)

	for i := 0; i < itemCount; i++ {
		itemID := idwrap.NewNow()
		itemIDs = append(itemIDs, itemID)

		if i%2 == 0 {
			// Create folder item
			folderID := idwrap.NewNow()
			err = queries.CreateItemFolder(ctx, gen.CreateItemFolderParams{
				ID:           folderID,
				Name:         fmt.Sprintf("Folder %d", i),
				CollectionID: sourceCollectionID,
			})
			if err != nil {
				b.Fatalf("Failed to create folder: %v", err)
			}

			err = queries.InsertCollectionItem(ctx, gen.InsertCollectionItemParams{
				ID:           itemID,
				CollectionID: sourceCollectionID,
				ItemType:     0, // folder
				FolderID:     &folderID,
				Name:         fmt.Sprintf("Folder %d", i),
			})
			if err != nil {
				b.Fatalf("Failed to create collection item: %v", err)
			}
		} else {
			// Create endpoint item  
			endpointID := idwrap.NewNow()
			err = queries.CreateItemApi(ctx, gen.CreateItemApiParams{
				ID:           endpointID,
				CollectionID: sourceCollectionID,
				Name:         fmt.Sprintf("Endpoint %d", i),
				Url:          fmt.Sprintf("https://api.example.com/endpoint%d", i),
				Method:       "GET",
			})
			if err != nil {
				b.Fatalf("Failed to create endpoint: %v", err)
			}

			err = queries.InsertCollectionItem(ctx, gen.InsertCollectionItemParams{
				ID:           itemID,
				CollectionID: sourceCollectionID,
				ItemType:     1, // endpoint
				EndpointID:   &endpointID,
				Name:         fmt.Sprintf("Endpoint %d", i),
			})
			if err != nil {
				b.Fatalf("Failed to create collection item: %v", err)
			}
		}
	}

	return sourceCollectionID, targetCollectionID, itemIDs
}

// setupLargeBenchmarkData creates a larger dataset for stress testing
func setupLargeBenchmarkData(ctx context.Context, b *testing.B, queries *gen.Queries, workspaceID idwrap.IDWrap, itemCount int) (sourceCollectionID, targetCollectionID idwrap.IDWrap, itemIDs []idwrap.IDWrap) {
	// Similar to setupBenchmarkData but with more items
	sourceCollectionID, targetCollectionID, _ = setupBenchmarkData(ctx, b, queries, workspaceID)
	
	itemIDs = make([]idwrap.IDWrap, 0, itemCount)
	
	// Create additional items for large dataset testing
	for i := 100; i < itemCount; i++ {
		itemID := idwrap.NewNow()
		itemIDs = append(itemIDs, itemID)

		// Create endpoint items (simpler than folders for large datasets)
		endpointID := idwrap.NewNow()
		err := queries.CreateItemApi(ctx, gen.CreateItemApiParams{
			ID:           endpointID,
			CollectionID: sourceCollectionID,
			Name:         fmt.Sprintf("Large Endpoint %d", i),
			Url:          fmt.Sprintf("https://api.example.com/large%d", i),
			Method:       "POST",
		})
		if err != nil {
			b.Fatalf("Failed to create large endpoint: %v", err)
		}

		err = queries.InsertCollectionItem(ctx, gen.InsertCollectionItemParams{
			ID:           itemID,
			CollectionID: sourceCollectionID,
			ItemType:     1, // endpoint
			EndpointID:   &endpointID,
			Name:         fmt.Sprintf("Large Endpoint %d", i),
		})
		if err != nil {
			b.Fatalf("Failed to create large collection item: %v", err)
		}

		// Add some progress logging for large datasets
		if i%1000 == 0 {
			b.Logf("Created %d items...", i)
		}
	}

	return sourceCollectionID, targetCollectionID, itemIDs
}

// initializeSchema creates the database schema for testing
func initializeSchema(db *sql.DB) error {
	// This would normally read from schema.sql file
	// For this benchmark, we'll include the essential tables and indexes
	
	schema := `
	-- Essential tables for cross-collection move testing
	CREATE TABLE users (id BLOB NOT NULL PRIMARY KEY, email TEXT NOT NULL UNIQUE, provider_type INT8 NOT NULL DEFAULT 0, provider_id TEXT);
	CREATE TABLE workspaces (id BLOB NOT NULL PRIMARY KEY, name TEXT NOT NULL);
	CREATE TABLE workspaces_users (id BLOB NOT NULL PRIMARY KEY, workspace_id BLOB NOT NULL, user_id BLOB NOT NULL, role INT8 NOT NULL DEFAULT 1, FOREIGN KEY (workspace_id) REFERENCES workspaces (id), FOREIGN KEY (user_id) REFERENCES users (id));
	CREATE TABLE collections (id BLOB NOT NULL PRIMARY KEY, workspace_id BLOB NOT NULL, name TEXT NOT NULL, FOREIGN KEY (workspace_id) REFERENCES workspaces (id));
	CREATE TABLE item_folder (id BLOB NOT NULL PRIMARY KEY, collection_id BLOB NOT NULL, name TEXT NOT NULL, FOREIGN KEY (collection_id) REFERENCES collections (id));
	CREATE TABLE item_api (id BLOB NOT NULL PRIMARY KEY, collection_id BLOB NOT NULL, name TEXT NOT NULL, url TEXT NOT NULL, method TEXT NOT NULL, FOREIGN KEY (collection_id) REFERENCES collections (id));
	CREATE TABLE collection_items (id BLOB NOT NULL PRIMARY KEY, collection_id BLOB NOT NULL, parent_folder_id BLOB, item_type INT8 NOT NULL, folder_id BLOB, endpoint_id BLOB, name TEXT NOT NULL, prev_id BLOB, next_id BLOB, CHECK (item_type IN (0, 1)), CHECK ((item_type = 0 AND folder_id IS NOT NULL AND endpoint_id IS NULL) OR (item_type = 1 AND folder_id IS NULL AND endpoint_id IS NOT NULL)), FOREIGN KEY (collection_id) REFERENCES collections (id), FOREIGN KEY (folder_id) REFERENCES item_folder (id), FOREIGN KEY (endpoint_id) REFERENCES item_api (id));

	-- Performance indexes for cross-collection operations
	CREATE INDEX collections_workspace_lookup ON collections (id, workspace_id);
	CREATE INDEX collection_items_workspace_lookup ON collection_items (id, collection_id);
	CREATE INDEX workspaces_users_collection_access ON workspaces_users (user_id, workspace_id);
	CREATE INDEX collections_workspace_id_lookup ON collections (workspace_id, id);
	CREATE INDEX item_api_collection_update ON item_api (id, collection_id);
	CREATE INDEX item_folder_collection_update ON item_folder (id, collection_id);
	`
	
	_, err := db.Exec(schema)
	return err
}

// Example usage:
// go test -bench=BenchmarkCrossCollectionMoveOperations -benchmem
// go test -bench=BenchmarkLargeDatasetOperations -benchmem -timeout=10m