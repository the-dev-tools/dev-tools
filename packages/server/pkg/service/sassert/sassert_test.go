package sassert

import (
	"context"
	"testing"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/testutil"
)

// TestAppendAssert tests the core linked-list append functionality
func TestAppendAssert(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	// Setup test data
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	// Create base entities
	err := base.Queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	err = base.Queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:        workspaceID,
		Name:      "Test Workspace",
		ActiveEnv: idwrap.NewNow(),
		GlobalEnv: idwrap.NewNow(),
	})
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	err = base.Queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
	})
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	err = base.Queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Test Endpoint",
		Url:          "https://api.example.com/test",
		Method:       "GET",
	})
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	err = base.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Test Example",
	})
	if err != nil {
		t.Fatalf("Failed to create example: %v", err)
	}

	// Create assertion service
	as := New(base.Queries)

	t.Run("AppendAssert creates first assertion with no prev/next", func(t *testing.T) {
		assert1 := massert.Assert{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "status == 200",
				},
			},
			Enable: true,
		}

		err := as.AppendAssert(ctx, assert1)
		if err != nil {
			t.Fatalf("Failed to append first assertion: %v", err)
		}

		// Verify the assertion was created
		retrieved, err := as.GetAssert(ctx, assert1.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve assertion: %v", err)
		}

		if retrieved.Prev != nil {
			t.Errorf("Expected first assertion to have nil prev, got: %v", retrieved.Prev)
		}
		if retrieved.Next != nil {
			t.Errorf("Expected first assertion to have nil next, got: %v", retrieved.Next)
		}
		if retrieved.Condition.Comparisons.Expression != "status == 200" {
			t.Errorf("Expected expression 'status == 200', got: %s", retrieved.Condition.Comparisons.Expression)
		}
	})

	t.Run("AppendAssert creates proper linked list with multiple assertions", func(t *testing.T) {
		assert2 := massert.Assert{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "response.data.length > 0",
				},
			},
			Enable: true,
		}

		assert3 := massert.Assert{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "response.time < 1000",
				},
			},
			Enable: true,
		}

		// Append second assertion
		err := as.AppendAssert(ctx, assert2)
		if err != nil {
			t.Fatalf("Failed to append second assertion: %v", err)
		}

		// Append third assertion
		err = as.AppendAssert(ctx, assert3)
		if err != nil {
			t.Fatalf("Failed to append third assertion: %v", err)
		}

		// Get all assertions in order
		orderedAsserts, err := as.GetAssertsOrdered(ctx, exampleID)
		if err != nil {
			t.Fatalf("Failed to get ordered assertions: %v", err)
		}

		if len(orderedAsserts) != 3 {
			t.Fatalf("Expected 3 assertions, got %d", len(orderedAsserts))
		}

		// Verify linking is correct
		// First assertion should have no prev, next pointing to second
		first := orderedAsserts[0]
		if first.Prev != nil {
			t.Errorf("First assertion should have nil prev")
		}
		if first.Next == nil || first.Next.Compare(orderedAsserts[1].ID) != 0 {
			t.Errorf("First assertion next should point to second assertion")
		}

		// Second assertion should have prev pointing to first, next pointing to third
		second := orderedAsserts[1]
		if second.Prev == nil || second.Prev.Compare(orderedAsserts[0].ID) != 0 {
			t.Errorf("Second assertion prev should point to first assertion")
		}
		if second.Next == nil || second.Next.Compare(orderedAsserts[2].ID) != 0 {
			t.Errorf("Second assertion next should point to third assertion")
		}

		// Third assertion should have prev pointing to second, no next
		third := orderedAsserts[2]
		if third.Prev == nil || third.Prev.Compare(orderedAsserts[1].ID) != 0 {
			t.Errorf("Third assertion prev should point to second assertion")
		}
		if third.Next != nil {
			t.Errorf("Third assertion should have nil next")
		}

		// Verify expressions are correct
		expectedExpressions := []string{
			"status == 200",
			"response.data.length > 0",
			"response.time < 1000",
		}
		for i, assert := range orderedAsserts {
			if assert.Condition.Comparisons.Expression != expectedExpressions[i] {
				t.Errorf("Assertion %d: expected expression '%s', got '%s'", 
					i, expectedExpressions[i], assert.Condition.Comparisons.Expression)
			}
		}
	})
}

// TestAppendBulkAssert tests bulk assertion creation with proper linking
func TestAppendBulkAssert(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	// Setup test data
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	// Create base entities (same as above)
	err := base.Queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	err = base.Queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:        workspaceID,
		Name:      "Test Workspace",
		ActiveEnv: idwrap.NewNow(),
		GlobalEnv: idwrap.NewNow(),
	})
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	err = base.Queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
	})
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	err = base.Queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Test Endpoint",
		Url:          "https://api.example.com/test",
		Method:       "GET",
	})
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	err = base.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Test Example",
	})
	if err != nil {
		t.Fatalf("Failed to create example: %v", err)
	}

	// Create assertion service
	as := New(base.Queries)

	t.Run("AppendBulkAssert creates proper linked list", func(t *testing.T) {
		assertions := []massert.Assert{
			{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				Condition: mcondition.Condition{
					Comparisons: mcondition.Comparison{
						Expression: "status == 200",
					},
				},
				Enable: true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				Condition: mcondition.Condition{
					Comparisons: mcondition.Comparison{
						Expression: "response.data != null",
					},
				},
				Enable: true,
			},
			{
				ID:        idwrap.NewNow(),
				ExampleID: exampleID,
				Condition: mcondition.Condition{
					Comparisons: mcondition.Comparison{
						Expression: "response.success == true",
					},
				},
				Enable: true,
			},
		}

		err := as.AppendBulkAssert(ctx, assertions)
		if err != nil {
			t.Fatalf("Failed to append bulk assertions: %v", err)
		}

		// Get all assertions in order
		orderedAsserts, err := as.GetAssertsOrdered(ctx, exampleID)
		if err != nil {
			t.Fatalf("Failed to get ordered assertions: %v", err)
		}

		if len(orderedAsserts) != 3 {
			t.Fatalf("Expected 3 assertions, got %d", len(orderedAsserts))
		}

		// Verify proper linking
		for i, assert := range orderedAsserts {
			if i == 0 {
				// First assertion
				if assert.Prev != nil {
					t.Errorf("First assertion should have nil prev")
				}
				if assert.Next == nil {
					t.Errorf("First assertion should have next")
				}
			} else if i == len(orderedAsserts)-1 {
				// Last assertion
				if assert.Prev == nil {
					t.Errorf("Last assertion should have prev")
				}
				if assert.Next != nil {
					t.Errorf("Last assertion should have nil next")
				}
			} else {
				// Middle assertions
				if assert.Prev == nil {
					t.Errorf("Middle assertion should have prev")
				}
				if assert.Next == nil {
					t.Errorf("Middle assertion should have next")
				}
			}
		}
	})
}

// TestUnlinkAssert tests removing assertions from the linked list
func TestUnlinkAssert(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	// Setup test data
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	// Create base entities
	err := base.Queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	err = base.Queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:        workspaceID,
		Name:      "Test Workspace",
		ActiveEnv: idwrap.NewNow(),
		GlobalEnv: idwrap.NewNow(),
	})
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	err = base.Queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
	})
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	err = base.Queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Test Endpoint",
		Url:          "https://api.example.com/test",
		Method:       "GET",
	})
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	err = base.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Test Example",
	})
	if err != nil {
		t.Fatalf("Failed to create example: %v", err)
	}

	// Create assertion service
	as := New(base.Queries)

	// Create test assertions
	assertions := []massert.Assert{
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "status == 200",
				},
			},
			Enable: true,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "response.data != null",
				},
			},
			Enable: true,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "response.success == true",
				},
			},
			Enable: true,
		},
	}

	err = as.AppendBulkAssert(ctx, assertions)
	if err != nil {
		t.Fatalf("Failed to create test assertions: %v", err)
	}

	t.Run("UnlinkAssert removes middle assertion and maintains links", func(t *testing.T) {
		// Remove the middle assertion
		err := as.UnlinkAssert(ctx, assertions[1].ID)
		if err != nil {
			t.Fatalf("Failed to unlink assertion: %v", err)
		}

		// Get remaining assertions
		orderedAsserts, err := as.GetAssertsOrdered(ctx, exampleID)
		if err != nil {
			t.Fatalf("Failed to get ordered assertions: %v", err)
		}

		if len(orderedAsserts) != 2 {
			t.Fatalf("Expected 2 assertions after unlinking, got %d", len(orderedAsserts))
		}

		// Verify the first and third assertions are now linked
		first := orderedAsserts[0]
		third := orderedAsserts[1]

		if first.Next == nil || first.Next.Compare(third.ID) != 0 {
			t.Errorf("First assertion should point to third assertion")
		}
		if third.Prev == nil || third.Prev.Compare(first.ID) != 0 {
			t.Errorf("Third assertion should point back to first assertion")
		}

		// Verify expressions
		if first.Condition.Comparisons.Expression != "status == 200" {
			t.Errorf("First assertion expression incorrect")
		}
		if third.Condition.Comparisons.Expression != "response.success == true" {
			t.Errorf("Third assertion expression incorrect")
		}
	})
}

// TestMoveAssert tests moving assertions within the linked list
func TestMoveAssert(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	defer base.Close()

	// Setup test data (similar setup as above)
	userID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	exampleID := idwrap.NewNow()

	// Create base entities
	err := base.Queries.CreateUser(ctx, gen.CreateUserParams{
		ID:    userID,
		Email: "test@example.com",
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	err = base.Queries.CreateWorkspace(ctx, gen.CreateWorkspaceParams{
		ID:        workspaceID,
		Name:      "Test Workspace",
		ActiveEnv: idwrap.NewNow(),
		GlobalEnv: idwrap.NewNow(),
	})
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	err = base.Queries.CreateCollection(ctx, gen.CreateCollectionParams{
		ID:          collectionID,
		WorkspaceID: workspaceID,
		Name:        "Test Collection",
	})
	if err != nil {
		t.Fatalf("Failed to create collection: %v", err)
	}

	err = base.Queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "Test Endpoint",
		Url:          "https://api.example.com/test",
		Method:       "GET",
	})
	if err != nil {
		t.Fatalf("Failed to create endpoint: %v", err)
	}

	err = base.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Test Example",
	})
	if err != nil {
		t.Fatalf("Failed to create example: %v", err)
	}

	// Create assertion service
	as := New(base.Queries)

	// Create test assertions with identifiable expressions
	assertions := []massert.Assert{
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "first",
				},
			},
			Enable: true,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "second",
				},
			},
			Enable: true,
		},
		{
			ID:        idwrap.NewNow(),
			ExampleID: exampleID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: "third",
				},
			},
			Enable: true,
		},
	}

	err = as.AppendBulkAssert(ctx, assertions)
	if err != nil {
		t.Fatalf("Failed to create test assertions: %v", err)
	}

	t.Run("MoveAssert moves first assertion after second", func(t *testing.T) {
		// Move first assertion after second: [first, second, third] -> [second, first, third]
		err := as.MoveAssert(ctx, assertions[0].ID, &assertions[1].ID, nil, exampleID)
		if err != nil {
			t.Fatalf("Failed to move assertion: %v", err)
		}

		// Get assertions in order
		orderedAsserts, err := as.GetAssertsOrdered(ctx, exampleID)
		if err != nil {
			t.Fatalf("Failed to get ordered assertions: %v", err)
		}

		if len(orderedAsserts) != 3 {
			t.Fatalf("Expected 3 assertions, got %d", len(orderedAsserts))
		}

		// Verify new order
		expectedOrder := []string{"second", "first", "third"}
		for i, assert := range orderedAsserts {
			if assert.Condition.Comparisons.Expression != expectedOrder[i] {
				t.Errorf("Position %d: expected '%s', got '%s'", 
					i, expectedOrder[i], assert.Condition.Comparisons.Expression)
			}
		}
	})
}