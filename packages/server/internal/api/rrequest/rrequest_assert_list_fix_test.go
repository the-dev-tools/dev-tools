package rrequest_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/go-cmp/cmp"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/service/sassert"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sexampleheader"
	"the-dev-tools/server/pkg/service/sexamplequery"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/service/sitemapiexample"
	"the-dev-tools/server/pkg/service/suser"
	"the-dev-tools/server/pkg/testutil"
	conditionv1 "the-dev-tools/spec/dist/buf/go/condition/v1"
	requestv1 "the-dev-tools/spec/dist/buf/go/collection/item/request/v1"
)

// TestAssertListOrderedMethod tests that AssertList RPC uses GetAssertsOrdered and returns assertions in correct order
func TestAssertListOrderedMethod(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries
	db := base.DB

	// Initialize all services
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	// Create workspace, user and collection
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create authenticated context
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// Create endpoint
	endpointID := idwrap.NewNow()
	endpoint := &mitemapi.ItemApi{
		ID:           endpointID,
		CollectionID: collectionID,
		Name:         "test-endpoint",
		Method:       "GET",
		Url:          "/api/test",
		Hidden:       false,
	}
	err := ias.CreateItemApi(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}

	// Create example
	exampleID := idwrap.NewNow()
	example := &mitemapiexample.ItemApiExample{
		ID:           exampleID,
		ItemApiID:    endpointID,
		CollectionID: collectionID,
		Name:         "Test Example",
	}
	err = iaes.CreateApiExample(ctx, example)
	if err != nil {
		t.Fatal(err)
	}

	// Create RPC handler
	rpc := rrequest.New(db, cs, us, ias, iaes, ehs, eqs, as)

	t.Run("AssertListReturnsOrderedAssertions", func(t *testing.T) {
		// Test data: create assertions in specific order
		assertionData := []struct {
			dotPath       string
			operator      string
			expectedValue string
		}{
			{"response.status", "==", "200"},
			{"response.body.success", "==", "true"},
			{"response.body.data", "!=", "null"},
			{"response.headers.content-type", "contains", "json"},
		}

		// Create assertions via RPC (this should maintain order using linked list)
		var createdAssertionIDs [][]byte
		for _, data := range assertionData {
			condition := &conditionv1.Condition{
				Comparison: &conditionv1.Comparison{
					Expression: fmt.Sprintf("%s %s %s", data.dotPath, data.operator, data.expectedValue),
				},
			}

			resp, err := rpc.AssertCreate(ctx, connect.NewRequest(&requestv1.AssertCreateRequest{
				ExampleId: exampleID.Bytes(),
				Condition: condition,
			}))
			if err != nil {
				t.Fatalf("Failed to create assertion %s: %v", data.dotPath, err)
			}
			createdAssertionIDs = append(createdAssertionIDs, resp.Msg.AssertId)
		}

		// Verify assertions were created
		if len(createdAssertionIDs) != len(assertionData) {
			t.Fatalf("Expected %d assertions, created %d", len(assertionData), len(createdAssertionIDs))
		}

		// Test AssertList RPC - this is the method we fixed
		listResp, err := rpc.AssertList(ctx, connect.NewRequest(&requestv1.AssertListRequest{
			ExampleId: exampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("AssertList RPC failed: %v", err)
		}

		// Verify we get the correct number of assertions
		if len(listResp.Msg.Items) != len(assertionData) {
			t.Fatalf("AssertList returned %d items, expected %d", len(listResp.Msg.Items), len(assertionData))
		}

		// Verify assertions are returned in the correct order (creation order)
		var returnedDotPaths []string
		for _, item := range listResp.Msg.Items {
			// Extract dot path from the condition expression
			expression := item.Condition.Comparison.Expression
			// For this test, we assume the expression format is "dotpath operator value"
			parts := strings.Fields(expression)
			if len(parts) >= 1 {
				returnedDotPaths = append(returnedDotPaths, parts[0])
			}
		}

		expectedDotPaths := []string{
			"response.status",
			"response.body.success",
			"response.body.data",
			"response.headers.content-type",
		}

		if !cmp.Equal(returnedDotPaths, expectedDotPaths) {
			t.Errorf("AssertList order mismatch:\nGot:      %v\nExpected: %v", returnedDotPaths, expectedDotPaths)
		}

		t.Log("✅ AssertList RPC returns assertions in correct order")

		// Cross-verify with direct service call to ensure consistency
		directAsserts, err := as.GetAssertsOrdered(ctx, exampleID)
		if err != nil {
			t.Fatalf("Direct GetAssertsOrdered failed: %v", err)
		}

		var directDotPaths []string
		for _, assert := range directAsserts {
			// Extract dot path from the condition expression
			expression := assert.Condition.Comparisons.Expression
			parts := strings.Fields(expression)
			if len(parts) >= 1 {
				directDotPaths = append(directDotPaths, parts[0])
			}
		}

		// Both methods should return the same order
		if !cmp.Equal(returnedDotPaths, directDotPaths) {
			t.Errorf("RPC and direct service results differ:\nRPC:    %v\nDirect: %v", returnedDotPaths, directDotPaths)
		}

		t.Log("✅ AssertList RPC and direct service return consistent results")
	})

	t.Run("AssertListEmptyExample", func(t *testing.T) {
		// Test AssertList with example that has no assertions
		emptyExampleID := idwrap.NewNow()
		emptyExample := &mitemapiexample.ItemApiExample{
			ID:           emptyExampleID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         "Empty Example",
		}
		err := iaes.CreateApiExample(ctx, emptyExample)
		if err != nil {
			t.Fatal(err)
		}

		// Call AssertList on empty example
		listResp, err := rpc.AssertList(ctx, connect.NewRequest(&requestv1.AssertListRequest{
			ExampleId: emptyExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("AssertList RPC failed on empty example: %v", err)
		}

		// Should return empty list
		if len(listResp.Msg.Items) != 0 {
			t.Errorf("Expected empty list for example with no assertions, got %d items", len(listResp.Msg.Items))
		}

		t.Log("✅ AssertList RPC handles empty examples correctly")
	})

	t.Run("AssertListSingleAssertion", func(t *testing.T) {
		// Test AssertList with single assertion
		singleExampleID := idwrap.NewNow()
		singleExample := &mitemapiexample.ItemApiExample{
			ID:           singleExampleID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         "Single Assertion Example",
		}
		err := iaes.CreateApiExample(ctx, singleExample)
		if err != nil {
			t.Fatal(err)
		}

		// Create single assertion
		condition := &conditionv1.Condition{
			Comparison: &conditionv1.Comparison{
				Expression: "response.status == 201",
			},
		}

		createResp, err := rpc.AssertCreate(ctx, connect.NewRequest(&requestv1.AssertCreateRequest{
			ExampleId: singleExampleID.Bytes(),
			Condition: condition,
		}))
		if err != nil {
			t.Fatalf("Failed to create single assertion: %v", err)
		}

		// Test AssertList
		listResp, err := rpc.AssertList(ctx, connect.NewRequest(&requestv1.AssertListRequest{
			ExampleId: singleExampleID.Bytes(),
		}))
		if err != nil {
			t.Fatalf("AssertList RPC failed on single assertion: %v", err)
		}

		// Verify single assertion is returned
		if len(listResp.Msg.Items) != 1 {
			t.Fatalf("Expected 1 assertion, got %d", len(listResp.Msg.Items))
		}

		item := listResp.Msg.Items[0]
		expression := item.Condition.Comparison.Expression
		if expression != "response.status == 201" {
			t.Errorf("Expected expression 'response.status == 201', got '%s'", expression)
		}

		// Verify assertion ID matches
		createdIDWrapped, err := idwrap.NewFromBytes(createResp.Msg.AssertId)
		if err != nil {
			t.Fatal(err)
		}
		returnedIDWrapped, err := idwrap.NewFromBytes(item.AssertId)
		if err != nil {
			t.Fatal(err)
		}

		if createdIDWrapped.Compare(returnedIDWrapped) != 0 {
			t.Errorf("Assertion ID mismatch: created %s, returned %s", createdIDWrapped.String(), returnedIDWrapped.String())
		}

		t.Log("✅ AssertList RPC handles single assertion correctly")
	})
}