package sitemapiexample_test

import (
	"context"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExampleDuplicationWithDefaultExample(t *testing.T) {
	t.Parallel()
	
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	
	// Initialize the example service
	iaes := sitemapiexample.New(queries)
	
	// Create test data
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	
	t.Run("Create default and user examples then duplicate", func(t *testing.T) {
		// Step 1: Create a default example (like what happens in endpoint creation)
		defaultExample := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    true,
			Name:         "Default",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		err := iaes.CreateApiExample(ctx, defaultExample)
		require.NoError(t, err, "Should create default example")
		
		// Verify default example was created
		retrieved, err := iaes.GetDefaultApiExample(ctx, endpointID)
		require.NoError(t, err)
		assert.Equal(t, "Default", retrieved.Name)
		assert.True(t, retrieved.IsDefault)
		assert.Nil(t, retrieved.Prev, "Default should have no prev")
		assert.Nil(t, retrieved.Next, "Default should have no next")
		
		t.Logf("✓ Created default example: isolated with prev=nil, next=nil")
		
		// Step 2: Create a user example (simulating user creating an example)
		userExample1 := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    false,
			Name:         "User Example 1",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		err = iaes.CreateApiExample(ctx, userExample1)
		require.NoError(t, err, "Should create first user example")
		
		// Check the structure
		retrieved1, err := iaes.GetApiExample(ctx, userExample1.ID)
		require.NoError(t, err)
		assert.Nil(t, retrieved1.Prev, "First user example should have prev=nil (head of user chain)")
		assert.Nil(t, retrieved1.Next, "First user example should have next=nil (also tail)")
		
		t.Logf("✓ Created first user example: head of user chain with prev=nil")
		
		// Step 3: Create another user example (simulating duplication)
		userExample2 := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    false,
			Name:         "User Example 1 - Copy",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		err = iaes.CreateApiExample(ctx, userExample2)
		require.NoError(t, err, "Should create second user example (duplication)")
		
		// Verify the linked list structure
		retrieved1Again, err := iaes.GetApiExample(ctx, userExample1.ID)
		require.NoError(t, err)
		retrieved2, err := iaes.GetApiExample(ctx, userExample2.ID)
		require.NoError(t, err)
		
		// Check that user examples form their own chain
		assert.Nil(t, retrieved1Again.Prev, "First user example should still have prev=nil")
		assert.NotNil(t, retrieved1Again.Next, "First user example should now point to second")
		if retrieved1Again.Next != nil {
			assert.Equal(t, userExample2.ID, *retrieved1Again.Next, "First should point to second")
		}
		
		assert.NotNil(t, retrieved2.Prev, "Second user example should have prev")
		if retrieved2.Prev != nil {
			assert.Equal(t, userExample1.ID, *retrieved2.Prev, "Second should point back to first")
		}
		assert.Nil(t, retrieved2.Next, "Second user example should be tail")
		
		t.Logf("✓ User examples form proper chain: Example1 -> Example2")
		
		// Step 4: Verify default example is still isolated
		defaultAgain, err := iaes.GetDefaultApiExample(ctx, endpointID)
		require.NoError(t, err)
		assert.Nil(t, defaultAgain.Prev, "Default should still have no prev")
		assert.Nil(t, defaultAgain.Next, "Default should still have no next")
		
		t.Logf("✓ Default example remains isolated from user chain")
		
		// Step 5: Get ordered examples (user view)
		orderedExamples, err := iaes.GetApiExamplesOrdered(ctx, endpointID)
		require.NoError(t, err, "Should get ordered examples")
		
		// Should return only user examples in order (default filtered out)
		assert.Equal(t, 2, len(orderedExamples), "Should have 2 user examples")
		if len(orderedExamples) == 2 {
			assert.Equal(t, "User Example 1", orderedExamples[0].Name)
			assert.Equal(t, "User Example 1 - Copy", orderedExamples[1].Name)
		}
		
		t.Logf("✓ GetApiExamplesOrdered returns user examples in correct order")
		
		// Step 6: Create a third example to ensure chain continues working
		userExample3 := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    false,
			Name:         "User Example 3",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		err = iaes.CreateApiExample(ctx, userExample3)
		require.NoError(t, err, "Should create third user example")
		
		// Final verification
		orderedFinal, err := iaes.GetApiExamplesOrdered(ctx, endpointID)
		require.NoError(t, err)
		assert.Equal(t, 3, len(orderedFinal), "Should have 3 user examples")
		
		t.Logf("✓ DUPLICATION WORKS! Chain maintains integrity with multiple examples")
	})
}