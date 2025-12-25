package shttp

import (
	"context"
	"testing"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
)

func TestBodyFormWriter_UpdateDelta_DeltaOrder(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	// Create parent HTTP request
	parentHTTPID := idwrap.NewNow()
	httpService := New(queries, nil)
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          parentHTTPID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Parent Request",
	})
	require.NoError(t, err)

	// Create parent BodyForm entry
	parentBodyFormID := idwrap.NewNow()
	bodyFormService := NewHttpBodyFormService(queries)
	err = bodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
		ID:      parentBodyFormID,
		HttpID:  parentHTTPID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	})
	require.NoError(t, err)

	// Create delta HTTP request
	deltaHTTPID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:           deltaHTTPID,
		WorkspaceID:  idwrap.NewNow(),
		Name:         "Delta Request",
		IsDelta:      true,
		ParentHttpID: &parentHTTPID,
	})
	require.NoError(t, err)

	// Create delta BodyForm entry with initial deltaOrder
	deltaBodyFormID := idwrap.NewNow()
	initialDeltaOrder := float32(1.0)
	err = bodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
		ID:                   deltaBodyFormID,
		HttpID:               deltaHTTPID,
		IsDelta:              true,
		ParentHttpBodyFormID: &parentBodyFormID,
		DeltaKey:             strPtr("delta_field"),
		DeltaValue:           strPtr("delta_value"),
		DeltaEnabled:         boolPtr(true),
		DeltaDisplayOrder:    &initialDeltaOrder,
	})
	require.NoError(t, err)

	// Retrieve and verify initial deltaOrder was persisted
	retrieved, err := bodyFormService.GetByID(ctx, deltaBodyFormID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, float32(1.0), *retrieved.DeltaDisplayOrder)

	// Update deltaOrder to a new value while preserving other delta fields
	newDeltaOrder := float32(2.5)
	writer := NewBodyFormWriter(db)
	err = writer.UpdateDelta(ctx, deltaBodyFormID,
		retrieved.DeltaKey,
		retrieved.DeltaValue,
		retrieved.DeltaEnabled,
		retrieved.DeltaDescription,
		&newDeltaOrder)
	require.NoError(t, err)

	// Retrieve and verify the updated deltaOrder was persisted
	updated, err := bodyFormService.GetByID(ctx, deltaBodyFormID)
	require.NoError(t, err)
	require.NotNil(t, updated.DeltaDisplayOrder)
	require.Equal(t, float32(2.5), *updated.DeltaDisplayOrder)

	// Verify other delta fields were not affected
	require.Equal(t, "delta_field", *updated.DeltaKey)
	require.Equal(t, "delta_value", *updated.DeltaValue)
	require.True(t, *updated.DeltaEnabled)
}

func TestBodyFormWriter_UpdateDelta_DeltaOrderNil(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	// Create parent HTTP request
	parentHTTPID := idwrap.NewNow()
	httpService := New(queries, nil)
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          parentHTTPID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Parent Request",
	})
	require.NoError(t, err)

	// Create parent BodyForm entry
	parentBodyFormID := idwrap.NewNow()
	bodyFormService := NewHttpBodyFormService(queries)
	err = bodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
		ID:      parentBodyFormID,
		HttpID:  parentHTTPID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	})
	require.NoError(t, err)

	// Create delta HTTP request
	deltaHTTPID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:           deltaHTTPID,
		WorkspaceID:  idwrap.NewNow(),
		Name:         "Delta Request",
		IsDelta:      true,
		ParentHttpID: &parentHTTPID,
	})
	require.NoError(t, err)

	// Create delta BodyForm entry with deltaOrder set
	deltaBodyFormID := idwrap.NewNow()
	initialDeltaOrder := float32(3.0)
	err = bodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
		ID:                   deltaBodyFormID,
		HttpID:               deltaHTTPID,
		IsDelta:              true,
		ParentHttpBodyFormID: &parentBodyFormID,
		DeltaKey:             strPtr("delta_field"),
		DeltaValue:           strPtr("delta_value"),
		DeltaEnabled:         boolPtr(true),
		DeltaDisplayOrder:    &initialDeltaOrder,
	})
	require.NoError(t, err)

	// Verify initial deltaOrder exists
	retrieved, err := bodyFormService.GetByID(ctx, deltaBodyFormID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, float32(3.0), *retrieved.DeltaDisplayOrder)

	// Update deltaOrder to nil (should unset the field) while preserving other delta fields
	writer := NewBodyFormWriter(db)
	err = writer.UpdateDelta(ctx, deltaBodyFormID,
		retrieved.DeltaKey,
		retrieved.DeltaValue,
		retrieved.DeltaEnabled,
		retrieved.DeltaDescription,
		nil)
	require.NoError(t, err)

	// Retrieve and verify deltaOrder was unset
	updated, err := bodyFormService.GetByID(ctx, deltaBodyFormID)
	require.NoError(t, err)
	// DeltaDisplayOrder should be nil after unsetting
	require.Nil(t, updated.DeltaDisplayOrder)

	// Verify other delta fields were not affected
	require.Equal(t, "delta_field", *updated.DeltaKey)
	require.Equal(t, "delta_value", *updated.DeltaValue)
	require.True(t, *updated.DeltaEnabled)

	// Set deltaOrder again to verify it can be set after being unset
	newDeltaOrder := float32(5.0)
	err = writer.UpdateDelta(ctx, deltaBodyFormID,
		updated.DeltaKey,
		updated.DeltaValue,
		updated.DeltaEnabled,
		updated.DeltaDescription,
		&newDeltaOrder)
	require.NoError(t, err)

	reupdated, err := bodyFormService.GetByID(ctx, deltaBodyFormID)
	require.NoError(t, err)
	require.NotNil(t, reupdated.DeltaDisplayOrder)
	require.Equal(t, float32(5.0), *reupdated.DeltaDisplayOrder)
}

func TestBodyFormWriter_UpdateDelta_DeltaOrderMultiple(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries, err := gen.Prepare(ctx, db)
	require.NoError(t, err)

	// Create parent HTTP request
	parentHTTPID := idwrap.NewNow()
	httpService := New(queries, nil)
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          parentHTTPID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Parent Request",
	})
	require.NoError(t, err)

	// Create parent BodyForm entry
	parentBodyFormID := idwrap.NewNow()
	bodyFormService := NewHttpBodyFormService(queries)
	err = bodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
		ID:      parentBodyFormID,
		HttpID:  parentHTTPID,
		Key:     "field1",
		Value:   "value1",
		Enabled: true,
	})
	require.NoError(t, err)

	// Create delta HTTP request
	deltaHTTPID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:           deltaHTTPID,
		WorkspaceID:  idwrap.NewNow(),
		Name:         "Delta Request",
		IsDelta:      true,
		ParentHttpID: &parentHTTPID,
	})
	require.NoError(t, err)

	// Create delta BodyForm entry
	deltaBodyFormID := idwrap.NewNow()
	err = bodyFormService.Create(ctx, &mhttp.HTTPBodyForm{
		ID:                   deltaBodyFormID,
		HttpID:               deltaHTTPID,
		IsDelta:              true,
		ParentHttpBodyFormID: &parentBodyFormID,
		DeltaKey:             strPtr("delta_field"),
		DeltaValue:           strPtr("delta_value"),
		DeltaEnabled:         boolPtr(true),
	})
	require.NoError(t, err)

	writer := NewBodyFormWriter(db)

	// Get initial state to preserve delta fields
	initial, err := bodyFormService.GetByID(ctx, deltaBodyFormID)
	require.NoError(t, err)

	// Test multiple sequential updates to deltaOrder
	testOrders := []float32{1.0, 10.5, 0.5, 100.0, 2.25}
	for _, expectedOrder := range testOrders {
		err = writer.UpdateDelta(ctx, deltaBodyFormID,
			initial.DeltaKey,
			initial.DeltaValue,
			initial.DeltaEnabled,
			initial.DeltaDescription,
			&expectedOrder)
		require.NoError(t, err)

		retrieved, err := bodyFormService.GetByID(ctx, deltaBodyFormID)
		require.NoError(t, err)
		require.NotNil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should not be nil for order %.2f", expectedOrder)
		require.Equal(t, expectedOrder, *retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should be %.2f", expectedOrder)
	}

	// Verify other delta fields remained stable
	final, err := bodyFormService.GetByID(ctx, deltaBodyFormID)
	require.NoError(t, err)
	require.Equal(t, "delta_field", *final.DeltaKey)
	require.Equal(t, "delta_value", *final.DeltaValue)
	require.True(t, *final.DeltaEnabled)
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
