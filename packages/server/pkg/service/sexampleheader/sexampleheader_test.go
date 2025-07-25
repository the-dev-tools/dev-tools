package sexampleheader_test

import (
	"context"
	"testing"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestExampleForService(t *testing.T, ctx context.Context, queries *gen.Queries) idwrap.IDWrap {
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	CollectionID := idwrap.NewNow()
	UserID := idwrap.NewNow()

	base := &testutil.BaseDBQueries{Queries: queries}
	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID,
		workspaceUserID, UserID, CollectionID)

	// Create item api
	itemAPIID := idwrap.NewNow()
	err := queries.CreateItemApi(ctx, gen.CreateItemApiParams{
		ID:           itemAPIID,
		CollectionID: CollectionID,
		Name:         "test api",
		Url:          "http://test.com",
		Method:       "GET",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create test example
	exampleID := idwrap.NewNow()
	err = queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:           exampleID,
		ItemApiID:    itemAPIID,
		CollectionID: CollectionID,
		IsDefault:    true,
		BodyType:     0,
		Name:         "test example",
		Prev:         nil,
		Next:         nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	return exampleID
}

func TestHeaderService_MoveHeader(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	service := sexampleheader.New(base.Queries)

	exampleID := setupTestExampleForService(t, ctx, base.Queries)

	// Create three headers: header1 -> header2 -> header3
	header1ID := idwrap.NewNow()
	header2ID := idwrap.NewNow()
	header3ID := idwrap.NewNow()

	headers := []mexampleheader.Header{
		{
			ID:          header1ID,
			ExampleID:   exampleID,
			HeaderKey:   "header1",
			Enable:      true,
			Description: "First header",
			Value:       "value1",
		},
		{
			ID:          header2ID,
			ExampleID:   exampleID,
			HeaderKey:   "header2",
			Enable:      true,
			Description: "Second header",
			Value:       "value2",
		},
		{
			ID:          header3ID,
			ExampleID:   exampleID,
			HeaderKey:   "header3",
			Enable:      true,
			Description: "Third header",
			Value:       "value3",
		},
	}

	// Create headers
	for _, header := range headers {
		err := service.CreateHeader(ctx, header)
		require.NoError(t, err)
	}

	// Set up initial order: header1 -> header2 -> header3
	err := base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
		ID:   header1ID,
		Prev: nil,
		Next: header2ID.Bytes(),
	})
	require.NoError(t, err)

	err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
		ID:   header2ID,
		Prev: header1ID.Bytes(),
		Next: header3ID.Bytes(),
	})
	require.NoError(t, err)

	err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
		ID:   header3ID,
		Prev: header2ID.Bytes(),
		Next: nil,
	})
	require.NoError(t, err)

	t.Run("MoveHeader_Before", func(t *testing.T) {
		// Move header3 before header1: header3 -> header1 -> header2
		err := service.MoveHeader(ctx, header3ID, header1ID, "before")
		require.NoError(t, err)

		// Verify the new order
		orderedHeaders, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedHeaders, 3)

		assert.Equal(t, header3ID, orderedHeaders[0].ID)
		assert.Equal(t, header1ID, orderedHeaders[1].ID)
		assert.Equal(t, header2ID, orderedHeaders[2].ID)
	})

	t.Run("MoveHeader_After", func(t *testing.T) {
		// Reset to original order first
		err := base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header1ID,
			Prev: nil,
			Next: header2ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header2ID,
			Prev: header1ID.Bytes(),
			Next: header3ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header3ID,
			Prev: header2ID.Bytes(),
			Next: nil,
		})
		require.NoError(t, err)

		// Move header1 after header3: header2 -> header3 -> header1
		err = service.MoveHeader(ctx, header1ID, header3ID, "after")
		require.NoError(t, err)

		// Verify the new order
		orderedHeaders, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedHeaders, 3)

		assert.Equal(t, header2ID, orderedHeaders[0].ID)
		assert.Equal(t, header3ID, orderedHeaders[1].ID)
		assert.Equal(t, header1ID, orderedHeaders[2].ID)
	})
}

func TestHeaderService_GetHeadersByExampleIDOrdered(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	service := sexampleheader.New(base.Queries)

	exampleID := setupTestExampleForService(t, ctx, base.Queries)

	t.Run("EmptyList", func(t *testing.T) {
		headers, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		assert.Len(t, headers, 0)
	})

	t.Run("SingleHeader", func(t *testing.T) {
		headerID := idwrap.NewNow()
		header := mexampleheader.Header{
			ID:          headerID,
			ExampleID:   exampleID,
			HeaderKey:   "single-header",
			Enable:      true,
			Description: "Single header",
			Value:       "single-value",
		}

		err := service.CreateHeader(ctx, header)
		require.NoError(t, err)

		headers, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, headers, 1)
		assert.Equal(t, headerID, headers[0].ID)
		assert.Nil(t, headers[0].Prev)
		assert.Nil(t, headers[0].Next)
	})

	t.Run("MultipleHeaders", func(t *testing.T) {
		// Clean up existing headers
		existingHeaders, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		for _, h := range existingHeaders {
			err := service.DeleteHeader(ctx, h.ID)
			require.NoError(t, err)
		}

		// Create headers in specific order
		header1ID := idwrap.NewNow()
		header2ID := idwrap.NewNow()
		header3ID := idwrap.NewNow()

		headers := []mexampleheader.Header{
			{
				ID:          header1ID,
				ExampleID:   exampleID,
				HeaderKey:   "first",
				Enable:      true,
				Description: "First",
				Value:       "value1",
			},
			{
				ID:          header2ID,
				ExampleID:   exampleID,
				HeaderKey:   "second",
				Enable:      true,
				Description: "Second",
				Value:       "value2",
			},
			{
				ID:          header3ID,
				ExampleID:   exampleID,
				HeaderKey:   "third",
				Enable:      true,
				Description: "Third",
				Value:       "value3",
			},
		}

		// Create headers
		for _, header := range headers {
			err := service.CreateHeader(ctx, header)
			require.NoError(t, err)
		}

		// Set up order: header1 -> header2 -> header3
		err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header1ID,
			Prev: nil,
			Next: header2ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header2ID,
			Prev: header1ID.Bytes(),
			Next: header3ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header3ID,
			Prev: header2ID.Bytes(),
			Next: nil,
		})
		require.NoError(t, err)

		// Test ordered retrieval
		orderedHeaders, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedHeaders, 3)

		assert.Equal(t, header1ID, orderedHeaders[0].ID)
		assert.Equal(t, header2ID, orderedHeaders[1].ID)
		assert.Equal(t, header3ID, orderedHeaders[2].ID)

		// Verify linking
		assert.Nil(t, orderedHeaders[0].Prev)
		require.NotNil(t, orderedHeaders[0].Next)
		assert.Equal(t, header2ID, *orderedHeaders[0].Next)

		require.NotNil(t, orderedHeaders[1].Prev)
		assert.Equal(t, header1ID, *orderedHeaders[1].Prev)
		require.NotNil(t, orderedHeaders[1].Next)
		assert.Equal(t, header3ID, *orderedHeaders[1].Next)

		require.NotNil(t, orderedHeaders[2].Prev)
		assert.Equal(t, header2ID, *orderedHeaders[2].Prev)
		assert.Nil(t, orderedHeaders[2].Next)
	})
}

func TestHeaderService_MoveHeaderEdgeCases(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	service := sexampleheader.New(base.Queries)

	exampleID := setupTestExampleForService(t, ctx, base.Queries)

	t.Run("MoveToBeginning", func(t *testing.T) {
		// Create two headers: header1 -> header2
		header1ID := idwrap.NewNow()
		header2ID := idwrap.NewNow()

		headers := []mexampleheader.Header{
			{
				ID:          header1ID,
				ExampleID:   exampleID,
				HeaderKey:   "first",
				Enable:      true,
				Description: "First",
				Value:       "value1",
			},
			{
				ID:          header2ID,
				ExampleID:   exampleID,
				HeaderKey:   "second",
				Enable:      true,
				Description: "Second",
				Value:       "value2",
			},
		}

		for _, header := range headers {
			err := service.CreateHeader(ctx, header)
			require.NoError(t, err)
		}

		// Set up order: header1 -> header2
		err := base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header1ID,
			Prev: nil,
			Next: header2ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header2ID,
			Prev: header1ID.Bytes(),
			Next: nil,
		})
		require.NoError(t, err)

		// Move header2 before header1: header2 -> header1
		err = service.MoveHeader(ctx, header2ID, header1ID, "before")
		require.NoError(t, err)

		// Verify order
		orderedHeaders, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedHeaders, 2)

		assert.Equal(t, header2ID, orderedHeaders[0].ID)
		assert.Equal(t, header1ID, orderedHeaders[1].ID)
	})

	t.Run("MoveToEnd", func(t *testing.T) {
		// Clean up
		existingHeaders, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		for _, h := range existingHeaders {
			err := service.DeleteHeader(ctx, h.ID)
			require.NoError(t, err)
		}

		// Create two headers: header1 -> header2
		header1ID := idwrap.NewNow()
		header2ID := idwrap.NewNow()

		headers := []mexampleheader.Header{
			{
				ID:          header1ID,
				ExampleID:   exampleID,
				HeaderKey:   "first",
				Enable:      true,
				Description: "First",
				Value:       "value1",
			},
			{
				ID:          header2ID,
				ExampleID:   exampleID,
				HeaderKey:   "second",
				Enable:      true,
				Description: "Second",
				Value:       "value2",
			},
		}

		for _, header := range headers {
			err := service.CreateHeader(ctx, header)
			require.NoError(t, err)
		}

		// Set up order: header1 -> header2
		err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header1ID,
			Prev: nil,
			Next: header2ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
			ID:   header2ID,
			Prev: header1ID.Bytes(),
			Next: nil,
		})
		require.NoError(t, err)

		// Move header1 after header2: header2 -> header1
		err = service.MoveHeader(ctx, header1ID, header2ID, "after")
		require.NoError(t, err)

		// Verify order
		orderedHeaders, err := service.GetHeadersByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedHeaders, 2)

		assert.Equal(t, header2ID, orderedHeaders[0].ID)
		assert.Equal(t, header1ID, orderedHeaders[1].ID)
	})
}