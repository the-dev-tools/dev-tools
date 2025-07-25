package sexamplequery_test

import (
	"context"
	"testing"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestExampleForQueryService(t *testing.T, ctx context.Context, queries *gen.Queries) idwrap.IDWrap {
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

func TestQueryService_MoveQuery(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	service := sexamplequery.New(base.Queries)

	exampleID := setupTestExampleForQueryService(t, ctx, base.Queries)

	// Create three queries: query1 -> query2 -> query3
	query1ID := idwrap.NewNow()
	query2ID := idwrap.NewNow()
	query3ID := idwrap.NewNow()

	queries := []mexamplequery.Query{
		{
			ID:          query1ID,
			ExampleID:   exampleID,
			QueryKey:    "query1",
			Enable:      true,
			Description: "First query",
			Value:       "value1",
		},
		{
			ID:          query2ID,
			ExampleID:   exampleID,
			QueryKey:    "query2",
			Enable:      true,
			Description: "Second query",
			Value:       "value2",
		},
		{
			ID:          query3ID,
			ExampleID:   exampleID,
			QueryKey:    "query3",
			Enable:      true,
			Description: "Third query",
			Value:       "value3",
		},
	}

	// Create queries
	for _, query := range queries {
		err := service.CreateExampleQuery(ctx, query)
		require.NoError(t, err)
	}

	// Set up initial order: query1 -> query2 -> query3
	err := base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   query1ID,
		Prev: nil,
		Next: query2ID.Bytes(),
	})
	require.NoError(t, err)

	err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   query2ID,
		Prev: query1ID.Bytes(),
		Next: query3ID.Bytes(),
	})
	require.NoError(t, err)

	err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   query3ID,
		Prev: query2ID.Bytes(),
		Next: nil,
	})
	require.NoError(t, err)

	t.Run("MoveQuery_Before", func(t *testing.T) {
		// Move query3 before query1: query3 -> query1 -> query2
		err := service.MoveQuery(ctx, query3ID, query1ID, "before")
		require.NoError(t, err)

		// Verify the new order
		orderedQueries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedQueries, 3)

		assert.Equal(t, query3ID, orderedQueries[0].ID)
		assert.Equal(t, query1ID, orderedQueries[1].ID)
		assert.Equal(t, query2ID, orderedQueries[2].ID)
	})

	t.Run("MoveQuery_After", func(t *testing.T) {
		// Reset to original order first
		err := base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query1ID,
			Prev: nil,
			Next: query2ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query2ID,
			Prev: query1ID.Bytes(),
			Next: query3ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query3ID,
			Prev: query2ID.Bytes(),
			Next: nil,
		})
		require.NoError(t, err)

		// Move query1 after query3: query2 -> query3 -> query1
		err = service.MoveQuery(ctx, query1ID, query3ID, "after")
		require.NoError(t, err)

		// Verify the new order
		orderedQueries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedQueries, 3)

		assert.Equal(t, query2ID, orderedQueries[0].ID)
		assert.Equal(t, query3ID, orderedQueries[1].ID)
		assert.Equal(t, query1ID, orderedQueries[2].ID)
	})
}

func TestQueryService_GetQueriesByExampleIDOrdered(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	service := sexamplequery.New(base.Queries)

	exampleID := setupTestExampleForQueryService(t, ctx, base.Queries)

	t.Run("EmptyList", func(t *testing.T) {
		queries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		assert.Len(t, queries, 0)
	})

	t.Run("SingleQuery", func(t *testing.T) {
		queryID := idwrap.NewNow()
		query := mexamplequery.Query{
			ID:          queryID,
			ExampleID:   exampleID,
			QueryKey:    "single-query",
			Enable:      true,
			Description: "Single query",
			Value:       "single-value",
		}

		err := service.CreateExampleQuery(ctx, query)
		require.NoError(t, err)

		queries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, queries, 1)
		assert.Equal(t, queryID, queries[0].ID)
		assert.Nil(t, queries[0].Prev)
		assert.Nil(t, queries[0].Next)
	})

	t.Run("MultipleQueries", func(t *testing.T) {
		// Clean up existing queries
		existingQueries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		for _, q := range existingQueries {
			err := service.DeleteExampleQuery(ctx, q.ID)
			require.NoError(t, err)
		}

		// Create queries in specific order
		query1ID := idwrap.NewNow()
		query2ID := idwrap.NewNow()
		query3ID := idwrap.NewNow()

		queries := []mexamplequery.Query{
			{
				ID:          query1ID,
				ExampleID:   exampleID,
				QueryKey:    "first",
				Enable:      true,
				Description: "First",
				Value:       "value1",
			},
			{
				ID:          query2ID,
				ExampleID:   exampleID,
				QueryKey:    "second",
				Enable:      true,
				Description: "Second",
				Value:       "value2",
			},
			{
				ID:          query3ID,
				ExampleID:   exampleID,
				QueryKey:    "third",
				Enable:      true,
				Description: "Third",
				Value:       "value3",
			},
		}

		// Create queries
		for _, query := range queries {
			err := service.CreateExampleQuery(ctx, query)
			require.NoError(t, err)
		}

		// Set up order: query1 -> query2 -> query3
		err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query1ID,
			Prev: nil,
			Next: query2ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query2ID,
			Prev: query1ID.Bytes(),
			Next: query3ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query3ID,
			Prev: query2ID.Bytes(),
			Next: nil,
		})
		require.NoError(t, err)

		// Test ordered retrieval
		orderedQueries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedQueries, 3)

		assert.Equal(t, query1ID, orderedQueries[0].ID)
		assert.Equal(t, query2ID, orderedQueries[1].ID)
		assert.Equal(t, query3ID, orderedQueries[2].ID)

		// Verify linking
		assert.Nil(t, orderedQueries[0].Prev)
		require.NotNil(t, orderedQueries[0].Next)
		assert.Equal(t, query2ID, *orderedQueries[0].Next)

		require.NotNil(t, orderedQueries[1].Prev)
		assert.Equal(t, query1ID, *orderedQueries[1].Prev)
		require.NotNil(t, orderedQueries[1].Next)
		assert.Equal(t, query3ID, *orderedQueries[1].Next)

		require.NotNil(t, orderedQueries[2].Prev)
		assert.Equal(t, query2ID, *orderedQueries[2].Prev)
		assert.Nil(t, orderedQueries[2].Next)
	})
}

func TestQueryService_MoveQueryEdgeCases(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	service := sexamplequery.New(base.Queries)

	exampleID := setupTestExampleForQueryService(t, ctx, base.Queries)

	t.Run("MoveToBeginning", func(t *testing.T) {
		// Create two queries: query1 -> query2
		query1ID := idwrap.NewNow()
		query2ID := idwrap.NewNow()

		queries := []mexamplequery.Query{
			{
				ID:          query1ID,
				ExampleID:   exampleID,
				QueryKey:    "first",
				Enable:      true,
				Description: "First",
				Value:       "value1",
			},
			{
				ID:          query2ID,
				ExampleID:   exampleID,
				QueryKey:    "second",
				Enable:      true,
				Description: "Second",
				Value:       "value2",
			},
		}

		for _, query := range queries {
			err := service.CreateExampleQuery(ctx, query)
			require.NoError(t, err)
		}

		// Set up order: query1 -> query2
		err := base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query1ID,
			Prev: nil,
			Next: query2ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query2ID,
			Prev: query1ID.Bytes(),
			Next: nil,
		})
		require.NoError(t, err)

		// Move query2 before query1: query2 -> query1
		err = service.MoveQuery(ctx, query2ID, query1ID, "before")
		require.NoError(t, err)

		// Verify order
		orderedQueries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedQueries, 2)

		assert.Equal(t, query2ID, orderedQueries[0].ID)
		assert.Equal(t, query1ID, orderedQueries[1].ID)
	})

	t.Run("MoveToEnd", func(t *testing.T) {
		// Clean up
		existingQueries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		for _, q := range existingQueries {
			err := service.DeleteExampleQuery(ctx, q.ID)
			require.NoError(t, err)
		}

		// Create two queries: query1 -> query2
		query1ID := idwrap.NewNow()
		query2ID := idwrap.NewNow()

		queries := []mexamplequery.Query{
			{
				ID:          query1ID,
				ExampleID:   exampleID,
				QueryKey:    "first",
				Enable:      true,
				Description: "First",
				Value:       "value1",
			},
			{
				ID:          query2ID,
				ExampleID:   exampleID,
				QueryKey:    "second",
				Enable:      true,
				Description: "Second",
				Value:       "value2",
			},
		}

		for _, query := range queries {
			err := service.CreateExampleQuery(ctx, query)
			require.NoError(t, err)
		}

		// Set up order: query1 -> query2
		err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query1ID,
			Prev: nil,
			Next: query2ID.Bytes(),
		})
		require.NoError(t, err)

		err = base.Queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   query2ID,
			Prev: query1ID.Bytes(),
			Next: nil,
		})
		require.NoError(t, err)

		// Move query1 after query2: query2 -> query1
		err = service.MoveQuery(ctx, query1ID, query2ID, "after")
		require.NoError(t, err)

		// Verify order
		orderedQueries, err := service.GetQueriesByExampleIDOrdered(ctx, exampleID)
		require.NoError(t, err)
		require.Len(t, orderedQueries, 2)

		assert.Equal(t, query2ID, orderedQueries[0].ID)
		assert.Equal(t, query1ID, orderedQueries[1].ID)
	})
}