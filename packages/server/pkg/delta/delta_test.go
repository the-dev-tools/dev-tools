package delta

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

// pointers for scalar types
func ptrStr(s string) *string { return &s }
func ptrBool(b bool) *bool    { return &b }

// ptrBodyKind helps creating pointers for HttpBodyKind
func ptrBodyKind(k mhttp.HttpBodyKind) *mhttp.HttpBodyKind { return &k }

func TestResolveHTTPScalar(t *testing.T) {
	baseID := idwrap.NewNow()
	deltaID := idwrap.NewNow()

	base := mhttp.HTTP{
		ID:          baseID,
		Name:        "Base Request",
		Method:      "GET",
		Url:         "http://example.com",
		Description: "Base Description",
		BodyKind:    mhttp.HttpBodyKindNone,
		IsDelta:     false,
	}

	t.Run("NoOverrides", func(t *testing.T) {
		delta := mhttp.HTTP{
			ID:      deltaID,
			IsDelta: true,
			// All Delta* fields are nil
		}

		input := ResolveHTTPInput{
			Base:  base,
			Delta: delta,
		}

		output := ResolveHTTP(input)
		resolved := output.Resolved

		if resolved.ID != baseID {
			t.Errorf("Expected ID %v, got %v", baseID, resolved.ID)
		}
		if resolved.Name != "Base Request" {
			t.Errorf("Expected Name 'Base Request', got '%s'", resolved.Name)
		}
		if resolved.Method != "GET" {
			t.Errorf("Expected Method 'GET', got '%s'", resolved.Method)
		}
		if resolved.IsDelta {
			t.Error("Expected IsDelta to be false")
		}
	})

	t.Run("PartialOverrides", func(t *testing.T) {
		delta := mhttp.HTTP{
			ID:          deltaID,
			IsDelta:     true,
			DeltaMethod: ptrStr("POST"),
		}

		input := ResolveHTTPInput{
			Base:  base,
			Delta: delta,
		}

		output := ResolveHTTP(input)
		resolved := output.Resolved

		if resolved.Method != "POST" {
			t.Errorf("Expected Method 'POST', got '%s'", resolved.Method)
		}
		if resolved.Name != "Base Request" {
			t.Errorf("Expected Name 'Base Request', got '%s'", resolved.Name)
		}
	})

	t.Run("FullOverrides", func(t *testing.T) {
		delta := mhttp.HTTP{
			ID:               deltaID,
			IsDelta:          true,
			DeltaName:        ptrStr("Updated Name"),
			DeltaMethod:      ptrStr("PUT"),
			DeltaUrl:         ptrStr("http://updated.com"),
			DeltaDescription: ptrStr("Updated Desc"),
			DeltaBodyKind:    ptrBodyKind(mhttp.HttpBodyKindFormData),
		}

		input := ResolveHTTPInput{
			Base:  base,
			Delta: delta,
		}

		output := ResolveHTTP(input)
		resolved := output.Resolved

		if resolved.Name != "Updated Name" {
			t.Errorf("Expected Name 'Updated Name', got '%s'", resolved.Name)
		}
		if resolved.Method != "PUT" {
			t.Errorf("Expected Method 'PUT', got '%s'", resolved.Method)
		}
		if resolved.Url != "http://updated.com" {
			t.Errorf("Expected Url 'http://updated.com', got '%s'", resolved.Url)
		}
		if resolved.Description != "Updated Desc" {
			t.Errorf("Expected Description 'Updated Desc', got '%s'", resolved.Description)
		}
		if resolved.BodyKind != mhttp.HttpBodyKindFormData {
			t.Errorf("Expected BodyKind %v, got %v", mhttp.HttpBodyKindFormData, resolved.BodyKind)
		}
		// Verify cleanup
		if resolved.DeltaName != nil {
			t.Error("Expected DeltaName to be nil in resolved object")
		}
	})
}

func TestCollectionResolution_StrictIDMatching(t *testing.T) {
	// Using Headers as the representative collection
	baseID1 := idwrap.NewNow()
	baseID2 := idwrap.NewNow()

	baseHeaders := []mhttp.HTTPHeader{
		{
			ID:          baseID1,
			Key:   "Content-Type",
			Value: "application/json",
			Enabled:     true,
		},
		{
			ID:          baseID2,
			Key:   "Authorization",
			Value: "Bearer token",
			Enabled:     true,
		},
	}

	t.Run("MatchAndOverride", func(t *testing.T) {
		// Delta targets baseID1
		deltaHeaders := []mhttp.HTTPHeader{
			{
				ID:               idwrap.NewNow(),
				ParentHttpHeaderID:   &baseID1,
				DeltaValue: ptrStr("application/xml"),
			},
		}

		input := ResolveHTTPInput{
			BaseHeaders:  baseHeaders,
			DeltaHeaders: deltaHeaders,
		}

		output := ResolveHTTP(input)
		resolved := output.ResolvedHeaders

		if len(resolved) != 2 {
			t.Fatalf("Expected 2 headers, got %d", len(resolved))
		}

		// First item should be modified
		if resolved[0].ID != baseID1 {
			t.Errorf("Expected first item ID %v, got %v", baseID1, resolved[0].ID)
		}
		if resolved[0].Value != "application/xml" {
			t.Errorf("Expected updated value 'application/xml', got '%s'", resolved[0].Value)
		}
		if resolved[0].Key != "Content-Type" {
			t.Errorf("Expected key 'Content-Type', got '%s'", resolved[0].Key)
		}

		// Second item should be untouched
		if resolved[1].ID != baseID2 {
			t.Errorf("Expected second item ID %v, got %v", baseID2, resolved[1].ID)
		}
		if resolved[1].Value != "Bearer token" {
			t.Errorf("Expected original value 'Bearer token', got '%s'", resolved[1].Value)
		}
	})

	t.Run("NoMatch_Ignored", func(t *testing.T) {
		// Delta targets a non-existent ID
		randomID := idwrap.NewNow()
		deltaHeaders := []mhttp.HTTPHeader{
			{
				ID:               idwrap.NewNow(),
				ParentHttpHeaderID:   &randomID, // Does not match any base item
				DeltaValue: ptrStr("Should Not Exist"),
			},
		}

		input := ResolveHTTPInput{
			BaseHeaders:  baseHeaders,
			DeltaHeaders: deltaHeaders,
		}

		output := ResolveHTTP(input)
		resolved := output.ResolvedHeaders

		if len(resolved) != 2 {
			t.Fatalf("Expected 2 headers, got %d", len(resolved))
		}
		// Base items should remain unchanged
		if resolved[0].Value != "application/json" {
			t.Error("Base item 1 modified unexpectedly")
		}
		if resolved[1].Value != "Bearer token" {
			t.Error("Base item 2 modified unexpectedly")
		}
	})
}

func TestCollectionResolution_Additions(t *testing.T) {
	baseHeaders := []mhttp.HTTPHeader{
		{
			ID:          idwrap.NewNow(),
			Key:   "Base",
			Value: "Val",
		},
	}

	t.Run("AddSpecificItem", func(t *testing.T) {
		newID := idwrap.NewNow()
		deltaHeaders := []mhttp.HTTPHeader{
			{
				ID:             newID,
				ParentHttpHeaderID: nil, // nil ParentID means addition
				Key:      "New-Header",
				Value:    "New-Value",
			},
		}

		input := ResolveHTTPInput{
			BaseHeaders:  baseHeaders,
			DeltaHeaders: deltaHeaders,
		}

		output := ResolveHTTP(input)
		resolved := output.ResolvedHeaders

		if len(resolved) != 2 {
			t.Fatalf("Expected 2 headers, got %d", len(resolved))
		}

		// Check the added item (it should be appended)
		added := resolved[1]
		if added.ID != newID {
			t.Errorf("Expected added item ID %v, got %v", newID, added.ID)
		}
		if added.Key != "New-Header" {
			t.Errorf("Expected added key 'New-Header', got '%s'", added.Key)
		}
		if added.IsDelta {
			t.Error("Expected IsDelta to be cleared on added item")
		}
	})
}

func TestAssertOrdering(t *testing.T) {
	// A -> B -> C (ordered by Order field)
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()
	idC := idwrap.NewNow()

	baseAsserts := []mhttp.HTTPAssert{
		{ID: idB, Key: "B", Order: 2.0},
		{ID: idA, Key: "A", Order: 1.0},
		{ID: idC, Key: "C", Order: 3.0},
	}

	t.Run("PreserveOrder", func(t *testing.T) {
		input := ResolveHTTPInput{
			BaseAsserts: baseAsserts,
		}
		output := ResolveHTTP(input)
		resolved := output.ResolvedAsserts

		if len(resolved) != 3 {
			t.Fatalf("Expected 3 asserts, got %d", len(resolved))
		}
		if resolved[0].ID != idA {
			t.Errorf("Expected first item A, got %s", resolved[0].Key)
		}
		if resolved[1].ID != idB {
			t.Errorf("Expected second item B, got %s", resolved[1].Key)
		}
		if resolved[2].ID != idC {
			t.Errorf("Expected third item C, got %s", resolved[2].Key)
		}
	})

	t.Run("OverrideMaintainsOrder", func(t *testing.T) {
		// Override B with B'
		deltaAsserts := []mhttp.HTTPAssert{
			{
				ID:               idwrap.NewNow(),
				ParentHttpAssertID:   &idB,
				DeltaValue: ptrStr("Updated B"),
			},
		}

		input := ResolveHTTPInput{
			BaseAsserts:  baseAsserts,
			DeltaAsserts: deltaAsserts,
		}
		output := ResolveHTTP(input)
		resolved := output.ResolvedAsserts

		if len(resolved) != 3 {
			t.Fatalf("Expected 3 asserts, got %d", len(resolved))
		}
		// Order should still be A -> B -> C
		if resolved[1].ID != idB {
			t.Errorf("Expected second item B, got %s", resolved[1].Key)
		}
		if resolved[1].Value != "Updated B" {
			t.Errorf("Expected updated value 'Updated B', got '%s'", resolved[1].Value)
		}
	})

	t.Run("AdditionsAppended", func(t *testing.T) {
		idD := idwrap.NewNow()
		deltaAsserts := []mhttp.HTTPAssert{
			{
				ID:             idD,
				ParentHttpAssertID: nil,
				Key:      "D",
			},
		}

		input := ResolveHTTPInput{
			BaseAsserts:  baseAsserts,
			DeltaAsserts: deltaAsserts,
		}
		output := ResolveHTTP(input)
		resolved := output.ResolvedAsserts

		if len(resolved) != 4 {
			t.Fatalf("Expected 4 asserts, got %d", len(resolved))
		}
		// A -> B -> C -> D
		if resolved[3].ID != idD {
			t.Errorf("Expected fourth item D, got %s", resolved[3].Key)
		}
	})
}
