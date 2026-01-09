package shttp

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlitemem"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

// TestSearchParamWriter_UpdateDelta_DeltaOrder verifies that deltaOrder persists when set
func TestSearchParamWriter_UpdateDelta_DeltaOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	writer := NewSearchParamWriter(db)
	reader := NewSearchParamReader(db)
	httpService := New(queries, nil)

	// Setup: Create base HTTP
	workspaceID := idwrap.NewNow()
	baseHTTPID := idwrap.NewNow()
	now := time.Now().Unix()

	baseHTTP := &mhttp.HTTP{
		ID:          baseHTTPID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Url:         "https://api.example.com",
		Method:      "GET",
		IsDelta:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = httpService.Create(ctx, baseHTTP)
	require.NoError(t, err)

	// Create base search param
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:           baseParamID,
		HttpID:       baseHTTPID,
		Key:          "query",
		Value:        "test",
		Enabled:      true,
		Description:  "Base param",
		DisplayOrder: 1.0,
		IsDelta:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = writer.Create(ctx, baseParam)
	require.NoError(t, err)

	// Create delta HTTP
	deltaHTTPID := idwrap.NewNow()
	deltaName := "Delta Request"
	deltaHTTP := &mhttp.HTTP{
		ID:           deltaHTTPID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request Delta",
		ParentHttpID: &baseHTTPID,
		IsDelta:      true,
		DeltaName:    &deltaName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = httpService.Create(ctx, deltaHTTP)
	require.NoError(t, err)

	// Create delta search param
	deltaParamID := idwrap.NewNow()
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHTTPID,
		Key:                     "query",
		Value:                   "test",
		Enabled:                 true,
		Description:             "Delta param",
		DisplayOrder:            1.0,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	err = writer.Create(ctx, deltaParam)
	require.NoError(t, err)

	// Act: Update delta with deltaOrder
	deltaOrder := 2.5
	err = writer.UpdateDelta(ctx, deltaParamID, nil, nil, nil, nil, &deltaOrder)
	require.NoError(t, err)

	// Assert: Verify persistence
	retrieved, err := reader.GetByID(ctx, deltaParamID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should not be nil")
	require.Equal(t, deltaOrder, *retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should match the value set")
}

// TestSearchParamWriter_UpdateDelta_DeltaOrderNil verifies that nil deltaOrder unsets the field
func TestSearchParamWriter_UpdateDelta_DeltaOrderNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	writer := NewSearchParamWriter(db)
	reader := NewSearchParamReader(db)
	httpService := New(queries, nil)

	// Setup: Create base HTTP
	workspaceID := idwrap.NewNow()
	baseHTTPID := idwrap.NewNow()
	now := time.Now().Unix()

	baseHTTP := &mhttp.HTTP{
		ID:          baseHTTPID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Url:         "https://api.example.com",
		Method:      "GET",
		IsDelta:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = httpService.Create(ctx, baseHTTP)
	require.NoError(t, err)

	// Create base search param
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:           baseParamID,
		HttpID:       baseHTTPID,
		Key:          "query",
		Value:        "test",
		Enabled:      true,
		Description:  "Base param",
		DisplayOrder: 1.0,
		IsDelta:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = writer.Create(ctx, baseParam)
	require.NoError(t, err)

	// Create delta HTTP
	deltaHTTPID := idwrap.NewNow()
	deltaName := "Delta Request"
	deltaHTTP := &mhttp.HTTP{
		ID:           deltaHTTPID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request Delta",
		ParentHttpID: &baseHTTPID,
		IsDelta:      true,
		DeltaName:    &deltaName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = httpService.Create(ctx, deltaHTTP)
	require.NoError(t, err)

	// Create delta search param with initial deltaOrder
	deltaParamID := idwrap.NewNow()
	initialDeltaOrder := 3.0
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHTTPID,
		Key:                     "query",
		Value:                   "test",
		Enabled:                 true,
		Description:             "Delta param",
		DisplayOrder:            1.0,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		DeltaDisplayOrder:       &initialDeltaOrder,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	err = writer.Create(ctx, deltaParam)
	require.NoError(t, err)

	// Verify initial state
	retrieved, err := reader.GetByID(ctx, deltaParamID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should be set initially")
	require.Equal(t, initialDeltaOrder, *retrieved.DeltaDisplayOrder)

	// Act: Update delta with nil deltaOrder to unset the field
	err = writer.UpdateDelta(ctx, deltaParamID, nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// Assert: Verify the field is now unset
	retrieved, err = reader.GetByID(ctx, deltaParamID)
	require.NoError(t, err)
	require.Nil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should be nil after update with nil")
}

// TestSearchParamWriter_UpdateDelta_DeltaOrderMultiple verifies that deltaOrder can be changed multiple times
func TestSearchParamWriter_UpdateDelta_DeltaOrderMultiple(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	writer := NewSearchParamWriter(db)
	reader := NewSearchParamReader(db)
	httpService := New(queries, nil)

	// Setup: Create base HTTP
	workspaceID := idwrap.NewNow()
	baseHTTPID := idwrap.NewNow()
	now := time.Now().Unix()

	baseHTTP := &mhttp.HTTP{
		ID:          baseHTTPID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Url:         "https://api.example.com",
		Method:      "GET",
		IsDelta:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = httpService.Create(ctx, baseHTTP)
	require.NoError(t, err)

	// Create base search param
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:           baseParamID,
		HttpID:       baseHTTPID,
		Key:          "query",
		Value:        "test",
		Enabled:      true,
		Description:  "Base param",
		DisplayOrder: 1.0,
		IsDelta:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = writer.Create(ctx, baseParam)
	require.NoError(t, err)

	// Create delta HTTP
	deltaHTTPID := idwrap.NewNow()
	deltaName := "Delta Request"
	deltaHTTP := &mhttp.HTTP{
		ID:           deltaHTTPID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request Delta",
		ParentHttpID: &baseHTTPID,
		IsDelta:      true,
		DeltaName:    &deltaName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = httpService.Create(ctx, deltaHTTP)
	require.NoError(t, err)

	// Create delta search param
	deltaParamID := idwrap.NewNow()
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHTTPID,
		Key:                     "query",
		Value:                   "test",
		Enabled:                 true,
		Description:             "Delta param",
		DisplayOrder:            1.0,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	err = writer.Create(ctx, deltaParam)
	require.NoError(t, err)

	// Act: Update delta order multiple times and verify each change
	testCases := []struct {
		name       string
		deltaOrder float64
	}{
		{"First update", 1.5},
		{"Second update", 10.0},
		{"Third update", 0.5},
		{"Fourth update", 999.99},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Update with new order
			err := writer.UpdateDelta(ctx, deltaParamID, nil, nil, nil, nil, &tc.deltaOrder)
			require.NoError(t, err)

			// Verify the new order persisted
			retrieved, err := reader.GetByID(ctx, deltaParamID)
			require.NoError(t, err)
			require.NotNil(t, retrieved.DeltaDisplayOrder)
			require.Equal(t, tc.deltaOrder, *retrieved.DeltaDisplayOrder)
		})
	}
}

// TestSearchParamWriter_UpdateDelta_DeltaOrderWithOtherFields verifies deltaOrder persists alongside other delta fields
func TestSearchParamWriter_UpdateDelta_DeltaOrderWithOtherFields(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	writer := NewSearchParamWriter(db)
	reader := NewSearchParamReader(db)
	httpService := New(queries, nil)

	// Setup: Create base HTTP
	workspaceID := idwrap.NewNow()
	baseHTTPID := idwrap.NewNow()
	now := time.Now().Unix()

	baseHTTP := &mhttp.HTTP{
		ID:          baseHTTPID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Url:         "https://api.example.com",
		Method:      "GET",
		IsDelta:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = httpService.Create(ctx, baseHTTP)
	require.NoError(t, err)

	// Create base search param
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:           baseParamID,
		HttpID:       baseHTTPID,
		Key:          "query",
		Value:        "base",
		Enabled:      true,
		Description:  "Base param",
		DisplayOrder: 1.0,
		IsDelta:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = writer.Create(ctx, baseParam)
	require.NoError(t, err)

	// Create delta HTTP
	deltaHTTPID := idwrap.NewNow()
	deltaName := "Delta Request"
	deltaHTTP := &mhttp.HTTP{
		ID:           deltaHTTPID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request Delta",
		ParentHttpID: &baseHTTPID,
		IsDelta:      true,
		DeltaName:    &deltaName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = httpService.Create(ctx, deltaHTTP)
	require.NoError(t, err)

	// Create delta search param
	deltaParamID := idwrap.NewNow()
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHTTPID,
		Key:                     "query",
		Value:                   "base",
		Enabled:                 true,
		Description:             "Delta param",
		DisplayOrder:            1.0,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	err = writer.Create(ctx, deltaParam)
	require.NoError(t, err)

	// Act: Update delta with all fields including deltaOrder
	deltaKey := "search"
	deltaValue := "updated"
	deltaEnabled := false
	deltaDescription := "Updated description"
	deltaOrder := 5.5

	err = writer.UpdateDelta(ctx, deltaParamID, &deltaKey, &deltaValue, &deltaEnabled, &deltaDescription, &deltaOrder)
	require.NoError(t, err)

	// Assert: Verify all fields persisted correctly
	retrieved, err := reader.GetByID(ctx, deltaParamID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaKey)
	require.Equal(t, deltaKey, *retrieved.DeltaKey)
	require.NotNil(t, retrieved.DeltaValue)
	require.Equal(t, deltaValue, *retrieved.DeltaValue)
	require.NotNil(t, retrieved.DeltaEnabled)
	require.Equal(t, deltaEnabled, *retrieved.DeltaEnabled)
	require.NotNil(t, retrieved.DeltaDescription)
	require.Equal(t, deltaDescription, *retrieved.DeltaDescription)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, deltaOrder, *retrieved.DeltaDisplayOrder)
}

// TestSearchParamWriter_ResetDelta_ClearsDeltaOrder verifies that ResetDelta clears deltaOrder
func TestSearchParamWriter_ResetDelta_ClearsDeltaOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	writer := NewSearchParamWriter(db)
	reader := NewSearchParamReader(db)
	httpService := New(queries, nil)

	// Setup: Create base HTTP
	workspaceID := idwrap.NewNow()
	baseHTTPID := idwrap.NewNow()
	now := time.Now().Unix()

	baseHTTP := &mhttp.HTTP{
		ID:          baseHTTPID,
		WorkspaceID: workspaceID,
		Name:        "Base Request",
		Url:         "https://api.example.com",
		Method:      "GET",
		IsDelta:     false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	err = httpService.Create(ctx, baseHTTP)
	require.NoError(t, err)

	// Create base search param
	baseParamID := idwrap.NewNow()
	baseParam := &mhttp.HTTPSearchParam{
		ID:           baseParamID,
		HttpID:       baseHTTPID,
		Key:          "query",
		Value:        "test",
		Enabled:      true,
		Description:  "Base param",
		DisplayOrder: 1.0,
		IsDelta:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = writer.Create(ctx, baseParam)
	require.NoError(t, err)

	// Create delta HTTP
	deltaHTTPID := idwrap.NewNow()
	deltaName := "Delta Request"
	deltaHTTP := &mhttp.HTTP{
		ID:           deltaHTTPID,
		WorkspaceID:  workspaceID,
		Name:         "Base Request Delta",
		ParentHttpID: &baseHTTPID,
		IsDelta:      true,
		DeltaName:    &deltaName,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	err = httpService.Create(ctx, deltaHTTP)
	require.NoError(t, err)

	// Create delta search param with deltaOrder set
	deltaParamID := idwrap.NewNow()
	deltaKey := "search"
	deltaValue := "delta"
	deltaEnabled := false
	deltaDescription := "Delta desc"
	deltaOrder := 7.5
	deltaParam := &mhttp.HTTPSearchParam{
		ID:                      deltaParamID,
		HttpID:                  deltaHTTPID,
		Key:                     "query",
		Value:                   "test",
		Enabled:                 true,
		Description:             "Delta param",
		DisplayOrder:            1.0,
		ParentHttpSearchParamID: &baseParamID,
		IsDelta:                 true,
		DeltaKey:                &deltaKey,
		DeltaValue:              &deltaValue,
		DeltaEnabled:            &deltaEnabled,
		DeltaDescription:        &deltaDescription,
		DeltaDisplayOrder:       &deltaOrder,
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	err = writer.Create(ctx, deltaParam)
	require.NoError(t, err)

	// Verify initial state with deltaOrder set
	retrieved, err := reader.GetByID(ctx, deltaParamID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, deltaOrder, *retrieved.DeltaDisplayOrder)

	// Act: Reset delta
	err = writer.ResetDelta(ctx, deltaParamID)
	require.NoError(t, err)

	// Assert: Verify all delta fields are cleared including deltaOrder
	// Note: ResetDelta calls Update which doesn't modify IsDelta or ParentHttpSearchParamID,
	// so we only verify the delta fields that are explicitly cleared via UpdateDelta
	retrieved, err = reader.GetByID(ctx, deltaParamID)
	require.NoError(t, err)
	require.Nil(t, retrieved.DeltaKey)
	require.Nil(t, retrieved.DeltaValue)
	require.Nil(t, retrieved.DeltaEnabled)
	require.Nil(t, retrieved.DeltaDescription)
	require.Nil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should be nil after ResetDelta")
}
