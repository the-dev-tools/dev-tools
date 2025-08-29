package sitemapiexample

import (
	"context"
	"testing"
	"time"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/testutil"
)

// TestCreateApiExampleLinking verifies that CreateApiExample properly links examples into the linked list
func TestCreateApiExampleLinking(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	// Setup test data
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	globalEnvID := idwrap.NewNow()

	err := base.Queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	err = base.Queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         time.Now().Unix(),
		CollectionCount: 0,
		FlowCount:       0,
		ActiveEnv:       globalEnvID,
		GlobalEnv:       globalEnvID,
	})
	if err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	collectionID := idwrap.NewNow()
	err = base.Queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
	})
	if err != nil {
		t.Fatalf("Failed to create test collection: %v", err)
	}

	endpointID := idwrap.NewNow()
	err = base.Queries.CreateItemApi(ctx, gen.CreateItemApiParams{
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

	service := New(base.Queries)

	t.Run("Create multiple examples and verify they are properly linked", func(t *testing.T) {
		// Create 3 examples
		example1ID := idwrap.NewNow()
		example2ID := idwrap.NewNow()
		example3ID := idwrap.NewNow()

		example1 := &mitemapiexample.ItemApiExample{
			ID:           example1ID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         "Example 1",
			IsDefault:    false,
			BodyType:     0,
		}

		example2 := &mitemapiexample.ItemApiExample{
			ID:           example2ID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         "Example 2",
			IsDefault:    false,
			BodyType:     0,
		}

		example3 := &mitemapiexample.ItemApiExample{
			ID:           example3ID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         "Example 3",
			IsDefault:    false,
			BodyType:     0,
		}

		// Create examples using the service
		err = service.CreateApiExample(ctx, example1)
		if err != nil {
			t.Fatalf("Failed to create example1: %v", err)
		}

		err = service.CreateApiExample(ctx, example2)
		if err != nil {
			t.Fatalf("Failed to create example2: %v", err)
		}

		err = service.CreateApiExample(ctx, example3)
		if err != nil {
			t.Fatalf("Failed to create example3: %v", err)
		}

		// Test 1: All 3 examples should be persisted
		allExamples, err := service.GetAllApiExamples(ctx, endpointID)
		if err != nil {
			t.Fatalf("Failed to get all examples: %v", err)
		}
		if len(allExamples) != 3 {
			t.Errorf("Expected 3 examples from GetAllApiExamples, got %d", len(allExamples))
		}

		// Debug: Let's see what the raw database state is
		rawExamples, err := base.Queries.GetItemApiExamples(ctx, endpointID)
		if err != nil {
			t.Fatalf("Failed to get raw examples: %v", err)
		}
		t.Logf("Raw examples in database:")
		for i, ex := range rawExamples {
			prevStr := "nil"
			nextStr := "nil" 
			if ex.Prev != nil {
				prevStr = ex.Prev.String()
			}
			if ex.Next != nil {
				nextStr = ex.Next.String()
			}
			t.Logf("  [%d] ID=%s, prev=%s, next=%s", i, ex.ID.String(), prevStr, nextStr)
		}

		// Test 2: GetApiExamplesOrdered should return all 3 (no broken linked list)
		orderedExamples, err := service.GetApiExamplesOrdered(ctx, endpointID)
		if err != nil {
			t.Fatalf("Failed to get ordered examples: %v", err)
		}
		if len(orderedExamples) != 3 {
			t.Errorf("Expected 3 examples from GetApiExamplesOrdered, got %d", len(orderedExamples))
		}

		// Test 3: Both queries should return the same count (no isolated examples)
		if len(allExamples) != len(orderedExamples) {
			t.Errorf("Mismatch: GetAllApiExamples returned %d, GetApiExamplesOrdered returned %d", 
				len(allExamples), len(orderedExamples))
		}

		// Test 4: Verify the linked list structure is valid
		err = service.movableRepository.ValidateLinkedListIntegrity(ctx, endpointID)
		if err != nil {
			t.Errorf("Linked list integrity validation failed: %v", err)
		}

		// Test 5: No isolated examples should be detected
		isolatedExamples, err := service.movableRepository.DetectIsolatedExamples(ctx, endpointID)
		if err != nil {
			t.Errorf("Failed to detect isolated examples: %v", err)
		}
		if len(isolatedExamples) > 0 {
			t.Errorf("Found %d isolated examples, expected 0", len(isolatedExamples))
		}

		t.Logf("✓ Successfully created and linked 3 examples")
		t.Logf("✓ GetAllApiExamples: %d examples", len(allExamples))
		t.Logf("✓ GetApiExamplesOrdered: %d examples", len(orderedExamples))
		t.Logf("✓ No isolated examples detected")
		t.Logf("✓ Linked list integrity validated")
	})
}

// TestBulkCreateApiExampleLinking verifies that bulk creation also works properly
func TestBulkCreateApiExampleLinking(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	// Setup test data
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	globalEnvID := idwrap.NewNow()

	err := base.Queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	err = base.Queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:              workspaceID,
		Name:            "Test Workspace",
		Updated:         time.Now().Unix(),
		CollectionCount: 0,
		FlowCount:       0,
		ActiveEnv:       globalEnvID,
		GlobalEnv:       globalEnvID,
	})
	if err != nil {
		t.Fatalf("Failed to create test workspace: %v", err)
	}

	collectionID := idwrap.NewNow()
	err = base.Queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
	})
	if err != nil {
		t.Fatalf("Failed to create test collection: %v", err)
	}

	endpointID := idwrap.NewNow()
	err = base.Queries.CreateItemApi(ctx, gen.CreateItemApiParams{
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

	service := New(base.Queries)

	t.Run("Bulk create multiple examples and verify they are properly linked", func(t *testing.T) {
		// Create 3 examples for bulk creation
		examples := []mitemapiexample.ItemApiExample{
			{
				ID:           idwrap.NewNow(),
				ItemApiID:    endpointID,
				CollectionID: collectionID,
				Name:         "Bulk Example 1",
				IsDefault:    false,
				BodyType:     0,
			},
			{
				ID:           idwrap.NewNow(),
				ItemApiID:    endpointID,
				CollectionID: collectionID,
				Name:         "Bulk Example 2",
				IsDefault:    false,
				BodyType:     0,
			},
			{
				ID:           idwrap.NewNow(),
				ItemApiID:    endpointID,
				CollectionID: collectionID,
				Name:         "Bulk Example 3",
				IsDefault:    false,
				BodyType:     0,
			},
		}

		// Create examples using bulk method
		err = service.CreateApiExampleBulk(ctx, examples)
		if err != nil {
			t.Fatalf("Failed to create bulk examples: %v", err)
		}

		// Verify all examples are properly linked
		allExamples, err := service.GetAllApiExamples(ctx, endpointID)
		if err != nil {
			t.Fatalf("Failed to get all examples: %v", err)
		}
		if len(allExamples) != 3 {
			t.Errorf("Expected 3 examples from GetAllApiExamples, got %d", len(allExamples))
		}

		orderedExamples, err := service.GetApiExamplesOrdered(ctx, endpointID)
		if err != nil {
			t.Fatalf("Failed to get ordered examples: %v", err)
		}
		if len(orderedExamples) != 3 {
			t.Errorf("Expected 3 examples from GetApiExamplesOrdered, got %d", len(orderedExamples))
		}

		// Verify no isolated examples
		isolatedExamples, err := service.movableRepository.DetectIsolatedExamples(ctx, endpointID)
		if err != nil {
			t.Errorf("Failed to detect isolated examples: %v", err)
		}
		if len(isolatedExamples) > 0 {
			t.Errorf("Found %d isolated examples, expected 0", len(isolatedExamples))
		}

		t.Logf("✓ Successfully bulk created and linked 3 examples")
	})
}