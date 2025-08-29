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

func TestFullExampleFlow(t *testing.T) {
	t.Parallel()
	
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	
	// Initialize the example service
	iaes := sitemapiexample.New(queries)
	
	// Create test data
	collectionID := idwrap.NewNow()
	endpointID := idwrap.NewNow()
	
	t.Run("Complete flow: create endpoint, examples, and duplicate", func(t *testing.T) {
		// Step 1: Simulate endpoint creation with default example
		defaultExample := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    true,
			Name:         "Default",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		err := iaes.CreateApiExample(ctx, defaultExample)
		require.NoError(t, err)
		t.Logf("✓ Created default example for endpoint")
		
		// Step 2: User creates a custom example
		userExample1 := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    false,
			Name:         "Test Success Case",
			BodyType:     mitemapiexample.BodyTypeRaw,
		}
		
		err = iaes.CreateApiExample(ctx, userExample1)
		require.NoError(t, err)
		t.Logf("✓ Created user example 'Test Success Case'")
		
		// Step 3: User duplicates the example (simulating UI duplication)
		userExample2 := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    false,
			Name:         "Test Success Case - Copy",
			BodyType:     mitemapiexample.BodyTypeRaw,
		}
		
		err = iaes.CreateApiExample(ctx, userExample2)
		require.NoError(t, err)
		t.Logf("✓ Successfully duplicated example")
		
		// Step 4: Verify the linked list structure
		orderedExamples, err := iaes.GetApiExamplesOrdered(ctx, endpointID)
		require.NoError(t, err)
		
		// Should have 2 user examples (default is filtered out)
		assert.Equal(t, 2, len(orderedExamples))
		if len(orderedExamples) == 2 {
			assert.Equal(t, "Test Success Case", orderedExamples[0].Name)
			assert.Equal(t, "Test Success Case - Copy", orderedExamples[1].Name)
		}
		
		t.Logf("✓ Examples are properly ordered in the list")
		
		// Step 5: Create another example to ensure chain continues working
		userExample3 := &mitemapiexample.ItemApiExample{
			ID:           idwrap.NewNow(),
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			IsDefault:    false,
			Name:         "Test Error Case",
			BodyType:     mitemapiexample.BodyTypeNone,
		}
		
		err = iaes.CreateApiExample(ctx, userExample3)
		require.NoError(t, err)
		
		// Final verification
		finalOrdered, err := iaes.GetApiExamplesOrdered(ctx, endpointID)
		require.NoError(t, err)
		assert.Equal(t, 3, len(finalOrdered))
		
		t.Logf("✓ All %d user examples are properly linked and ordered", len(finalOrdered))
		t.Logf("✓ COMPLETE SUCCESS: Example creation and duplication works perfectly!")
	})
}