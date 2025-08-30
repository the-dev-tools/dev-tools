package rrequest_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"the-dev-tools/server/internal/api/middleware/mwauth"
	"the-dev-tools/server/internal/api/rrequest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/logger/mocklogger"
	"the-dev-tools/server/pkg/model/massert"
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

// TestAssertDeltaFiltering focuses on the specific delta filtering logic bug
func TestAssertDeltaFiltering(t *testing.T) {
	ctx := context.Background()
	base := testutil.CreateBaseDB(ctx, t)
	queries := base.Queries

	// Initialize all services
	ias := sitemapi.New(queries)
	iaes := sitemapiexample.New(queries)
	mockLogger := mocklogger.NewMockLogger()
	cs := scollection.New(queries, mockLogger)
	us := suser.New(queries)
	ehs := sexampleheader.New(queries)
	eqs := sexamplequery.New(queries)
	as := sassert.New(queries)

	// Create RequestRPC
	rpc := rrequest.New(base.DB, cs, us, ias, iaes, ehs, eqs, as)

	// Create workspace, user and collection
	workspaceID := idwrap.NewNow()
	workspaceUserID := idwrap.NewNow()
	collectionID := idwrap.NewNow()
	userID := idwrap.NewNow()

	baseServices := base.GetBaseServices()
	baseServices.CreateTempCollection(t, ctx, workspaceID, workspaceUserID, userID, collectionID)

	// Create authenticated context
	ctx = mwauth.CreateAuthedContext(ctx, userID)

	// Test Scenario 1: Regular example (no VersionParentID)
	t.Run("RegularExample", func(t *testing.T) {
		// Create endpoint
		endpointID := idwrap.NewNow()
		endpoint := &mitemapi.ItemApi{
			ID:           endpointID,
			CollectionID: collectionID,
			Name:         "Regular Endpoint",
			Method:       "GET",
			Url:          "/api/regular",
			Hidden:       false,
		}
		err := ias.CreateItemApi(ctx, endpoint)
		if err != nil {
			t.Fatal(err)
		}

		// Create regular example (no VersionParentID)
		exampleID := idwrap.NewNow()
		example := &mitemapiexample.ItemApiExample{
			ID:           exampleID,
			ItemApiID:    endpointID,
			CollectionID: collectionID,
			Name:         "Regular Example",
			// No VersionParentID - this is a regular example
		}
		err = iaes.CreateApiExample(ctx, example)
		if err != nil {
			t.Fatal(err)
		}

		// Create assertion on regular example
		createResp, err := rpc.AssertCreate(ctx, &connect.Request[requestv1.AssertCreateRequest]{
			Msg: &requestv1.AssertCreateRequest{
				ExampleId: exampleID.Bytes(),
				Condition: &conditionv1.Condition{
					Comparison: &conditionv1.Comparison{
						Expression: "response.status == 200",
					},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Verify assertion is returned by list (should work for regular examples)
		listResp, err := rpc.AssertList(ctx, &connect.Request[requestv1.AssertListRequest]{
			Msg: &requestv1.AssertListRequest{
				ExampleId: exampleID.Bytes(),
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(listResp.Msg.Items) != 1 {
			t.Fatalf("Expected 1 assertion for regular example, got %d", len(listResp.Msg.Items))
		}

		// Verify the assertion ID matches
		if string(listResp.Msg.Items[0].AssertId) != string(createResp.Msg.AssertId) {
			t.Error("Assertion ID mismatch for regular example")
		}

		// Verify direct service call to understand the delta type logic
		assertID := idwrap.NewFromBytesMust(createResp.Msg.AssertId)
		assert, err := as.GetAssert(ctx, assertID)
		if err != nil {
			t.Fatal(err)
		}
		
		// This should be AssertSourceOrigin for regular examples
		deltaType := assert.DetermineDeltaType(false) // exampleHasVersionParent = false
		t.Logf("Regular example assertion delta type: %d (expecting %d for AssertSourceOrigin)", 
			deltaType, massert.AssertSourceOrigin)
		
		if deltaType != massert.AssertSourceOrigin {
			t.Errorf("Expected AssertSourceOrigin (%d) for regular example, got %d", 
				massert.AssertSourceOrigin, deltaType)
		}
	})

	// Test Scenario 2: Delta example (with VersionParentID)
	t.Run("DeltaExample", func(t *testing.T) {
		// Create base endpoint for the original
		baseEndpointID := idwrap.NewNow()
		baseEndpoint := &mitemapi.ItemApi{
			ID:           baseEndpointID,
			CollectionID: collectionID,
			Name:         "Base Endpoint",
			Method:       "GET",
			Url:          "/api/base",
			Hidden:       false,
		}
		err := ias.CreateItemApi(ctx, baseEndpoint)
		if err != nil {
			t.Fatal(err)
		}

		// Create base example
		baseExampleID := idwrap.NewNow()
		baseExample := &mitemapiexample.ItemApiExample{
			ID:           baseExampleID,
			ItemApiID:    baseEndpointID,
			CollectionID: collectionID,
			Name:         "Base Example",
		}
		err = iaes.CreateApiExample(ctx, baseExample)
		if err != nil {
			t.Fatal(err)
		}

		// Create delta endpoint
		deltaEndpointID := idwrap.NewNow()
		deltaEndpoint := &mitemapi.ItemApi{
			ID:              deltaEndpointID,
			CollectionID:    collectionID,
			Name:            "Delta Endpoint",
			Method:          "POST",
			Url:             "/api/delta",
			Hidden:          true,
			VersionParentID: &baseEndpointID,
		}
		err = ias.CreateItemApi(ctx, deltaEndpoint)
		if err != nil {
			t.Fatal(err)
		}

		// Create delta example (with VersionParentID)
		deltaExampleID := idwrap.NewNow()
		deltaExample := &mitemapiexample.ItemApiExample{
			ID:              deltaExampleID,
			ItemApiID:       deltaEndpointID,
			CollectionID:    collectionID,
			Name:            "Delta Example",
			VersionParentID: &baseExampleID, // This makes it a delta example
		}
		err = iaes.CreateApiExample(ctx, deltaExample)
		if err != nil {
			t.Fatal(err)
		}

		// Create assertion on delta example
		createResp, err := rpc.AssertCreate(ctx, &connect.Request[requestv1.AssertCreateRequest]{
			Msg: &requestv1.AssertCreateRequest{
				ExampleId: deltaExampleID.Bytes(),
				Condition: &conditionv1.Condition{
					Comparison: &conditionv1.Comparison{
						Expression: "response.status == 201",
					},
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Here's the potential bug: AssertList might filter out this assertion
		listResp, err := rpc.AssertList(ctx, &connect.Request[requestv1.AssertListRequest]{
			Msg: &requestv1.AssertListRequest{
				ExampleId: deltaExampleID.Bytes(),
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// DEBUG: Check what we got
		t.Logf("Delta example assertion list returned %d items", len(listResp.Msg.Items))
		
		// This might be the bug - if the delta filtering is incorrect, this will return 0
		if len(listResp.Msg.Items) == 0 {
			t.Log("BUG REPRODUCED: Delta example assertion not returned by list!")
			
			// Let's investigate why by checking the assertion directly
			assertID := idwrap.NewFromBytesMust(createResp.Msg.AssertId)
			assert, err := as.GetAssert(ctx, assertID)
			if err != nil {
				t.Fatal(err)
			}
			
			// Check what delta type it determines
			deltaType := assert.DetermineDeltaType(true) // exampleHasVersionParent = true
			t.Logf("Delta example assertion delta type: %d", deltaType)
			t.Logf("AssertSourceOrigin = %d, AssertSourceMixed = %d, AssertSourceDelta = %d",
				massert.AssertSourceOrigin, massert.AssertSourceMixed, massert.AssertSourceDelta)
				
			// The issue is likely that assertions created with no DeltaParentID on a delta example
			// get classified as AssertSourceDelta (type 3) but AssertList only returns AssertSourceOrigin (type 1)
			if deltaType == massert.AssertSourceDelta {
				t.Log("CONFIRMED BUG: Assertion on delta example classified as AssertSourceDelta, but AssertList only returns AssertSourceOrigin")
			}
		}

		if len(listResp.Msg.Items) != 1 {
			t.Errorf("Expected 1 assertion for delta example, got %d", len(listResp.Msg.Items))
			return
		}

		// If we get here, verify the assertion ID matches
		if string(listResp.Msg.Items[0].AssertId) != string(createResp.Msg.AssertId) {
			t.Error("Assertion ID mismatch for delta example")
		}
	})
}

// TestAssertDeltaTypeLogic specifically tests the delta type determination logic
func TestAssertDeltaTypeLogic(t *testing.T) {

	// Test cases for DetermineDeltaType logic
	testCases := []struct {
		name                    string
		hasDeltaParentID        bool
		exampleHasVersionParent bool
		expectedDeltaType       massert.AssertSource
	}{
		{
			name:                    "RegularExampleNoDeltaParent",
			hasDeltaParentID:        false,
			exampleHasVersionParent: false,
			expectedDeltaType:       massert.AssertSourceOrigin,
		},
		{
			name:                    "RegularExampleWithDeltaParent", 
			hasDeltaParentID:        true,
			exampleHasVersionParent: false,
			expectedDeltaType:       massert.AssertSourceMixed,
		},
		{
			name:                    "DeltaExampleNoDeltaParent",
			hasDeltaParentID:        false,
			exampleHasVersionParent: true,
			expectedDeltaType:       massert.AssertSourceDelta, // This might be the issue!
		},
		{
			name:                    "DeltaExampleWithDeltaParent",
			hasDeltaParentID:        true,
			exampleHasVersionParent: true,
			expectedDeltaType:       massert.AssertSourceMixed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test assertion
			assert := massert.Assert{
				ID:        idwrap.NewNow(),
				ExampleID: idwrap.NewNow(),
			}

			if tc.hasDeltaParentID {
				parentID := idwrap.NewNow()
				assert.DeltaParentID = &parentID
			}

			deltaType := assert.DetermineDeltaType(tc.exampleHasVersionParent)
			
			t.Logf("Test case: %s", tc.name)
			t.Logf("  hasDeltaParentID: %t", tc.hasDeltaParentID)
			t.Logf("  exampleHasVersionParent: %t", tc.exampleHasVersionParent)
			t.Logf("  expectedDeltaType: %d", tc.expectedDeltaType)
			t.Logf("  actualDeltaType: %d", deltaType)

			if deltaType != tc.expectedDeltaType {
				t.Errorf("Expected delta type %d, got %d", tc.expectedDeltaType, deltaType)
			}

			// The key insight: AssertList only returns AssertSourceOrigin (type 1)
			// So any assertion that gets classified as AssertSourceDelta (type 3) will be filtered out!
			if deltaType == massert.AssertSourceDelta {
				t.Logf("  WARNING: This assertion would be filtered out by AssertList!")
			}
		})
	}
}