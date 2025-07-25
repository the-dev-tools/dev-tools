package sqlc_test

import (
	"context"
	"testing"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/testutil"
)

// setupTestExample creates the necessary test data and returns the example ID
func setupTestExample(t *testing.T, ctx context.Context, queries *gen.Queries) idwrap.IDWrap {
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

func TestGetHeadersByExampleIDOrdered(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	exampleID := setupTestExample(t, ctx, queries)

	// Create headers in a specific order: header1 -> header2 -> header3
	header1ID := idwrap.NewNow()
	header2ID := idwrap.NewNow()
	header3ID := idwrap.NewNow()

	// Create headers
	for _, headerData := range []struct {
		id    idwrap.IDWrap
		key   string
		value string
		desc  string
	}{
		{header1ID, "header1", "value1", "First header"},
		{header2ID, "header2", "value2", "Second header"},
		{header3ID, "header3", "value3", "Third header"},
	} {
		err := queries.CreateHeader(ctx, gen.CreateHeaderParams{
			ID:          headerData.id,
			ExampleID:   exampleID,
			HeaderKey:   headerData.key,
			Enable:      true,
			Description: headerData.desc,
			Value:       headerData.value,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Set up the linked list: header1 -> header2 -> header3
	err := queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
		ID:   header1ID,
		Prev: nil,
		Next: &header2ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
		ID:   header2ID,
		Prev: &header1ID,
		Next: &header3ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
		ID:   header3ID,
		Prev: &header2ID,
		Next: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test GetHeadersByExampleIDOrdered
	headers, err := queries.GetHeadersByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}

	if len(headers) != 3 {
		t.Fatalf("Expected 3 headers, got %d", len(headers))
	}

	// Verify order: header1, header2, header3
	testutil.Assert(t, header1ID, headers[0].ID)
	testutil.Assert(t, header2ID, headers[1].ID)
	testutil.Assert(t, header3ID, headers[2].ID)
}

func TestHeaderMoveOperations(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	exampleID := setupTestExample(t, ctx, queries)

	// Create a header for testing moves
	headerID := idwrap.NewNow()
	err := queries.CreateHeader(ctx, gen.CreateHeaderParams{
		ID:          headerID,
		ExampleID:   exampleID,
		HeaderKey:   "test-header",
		Enable:      true,
		Description: "Test header",
		Value:       "test-value",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test UpdateHeaderPrev
	newPrevID := idwrap.NewNow()
	err = queries.UpdateHeaderPrev(ctx, gen.UpdateHeaderPrevParams{
		ID:   headerID,
		Prev: &newPrevID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify UpdateHeaderPrev worked
	header, err := queries.GetHeader(ctx, headerID)
	if err != nil {
		t.Fatal(err)
	}
	if header.Prev == nil || *header.Prev != newPrevID {
		t.Errorf("Expected prev to be %s, got %v", newPrevID, header.Prev)
	}

	// Test UpdateHeaderNext
	newNextID := idwrap.NewNow()
	err = queries.UpdateHeaderNext(ctx, gen.UpdateHeaderNextParams{
		ID:   headerID,
		Next: &newNextID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify UpdateHeaderNext worked
	header, err = queries.GetHeader(ctx, headerID)
	if err != nil {
		t.Fatal(err)
	}
	if header.Next == nil || *header.Next != newNextID {
		t.Errorf("Expected next to be %s, got %v", newNextID, header.Next)
	}

	// Test UpdateHeaderOrder (updating both prev and next)
	anotherPrevID := idwrap.NewNow()
	anotherNextID := idwrap.NewNow()
	err = queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
		ID:   headerID,
		Prev: &anotherPrevID,
		Next: &anotherNextID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify UpdateHeaderOrder worked
	header, err = queries.GetHeader(ctx, headerID)
	if err != nil {
		t.Fatal(err)
	}
	if header.Prev == nil {
		t.Fatal("Expected prev to be set")
	}
	testutil.Assert(t, anotherPrevID, *header.Prev)
	
	if header.Next == nil {
		t.Fatal("Expected next to be set")
	}
	testutil.Assert(t, anotherNextID, *header.Next)
}

// TestGetHeaderByPrevNext is disabled for now due to complex SQL parameter generation issues
// TODO: Implement this test when sqlc supports complex conditional NULL queries
/*
func TestGetHeaderByPrevNext(t *testing.T) {
	// Test disabled - GetHeaderByPrevNext query not generated by sqlc
}
*/

func TestHeaderOrderingEdgeCases(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	exampleID := setupTestExample(t, ctx, queries)

	// Test with no headers
	headers, err := queries.GetHeadersByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 0 {
		t.Fatalf("Expected 0 headers for empty example, got %d", len(headers))
	}

	// Test with single header
	singleHeaderID := idwrap.NewNow()
	err = queries.CreateHeader(ctx, gen.CreateHeaderParams{
		ID:          singleHeaderID,
		ExampleID:   exampleID,
		HeaderKey:   "single-header",
		Enable:      true,
		Description: "Single header",
		Value:       "single-value",
	})
	if err != nil {
		t.Fatal(err)
	}

	headers, err = queries.GetHeadersByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 1 {
		t.Fatalf("Expected 1 header, got %d", len(headers))
	}
	testutil.Assert(t, singleHeaderID, headers[0].ID)
	if headers[0].Prev != nil {
		t.Error("Single header should have prev = NULL")
	}
	if headers[0].Next != nil {
		t.Error("Single header should have next = NULL")
	}
}