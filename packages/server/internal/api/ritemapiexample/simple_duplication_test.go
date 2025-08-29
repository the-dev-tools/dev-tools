package ritemapiexample_test

import (
	"context"
	"testing"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleExampleDuplication(t *testing.T) {
	t.Parallel()
	
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	mockLogger := mocklogger.NewMockLogger()
	_ = mockLogger
	
	// Initialize the example service
	iaes := sitemapiexample.New(queries)
	
	// Create test data
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	
	t.Run("Analyze default example structure", func(t *testing.T) {
		// Create a default example (simulating what happens in endpoint creation)
		defaultExample := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    true,
			Name:         "Default",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		// Create the default example
		err := iaes.CreateItemApiExample(ctx, defaultExample)
		require.NoError(t, err)
		
		// Retrieve it to see the structure
		retrieved, err := iaes.GetDefaultApiExample(ctx, endpointID)
		require.NoError(t, err)
		
		t.Logf("Default example after creation:")
		t.Logf("  ID: %s", retrieved.ID.String())
		t.Logf("  Name: %s", retrieved.Name)
		t.Logf("  IsDefault: %v", retrieved.IsDefault)
		t.Logf("  Prev: %v", retrieved.Prev)
		t.Logf("  Next: %v", retrieved.Next)
		
		// Check for circular references
		hasCircularPrev := retrieved.Prev != nil && *retrieved.Prev == retrieved.ID
		hasCircularNext := retrieved.Next != nil && *retrieved.Next == retrieved.ID
		
		if hasCircularPrev || hasCircularNext {
			t.Logf("⚠ CIRCULAR REFERENCE DETECTED!")
			t.Logf("  Circular Prev: %v", hasCircularPrev)
			t.Logf("  Circular Next: %v", hasCircularNext)
			t.Logf("This is likely why duplication fails!")
		}
		
		// Try to duplicate using the service
		t.Logf("\nAttempting to duplicate default example...")
		
		// The duplication would need to:
		// 1. Copy the example
		// 2. Update the linked list pointers
		// 3. Handle the circular reference case
		
		duplicated := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    false, // Duplicates should not be default
			Name:         "Default Copy",
			BodyType:     retrieved.BodyType,
			Prev:         &retrieved.ID, // Point to the original
			Next:         nil,           // Last in the list
		}
		
		// Try to create the duplicate
		err = iaes.CreateItemApiExample(ctx, duplicated)
		if err != nil {
			t.Logf("✗ Failed to create duplicate: %v", err)
			t.Logf("Error indicates: %v", err)
		} else {
			t.Logf("✓ Created duplicate: %s", duplicated.ID.String())
			
			// Now we need to update the original's Next pointer
			// This is where the circular reference causes problems
			t.Logf("\nUpdating original's Next pointer...")
			
			// The issue: if Next points to itself, we need to break that
			// and point it to the new duplicate
		}
	})
	
	t.Run("Create multiple examples and check structure", func(t *testing.T) {
		endpointID2 := idwrap.NewNow()
		
		// Create first example
		ex1 := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID2,
			CollectionID: collectionID,
			IsDefault:    true,
			Name:         "First",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		err := iaes.CreateItemApiExample(ctx, ex1)
		require.NoError(t, err)
		
		// Retrieve to see structure
		retrieved1, err := iaes.GetItemApiExample(ctx, ex1.ID)
		require.NoError(t, err)
		
		t.Logf("First example:")
		t.Logf("  ID: %s, Prev: %v, Next: %v", 
			retrieved1.ID.String(), retrieved1.Prev, retrieved1.Next)
		
		// Create second example
		ex2 := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID2,
			CollectionID: collectionID,
			IsDefault:    false,
			Name:         "Second",
			BodyType:     mitemapiexample.BodyTypeNone,
			Prev:         &ex1.ID,
		}
		
		err = iaes.CreateItemApiExample(ctx, ex2)
		if err != nil {
			t.Logf("Failed to create second example: %v", err)
			
			// This might fail because of linked list constraints
			// The service might expect proper Next pointer updates
		} else {
			require.NoError(t, err)
			
			// Retrieve both to see final structure
			retrieved1Again, _ := iaes.GetItemApiExample(ctx, ex1.ID)
			retrieved2, _ := iaes.GetItemApiExample(ctx, ex2.ID)
			
			t.Logf("\nAfter creating second example:")
			t.Logf("  First:  ID=%s, Prev=%v, Next=%v",
				retrieved1Again.ID.String(), retrieved1Again.Prev, retrieved1Again.Next)
			t.Logf("  Second: ID=%s, Prev=%v, Next=%v",
				retrieved2.ID.String(), retrieved2.Prev, retrieved2.Next)
		}
	})
	
	t.Run("Test GetExamplesByEndpointIDOrdered", func(t *testing.T) {
		endpointID3 := idwrap.NewNow()
		
		// Create a default example with the new fix (always isDefault = true)
		defaultEx := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID3,
			CollectionID: collectionID,
			IsDefault:    true,
			Name:         "Default",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		err := iaes.CreateItemApiExample(ctx, defaultEx)
		require.NoError(t, err)
		
		// Try to get examples using the ordered query
		examples, err := iaes.GetExamplesByEndpointIDOrdered(ctx, endpointID3)
		if err != nil {
			t.Logf("✗ GetExamplesByEndpointIDOrdered failed: %v", err)
			t.Logf("This might be due to SQL filtering issues")
		} else {
			t.Logf("✓ GetExamplesByEndpointIDOrdered returned %d examples", len(examples))
			for i, ex := range examples {
				t.Logf("  %d: ID=%s, Name=%s, IsDefault=%v, Prev=%v, Next=%v",
					i, ex.ID.String(), ex.Name, ex.IsDefault, ex.Prev, ex.Next)
			}
			
			// Check if default examples are being filtered out
			if len(examples) == 0 && defaultEx.IsDefault {
				t.Logf("⚠ WARNING: Default example not returned by GetExamplesByEndpointIDOrdered!")
				t.Logf("This suggests the SQL query filters out is_default = TRUE")
			}
		}
		
		// Also try GetDefaultApiExample
		defaultRetrieved, err := iaes.GetDefaultApiExample(ctx, endpointID3)
		if err != nil {
			t.Logf("✗ GetDefaultApiExample failed: %v", err)
		} else {
			t.Logf("✓ GetDefaultApiExample found: %s", defaultRetrieved.Name)
			assert.True(t, defaultRetrieved.IsDefault)
		}
	})
}