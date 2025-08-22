package sitemapiexample

import (
	"context"
	"testing"
	"time"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/testutil"
)

// TestAutoLinkBasicFunctionality tests the core auto-link functionality
func TestAutoLinkBasicFunctionality(t *testing.T) {
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

	t.Run("Empty endpoint auto-link", func(t *testing.T) {
		// Should not fail on empty endpoint
		err := service.AutoLinkIsolatedExamples(ctx, endpointID)
		if err != nil {
			t.Errorf("AutoLinkIsolatedExamples failed for empty endpoint: %v", err)
		}
	})

	t.Run("Single standalone example", func(t *testing.T) {
		// Create a standalone example (prev=NULL, next=NULL)
		exampleID := idwrap.NewNow()
		err := base.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
			ID:           exampleID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         "Standalone Example",
			IsDefault:    false,
			BodyType:     0,
			// No prev/next - standalone single node
		})
		if err != nil {
			t.Fatalf("Failed to create standalone example: %v", err)
		}

		// Test GetAllApiExamples (should find it)
		allExamples, err := service.GetAllApiExamples(ctx, endpointID)
		if err != nil {
			t.Fatalf("Failed to get all examples: %v", err)
		}
		if len(allExamples) != 1 {
			t.Errorf("Expected 1 example from GetAllApiExamples, got %d", len(allExamples))
		}

		// Test GetApiExamplesOrdered (should also find it - standalone nodes are valid heads)
		orderedExamples, err := service.GetApiExamplesOrdered(ctx, endpointID)
		if err != nil {
			t.Fatalf("Failed to get ordered examples: %v", err)
		}
		if len(orderedExamples) != 1 {
			t.Errorf("Expected 1 example from GetApiExamplesOrdered, got %d", len(orderedExamples))
		}

		// Auto-link should not change anything (no isolated examples)
		err = service.AutoLinkIsolatedExamples(ctx, endpointID)
		if err != nil {
			t.Errorf("AutoLinkIsolatedExamples failed: %v", err)
		}

		// Should still be 1 example
		orderedAfter, err := service.GetApiExamplesOrdered(ctx, endpointID)
		if err != nil {
			t.Fatalf("Failed to get ordered examples after auto-link: %v", err)
		}
		if len(orderedAfter) != 1 {
			t.Errorf("Expected 1 example after auto-link, got %d", len(orderedAfter))
		}
	})

	t.Run("Fallback integration test", func(t *testing.T) {
		// Verify that GetAllApiExamples works as a fallback
		allExamples, err := service.GetAllApiExamples(ctx, endpointID)
		if err != nil {
			t.Fatalf("Fallback GetAllApiExamples failed: %v", err)
		}

		orderedExamples, err := service.GetApiExamplesOrdered(ctx, endpointID)
		if err != nil {
			t.Fatalf("GetApiExamplesOrdered failed: %v", err)
		}

		// Both should return the same count for a healthy chain
		if len(allExamples) != len(orderedExamples) {
			t.Logf("GetAllApiExamples: %d, GetApiExamplesOrdered: %d", len(allExamples), len(orderedExamples))
			t.Errorf("Healthy chain should have same count from both queries")
		} else {
			t.Logf("✓ Both queries return %d examples", len(allExamples))
		}
	})
}