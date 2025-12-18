package delta

import (
	"testing"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
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

		require.Equal(t, baseID, resolved.ID, "Expected ID to match")
		require.Equal(t, "Base Request", resolved.Name, "Expected Name 'Base Request'")
		require.Equal(t, "GET", resolved.Method, "Expected Method 'GET'")
		require.False(t, resolved.IsDelta, "Expected IsDelta to be false")
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

		require.Equal(t, "POST", resolved.Method, "Expected Method 'POST'")
		require.Equal(t, "Base Request", resolved.Name, "Expected Name 'Base Request'")
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

		require.Equal(t, "Updated Name", resolved.Name, "Expected Name 'Updated Name'")
		require.Equal(t, "PUT", resolved.Method, "Expected Method 'PUT'")
		require.Equal(t, "http://updated.com", resolved.Url, "Expected Url 'http://updated.com'")
		require.Equal(t, "Updated Desc", resolved.Description, "Expected Description 'Updated Desc'")
		require.Equal(t, mhttp.HttpBodyKindFormData, resolved.BodyKind, "Expected BodyKind")
		// Verify cleanup
		require.Nil(t, resolved.DeltaName, "Expected DeltaName to be nil in resolved object")
	})
}

func TestCollectionResolution_StrictIDMatching(t *testing.T) {
	// Using Headers as the representative collection
	baseID1 := idwrap.NewNow()
	baseID2 := idwrap.NewNow()

	baseHeaders := []mhttp.HTTPHeader{
		{
			ID:      baseID1,
			Key:     "Content-Type",
			Value:   "application/json",
			Enabled: true,
		},
		{
			ID:      baseID2,
			Key:     "Authorization",
			Value:   "Bearer token",
			Enabled: true,
		},
	}

	t.Run("MatchAndOverride", func(t *testing.T) {
		// Delta targets baseID1
		deltaHeaders := []mhttp.HTTPHeader{
			{
				ID:                 idwrap.NewNow(),
				ParentHttpHeaderID: &baseID1,
				DeltaValue:         ptrStr("application/xml"),
			},
		}

		input := ResolveHTTPInput{
			BaseHeaders:  baseHeaders,
			DeltaHeaders: deltaHeaders,
		}

		output := ResolveHTTP(input)
		resolved := output.ResolvedHeaders

		require.Len(t, resolved, 2, "Expected 2 headers")

		// First item should be modified
		require.Equal(t, baseID1, resolved[0].ID, "Expected first item ID")
		require.Equal(t, "application/xml", resolved[0].Value, "Expected updated value 'application/xml'")
		require.Equal(t, "Content-Type", resolved[0].Key, "Expected key 'Content-Type'")

		// Second item should be untouched
		require.Equal(t, baseID2, resolved[1].ID, "Expected second item ID")
		require.Equal(t, "Bearer token", resolved[1].Value, "Expected original value 'Bearer token'")
	})

	t.Run("NoMatch_Ignored", func(t *testing.T) {
		// Delta targets a non-existent ID
		randomID := idwrap.NewNow()
		deltaHeaders := []mhttp.HTTPHeader{
			{
				ID:                 idwrap.NewNow(),
				ParentHttpHeaderID: &randomID, // Does not match any base item
				DeltaValue:         ptrStr("Should Not Exist"),
			},
		}

		input := ResolveHTTPInput{
			BaseHeaders:  baseHeaders,
			DeltaHeaders: deltaHeaders,
		}

		output := ResolveHTTP(input)
		resolved := output.ResolvedHeaders

		require.Len(t, resolved, 2, "Expected 2 headers")
		// Base items should remain unchanged
		require.Equal(t, "application/json", resolved[0].Value, "Base item 1 modified unexpectedly")
		require.Equal(t, "Bearer token", resolved[1].Value, "Base item 2 modified unexpectedly")
	})
}

func TestCollectionResolution_Additions(t *testing.T) {
	baseHeaders := []mhttp.HTTPHeader{
		{
			ID:    idwrap.NewNow(),
			Key:   "Base",
			Value: "Val",
		},
	}

	t.Run("AddSpecificItem", func(t *testing.T) {
		newID := idwrap.NewNow()
		deltaHeaders := []mhttp.HTTPHeader{
			{
				ID:                 newID,
				ParentHttpHeaderID: nil, // nil ParentID means addition
				Key:                "New-Header",
				Value:              "New-Value",
			},
		}

		input := ResolveHTTPInput{
			BaseHeaders:  baseHeaders,
			DeltaHeaders: deltaHeaders,
		}

		output := ResolveHTTP(input)
		resolved := output.ResolvedHeaders

		require.Len(t, resolved, 2, "Expected 2 headers")

		// Check the added item (it should be appended)
		added := resolved[1]
		require.Equal(t, newID, added.ID, "Expected added item ID")
		require.Equal(t, "New-Header", added.Key, "Expected added key 'New-Header'")
		require.False(t, added.IsDelta, "Expected IsDelta to be cleared on added item")
	})
}

func TestAssertOrdering(t *testing.T) {
	// A -> B -> C (ordered by Order field)
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()
	idC := idwrap.NewNow()

	baseAsserts := []mhttp.HTTPAssert{
		{ID: idB, Value: "B", DisplayOrder: 2.0},
		{ID: idA, Value: "A", DisplayOrder: 1.0},
		{ID: idC, Value: "C", DisplayOrder: 3.0},
	}

	t.Run("PreserveOrder", func(t *testing.T) {
		input := ResolveHTTPInput{
			BaseAsserts: baseAsserts,
		}
		output := ResolveHTTP(input)
		resolved := output.ResolvedAsserts

		require.Len(t, resolved, 3, "Expected 3 asserts")
		require.Equal(t, idA, resolved[0].ID, "Expected first item A")
		require.Equal(t, idB, resolved[1].ID, "Expected second item B")
		require.Equal(t, idC, resolved[2].ID, "Expected third item C")
	})

	t.Run("OverrideMaintainsOrder", func(t *testing.T) {
		// Override B with B'
		deltaAsserts := []mhttp.HTTPAssert{
			{
				ID:                 idwrap.NewNow(),
				ParentHttpAssertID: &idB,
				DeltaValue:         ptrStr("Updated B"),
			},
		}

		input := ResolveHTTPInput{
			BaseAsserts:  baseAsserts,
			DeltaAsserts: deltaAsserts,
		}
		output := ResolveHTTP(input)
		resolved := output.ResolvedAsserts

		require.Len(t, resolved, 3, "Expected 3 asserts")
		// Order should still be A -> B -> C
		require.Equal(t, idB, resolved[1].ID, "Expected second item B")
		require.Equal(t, "Updated B", resolved[1].Value, "Expected updated value 'Updated B'")
	})

	t.Run("AdditionsAppended", func(t *testing.T) {
		idD := idwrap.NewNow()
		deltaAsserts := []mhttp.HTTPAssert{
			{
				ID:                 idD,
				ParentHttpAssertID: nil,
				Value:              "D",
			},
		}

		input := ResolveHTTPInput{
			BaseAsserts:  baseAsserts,
			DeltaAsserts: deltaAsserts,
		}
		output := ResolveHTTP(input)
		resolved := output.ResolvedAsserts

		require.Len(t, resolved, 4, "Expected 4 asserts")
		// A -> B -> C -> D
		require.Equal(t, idD, resolved[3].ID, "Expected fourth item D")
	})
}
