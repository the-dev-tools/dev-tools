package sitemapiexample_test

import (
	"context"
	"testing"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/testutil"
)

func TestExampleMovableRepository_BasicOperations(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	queries := base.Queries
	
	// Create test workspace
	workspaceID := idwrap.NewNow()
	activeEnvID := idwrap.NewNow()
	globalEnvID := idwrap.NewNow()
	err := queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         1234567890,
		CollectionCount: 0,
		FlowCount:       0,
		ActiveEnv:       activeEnvID,
		GlobalEnv:       globalEnvID,
	})
	if err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}
	
	// Create test collection
	collectionID := idwrap.NewNow()
	err = queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
	})
	if err != nil {
		t.Fatalf("Failed to create test collection: %v", err)
	}
	
	// Create test endpoint
	endpointID := idwrap.NewNow()
	err = queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Test Endpoint",
		Url:          "/test",
		Method:       "GET",
		Hidden:       false,
	})
	if err != nil {
		t.Fatalf("Failed to create test endpoint: %v", err)
	}
	
	// Create repository
	repo := sitemapiexample.NewExampleMovableRepository(queries)
	
	// Create test examples with proper linked list structure
	example1ID := idwrap.NewNow()
	example2ID := idwrap.NewNow()
	
	// Example 1 is head (prev=NULL, next=example2)
	err = queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           example1ID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Example 1",
		IsDefault:    false,
		BodyType:     0,
		Next:         &example2ID,
	})
	if err != nil {
		t.Fatalf("Failed to create example 1: %v", err)
	}
	
	// Example 2 is tail (prev=example1, next=NULL)
	err = queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           example2ID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Example 2",
		IsDefault:    false,
		BodyType:     0,
		Prev:         &example1ID,
	})
	if err != nil {
		t.Fatalf("Failed to create example 2: %v", err)
	}
	
	// Test GetItemsByParent
	listType := movable.CollectionListTypeExamples
	items, err := repo.GetItemsByParent(ctx, endpointID, listType)
	if err != nil {
		t.Fatalf("Failed to get items by parent: %v", err)
	}
	
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}
	
	// Test GetMaxPosition
	maxPos, err := repo.GetMaxPosition(ctx, endpointID, listType)
	if err != nil {
		t.Fatalf("Failed to get max position: %v", err)
	}
	
	expectedMaxPos := len(items) - 1
	if maxPos != expectedMaxPos {
		t.Errorf("Expected max position %d, got %d", expectedMaxPos, maxPos)
	}
	
	// Test UpdatePosition
	if len(items) >= 2 {
		// Move first item to position 1
		err = repo.UpdatePosition(ctx, nil, items[0].ID, listType, 1)
		if err != nil {
			t.Fatalf("Failed to update position: %v", err)
		}
		
		// Verify the position change
		updatedItems, err := repo.GetItemsByParent(ctx, endpointID, listType)
		if err != nil {
			t.Fatalf("Failed to get updated items: %v", err)
		}
		
		if len(updatedItems) != 2 {
			t.Errorf("Expected 2 items after reorder, got %d", len(updatedItems))
		}
		
		// The first item (originally at position 0) should now be at position 1
		var foundAtPosition1 bool
		for _, item := range updatedItems {
			if item.ID == items[0].ID && item.Position == 1 {
				foundAtPosition1 = true
				break
			}
		}
		
		if !foundAtPosition1 {
			t.Errorf("Expected moved item (ID=%s) to be at position 1", items[0].ID.String())
		}
	}
}

func TestExampleMovableRepository_EmptyEndpoint(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()
	queries := base.Queries
	
	// Create test workspace and collection
	workspaceID := idwrap.NewNow()
	activeEnvID := idwrap.NewNow()
	globalEnvID := idwrap.NewNow()
	err := queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         1234567890,
		CollectionCount: 0,
		FlowCount:       0,
		ActiveEnv:       activeEnvID,
		GlobalEnv:       globalEnvID,
	})
	if err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}
	
	collectionID := idwrap.NewNow()
	err = queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
	})
	if err != nil {
		t.Fatalf("Failed to create test collection: %v", err)
	}
	
	// Create endpoint but no examples
	endpointID := idwrap.NewNow()
	err = queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Empty Endpoint",
		Url:          "/empty",
		Method:       "GET",
		Hidden:       false,
	})
	if err != nil {
		t.Fatalf("Failed to create test endpoint: %v", err)
	}
	
	repo := sitemapiexample.NewExampleMovableRepository(queries)
	
	// Test empty endpoint
	listType := movable.CollectionListTypeExamples
	items, err := repo.GetItemsByParent(ctx, endpointID, listType)
	if err != nil {
		t.Fatalf("Failed to get items from empty endpoint: %v", err)
	}
	
	if len(items) != 0 {
		t.Errorf("Expected 0 items from empty endpoint, got %d", len(items))
	}
	
	// Test max position on empty endpoint
	maxPos, err := repo.GetMaxPosition(ctx, endpointID, listType)
	if err != nil {
		t.Fatalf("Failed to get max position from empty endpoint: %v", err)
	}
	
	if maxPos != -1 {
		t.Errorf("Expected max position -1 from empty endpoint, got %d", maxPos)
	}
}