package sqlc_test

import (
	"context"
	"testing"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/testutil"
)

func TestGetQueriesByExampleIDOrdered(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	exampleID := setupTestExample(t, ctx, queries)

	// Create queries in a specific order: query1 -> query2 -> query3
	query1ID := idwrap.NewNow()
	query2ID := idwrap.NewNow()
	query3ID := idwrap.NewNow()

	// Create queries
	for _, queryData := range []struct {
		id    idwrap.IDWrap
		key   string
		value string
		desc  string
	}{
		{query1ID, "query1", "value1", "First query"},
		{query2ID, "query2", "value2", "Second query"},
		{query3ID, "query3", "value3", "Third query"},
	} {
		err := queries.CreateQuery(ctx, gen.CreateQueryParams{
			ID:          queryData.id,
			ExampleID:   exampleID,
			QueryKey:    queryData.key,
			Enable:      true,
			Description: queryData.desc,
			Value:       queryData.value,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set up the linked list: query1 -> query2 -> query3
	err := queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   query1ID,
		Prev: nil,
		Next: query2ID.Bytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   query2ID,
		Prev: query1ID.Bytes(),
		Next: query3ID.Bytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   query3ID,
		Prev: query2ID.Bytes(),
		Next: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test GetQueriesByExampleIDOrdered
	queryResults, err := queries.GetQueriesByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(queryResults) != 3 {
		t.Fatalf("Expected 3 queries, got %d", len(queryResults))
	}

	// Verify order: query1, query2, query3
	testutil.Assert(t, query1ID, queryResults[0].ID)
	testutil.Assert(t, query2ID, queryResults[1].ID)
	testutil.Assert(t, query3ID, queryResults[2].ID)
}

func TestQueryMoveOperations(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	exampleID := setupTestExample(t, ctx, queries)

	// Create a query for testing moves
	queryID := idwrap.NewNow()
	err := queries.CreateQuery(ctx, gen.CreateQueryParams{
		ID:          queryID,
		ExampleID:   exampleID,
		QueryKey:    "test-query",
		Enable:      true,
		Description: "Test query",
		Value:       "test-value",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test UpdateQueryPrev
	newPrevID := idwrap.NewNow()
	err = queries.UpdateQueryPrev(ctx, gen.UpdateQueryPrevParams{
		ID:   queryID,
		Prev: newPrevID.Bytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify UpdateQueryPrev worked
	query, err := queries.GetQuery(ctx, queryID)
	if err != nil {
		t.Fatal(err)
	}
	prevID, err := idwrap.NewFromBytes(query.Prev)
	if err != nil || prevID != newPrevID {
		t.Errorf("Expected prev to be %s, got %v", newPrevID, prevID)
	}

	// Test UpdateQueryNext
	newNextID := idwrap.NewNow()
	err = queries.UpdateQueryNext(ctx, gen.UpdateQueryNextParams{
		ID:   queryID,
		Next: newNextID.Bytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify UpdateQueryNext worked
	query, err = queries.GetQuery(ctx, queryID)
	if err != nil {
		t.Fatal(err)
	}
	nextID, err := idwrap.NewFromBytes(query.Next)
	if err != nil || nextID != newNextID {
		t.Errorf("Expected next to be %s, got %v", newNextID, nextID)
	}

	// Test UpdateQueryOrder (updating both prev and next)
	anotherPrevID := idwrap.NewNow()
	anotherNextID := idwrap.NewNow()
	err = queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   queryID,
		Prev: anotherPrevID.Bytes(),
		Next: anotherNextID.Bytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify UpdateQueryOrder worked
	query, err = queries.GetQuery(ctx, queryID)
	if err != nil {
		t.Fatal(err)
	}
	prevID, err = idwrap.NewFromBytes(query.Prev)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Assert(t, anotherPrevID, prevID)

	nextID, err = idwrap.NewFromBytes(query.Next)
	if err != nil {
		t.Fatal(err)
	}
	testutil.Assert(t, anotherNextID, nextID)
}

func TestGetQueryByPrevNext(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	exampleID := setupTestExample(t, ctx, queries)

	// Create queries
	query1ID := idwrap.NewNow()
	query2ID := idwrap.NewNow()

	for _, queryData := range []struct {
		id    idwrap.IDWrap
		key   string
		value string
		desc  string
	}{
		{query1ID, "query1", "value1", "First query"},
		{query2ID, "query2", "value2", "Second query"},
	} {
		err := queries.CreateQuery(ctx, gen.CreateQueryParams{
			ID:          queryData.id,
			ExampleID:   exampleID,
			QueryKey:    queryData.key,
			Enable:      true,
			Description: queryData.desc,
			Value:       queryData.value,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set up linked list: query1 -> query2
	err := queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   query1ID,
		Prev: nil,
		Next: query2ID.Bytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
		ID:   query2ID,
		Prev: query1ID.Bytes(),
		Next: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test GetQueryByPrevNext - find query2 by its position
	query, err := queries.GetQueryByPrevNext(ctx, gen.GetQueryByPrevNextParams{
		ExampleID: exampleID,
		PrevValue: query1ID.Bytes(),
		NextValue: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	testutil.Assert(t, query2ID, query.ID)

	// Test GetQueryByPrevNext - find query1 by its position
	query, err = queries.GetQueryByPrevNext(ctx, gen.GetQueryByPrevNextParams{
		ExampleID: exampleID,
		PrevValue: nil,
		NextValue: query2ID.Bytes(),
	})
	if err != nil {
		t.Fatal(err)
	}

	testutil.Assert(t, query1ID, query.ID)
}

func TestQueryOrderingEdgeCases(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	exampleID := setupTestExample(t, ctx, queries)

	// Test with no queries
	queryResults, err := queries.GetQueriesByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(queryResults) != 0 {
		t.Fatalf("Expected 0 queries for empty example, got %d", len(queryResults))
	}

	// Test with single query
	singleQueryID := idwrap.NewNow()
	err = queries.CreateQuery(ctx, gen.CreateQueryParams{
		ID:          singleQueryID,
		ExampleID:   exampleID,
		QueryKey:    "single-query",
		Enable:      true,
		Description: "Single query",
		Value:       "single-value",
	})
	if err != nil {
		t.Fatal(err)
	}

	queryResults, err = queries.GetQueriesByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(queryResults) != 1 {
		t.Fatalf("Expected 1 query, got %d", len(queryResults))
	}
	testutil.Assert(t, singleQueryID, queryResults[0].ID)
	if len(queryResults[0].Prev) != 0 {
		t.Error("Single query should have prev = NULL")
	}
	if len(queryResults[0].Next) != 0 {
		t.Error("Single query should have next = NULL")
	}
}