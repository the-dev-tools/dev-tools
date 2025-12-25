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

func TestBodyUrlEncodedWriter_UpdateDelta_DeltaOrder(t *testing.T) {
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

	// Create parent BodyUrlEncoded entry
	parentBodyUrlEncodedID := idwrap.NewNow()
	bodyUrlEncodedService := NewHttpBodyUrlEncodedService(queries)
	err = bodyUrlEncodedService.Create(ctx, &mhttp.HTTPBodyUrlencoded{
		ID:      parentBodyUrlEncodedID,
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

	// Create delta BodyUrlEncoded entry with initial deltaOrder
	deltaBodyUrlEncodedID := idwrap.NewNow()
	initialDeltaOrder := float32(1.0)
	err = bodyUrlEncodedService.Create(ctx, &mhttp.HTTPBodyUrlencoded{
		ID:                         deltaBodyUrlEncodedID,
		HttpID:                     deltaHTTPID,
		IsDelta:                    true,
		ParentHttpBodyUrlEncodedID: &parentBodyUrlEncodedID,
		DeltaKey:                   strPtr("delta_field"),
		DeltaValue:                 strPtr("delta_value"),
		DeltaEnabled:               boolPtr(true),
		DeltaDisplayOrder:          &initialDeltaOrder,
	})
	require.NoError(t, err)

	// Retrieve and verify initial deltaOrder was persisted
	retrieved, err := bodyUrlEncodedService.GetByID(ctx, deltaBodyUrlEncodedID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, float32(1.0), *retrieved.DeltaDisplayOrder)

	// Update deltaOrder to a new value while preserving other delta fields
	newDeltaOrder := float32(2.5)
	writer := NewBodyUrlEncodedWriter(db)
	err = writer.UpdateDelta(ctx, deltaBodyUrlEncodedID,
		retrieved.DeltaKey,
		retrieved.DeltaValue,
		retrieved.DeltaEnabled,
		retrieved.DeltaDescription,
		&newDeltaOrder)
	require.NoError(t, err)

	// Retrieve and verify the updated deltaOrder was persisted
	updated, err := bodyUrlEncodedService.GetByID(ctx, deltaBodyUrlEncodedID)
	require.NoError(t, err)
	require.NotNil(t, updated.DeltaDisplayOrder)
	require.Equal(t, float32(2.5), *updated.DeltaDisplayOrder)

	// Verify other delta fields were not affected
	require.Equal(t, "delta_field", *updated.DeltaKey)
	require.Equal(t, "delta_value", *updated.DeltaValue)
	require.True(t, *updated.DeltaEnabled)
}

func TestBodyUrlEncodedWriter_UpdateDelta_DeltaOrderNil(t *testing.T) {
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

	// Create parent BodyUrlEncoded entry
	parentBodyUrlEncodedID := idwrap.NewNow()
	bodyUrlEncodedService := NewHttpBodyUrlEncodedService(queries)
	err = bodyUrlEncodedService.Create(ctx, &mhttp.HTTPBodyUrlencoded{
		ID:      parentBodyUrlEncodedID,
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

	// Create delta BodyUrlEncoded entry with deltaOrder set
	deltaBodyUrlEncodedID := idwrap.NewNow()
	initialDeltaOrder := float32(3.0)
	err = bodyUrlEncodedService.Create(ctx, &mhttp.HTTPBodyUrlencoded{
		ID:                         deltaBodyUrlEncodedID,
		HttpID:                     deltaHTTPID,
		IsDelta:                    true,
		ParentHttpBodyUrlEncodedID: &parentBodyUrlEncodedID,
		DeltaKey:                   strPtr("delta_field"),
		DeltaValue:                 strPtr("delta_value"),
		DeltaEnabled:               boolPtr(true),
		DeltaDisplayOrder:          &initialDeltaOrder,
	})
	require.NoError(t, err)

	// Verify initial deltaOrder exists
	retrieved, err := bodyUrlEncodedService.GetByID(ctx, deltaBodyUrlEncodedID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, float32(3.0), *retrieved.DeltaDisplayOrder)

	// Update deltaOrder to nil (should unset the field) while preserving other delta fields
	writer := NewBodyUrlEncodedWriter(db)
	err = writer.UpdateDelta(ctx, deltaBodyUrlEncodedID,
		retrieved.DeltaKey,
		retrieved.DeltaValue,
		retrieved.DeltaEnabled,
		retrieved.DeltaDescription,
		nil)
	require.NoError(t, err)

	// Retrieve and verify deltaOrder was unset
	updated, err := bodyUrlEncodedService.GetByID(ctx, deltaBodyUrlEncodedID)
	require.NoError(t, err)
	// DeltaDisplayOrder should be nil after unsetting
	require.Nil(t, updated.DeltaDisplayOrder)

	// Verify other delta fields were not affected
	require.Equal(t, "delta_field", *updated.DeltaKey)
	require.Equal(t, "delta_value", *updated.DeltaValue)
	require.True(t, *updated.DeltaEnabled)

	// Set deltaOrder again to verify it can be set after being unset
	newDeltaOrder := float32(5.0)
	err = writer.UpdateDelta(ctx, deltaBodyUrlEncodedID,
		updated.DeltaKey,
		updated.DeltaValue,
		updated.DeltaEnabled,
		updated.DeltaDescription,
		&newDeltaOrder)
	require.NoError(t, err)

	reupdated, err := bodyUrlEncodedService.GetByID(ctx, deltaBodyUrlEncodedID)
	require.NoError(t, err)
	require.NotNil(t, reupdated.DeltaDisplayOrder)
	require.Equal(t, float32(5.0), *reupdated.DeltaDisplayOrder)
}

func TestBodyUrlEncodedWriter_UpdateDelta_DeltaOrderMultiple(t *testing.T) {
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

	// Create parent BodyUrlEncoded entry
	parentBodyUrlEncodedID := idwrap.NewNow()
	bodyUrlEncodedService := NewHttpBodyUrlEncodedService(queries)
	err = bodyUrlEncodedService.Create(ctx, &mhttp.HTTPBodyUrlencoded{
		ID:      parentBodyUrlEncodedID,
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

	// Create delta BodyUrlEncoded entry
	deltaBodyUrlEncodedID := idwrap.NewNow()
	err = bodyUrlEncodedService.Create(ctx, &mhttp.HTTPBodyUrlencoded{
		ID:                         deltaBodyUrlEncodedID,
		HttpID:                     deltaHTTPID,
		IsDelta:                    true,
		ParentHttpBodyUrlEncodedID: &parentBodyUrlEncodedID,
		DeltaKey:                   strPtr("delta_field"),
		DeltaValue:                 strPtr("delta_value"),
		DeltaEnabled:               boolPtr(true),
	})
	require.NoError(t, err)

	writer := NewBodyUrlEncodedWriter(db)

	// Get initial state to preserve delta fields
	initial, err := bodyUrlEncodedService.GetByID(ctx, deltaBodyUrlEncodedID)
	require.NoError(t, err)

	// Test multiple sequential updates to deltaOrder
	testOrders := []float32{1.0, 10.5, 0.5, 100.0, 2.25}
	for _, expectedOrder := range testOrders {
		err = writer.UpdateDelta(ctx, deltaBodyUrlEncodedID,
			initial.DeltaKey,
			initial.DeltaValue,
			initial.DeltaEnabled,
			initial.DeltaDescription,
			&expectedOrder)
		require.NoError(t, err)

		retrieved, err := bodyUrlEncodedService.GetByID(ctx, deltaBodyUrlEncodedID)
		require.NoError(t, err)
		require.NotNil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should not be nil for order %.2f", expectedOrder)
		require.Equal(t, expectedOrder, *retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should be %.2f", expectedOrder)
	}

	// Verify other delta fields remained stable
	final, err := bodyUrlEncodedService.GetByID(ctx, deltaBodyUrlEncodedID)
	require.NoError(t, err)
	require.Equal(t, "delta_field", *final.DeltaKey)
	require.Equal(t, "delta_value", *final.DeltaValue)
	require.True(t, *final.DeltaEnabled)
}
