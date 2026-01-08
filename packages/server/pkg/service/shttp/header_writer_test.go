package shttp

import (
	"context"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
)

func TestHeaderWriter_UpdateDelta_DeltaOrder(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)

	writer := NewHeaderWriterFromQueries(db)
	reader := NewHeaderReaderFromQueries(db)

	// Create parent HTTP request
	httpService := New(db, nil)
	parentHttpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          parentHttpID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Parent HTTP",
	})
	require.NoError(t, err)

	// Create parent header
	parentHeaderID := idwrap.NewNow()
	parentHeader := &mhttp.HTTPHeader{
		ID:      parentHeaderID,
		HttpID:  parentHttpID,
		Key:     "Content-Type",
		Value:   "application/json",
		Enabled: true,
	}
	err = writer.Create(ctx, parentHeader)
	require.NoError(t, err)

	// Create delta HTTP request
	deltaHttpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  idwrap.NewNow(),
		Name:         "Delta HTTP",
		IsDelta:      true,
		ParentHttpID: &parentHttpID,
	})
	require.NoError(t, err)

	// Create delta header
	deltaHeaderID := idwrap.NewNow()
	deltaKey := "Authorization"
	deltaValue := "Bearer token123"
	deltaEnabled := true
	deltaHeader := &mhttp.HTTPHeader{
		ID:                 deltaHeaderID,
		HttpID:             deltaHttpID,
		IsDelta:            true,
		ParentHttpHeaderID: &parentHeaderID,
		DeltaKey:           &deltaKey,
		DeltaValue:         &deltaValue,
		DeltaEnabled:       &deltaEnabled,
	}
	err = writer.Create(ctx, deltaHeader)
	require.NoError(t, err)

	// Update delta with deltaOrder
	deltaOrder := float32(1.5)
	err = writer.UpdateDelta(ctx, deltaHeaderID, nil, nil, nil, nil, &deltaOrder)
	require.NoError(t, err)

	// Verify deltaOrder was persisted
	retrieved, err := reader.GetByID(ctx, deltaHeaderID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should not be nil")
	require.Equal(t, float32(1.5), *retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should be 1.5")
}

func TestHeaderWriter_UpdateDelta_DeltaOrderNil(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)

	writer := NewHeaderWriterFromQueries(db)
	reader := NewHeaderReaderFromQueries(db)

	// Create parent HTTP request
	httpService := New(db, nil)
	parentHttpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          parentHttpID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Parent HTTP",
	})
	require.NoError(t, err)

	// Create parent header
	parentHeaderID := idwrap.NewNow()
	parentHeader := &mhttp.HTTPHeader{
		ID:      parentHeaderID,
		HttpID:  parentHttpID,
		Key:     "Content-Type",
		Value:   "application/json",
		Enabled: true,
	}
	err = writer.Create(ctx, parentHeader)
	require.NoError(t, err)

	// Create delta HTTP request
	deltaHttpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  idwrap.NewNow(),
		Name:         "Delta HTTP",
		IsDelta:      true,
		ParentHttpID: &parentHttpID,
	})
	require.NoError(t, err)

	// Create delta header with initial deltaOrder
	deltaHeaderID := idwrap.NewNow()
	deltaKey := "Authorization"
	deltaValue := "Bearer token123"
	deltaEnabled := true
	initialOrder := float32(2.0)
	deltaHeader := &mhttp.HTTPHeader{
		ID:                 deltaHeaderID,
		HttpID:             deltaHttpID,
		IsDelta:            true,
		ParentHttpHeaderID: &parentHeaderID,
		DeltaKey:           &deltaKey,
		DeltaValue:         &deltaValue,
		DeltaEnabled:       &deltaEnabled,
		DeltaDisplayOrder:  &initialOrder,
	}
	err = writer.Create(ctx, deltaHeader)
	require.NoError(t, err)

	// Verify initial deltaOrder was set
	retrieved, err := reader.GetByID(ctx, deltaHeaderID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should be set initially")
	require.Equal(t, float32(2.0), *retrieved.DeltaDisplayOrder, "Initial DeltaDisplayOrder should be 2.0")

	// Update delta with nil deltaOrder to unset it
	err = writer.UpdateDelta(ctx, deltaHeaderID, nil, nil, nil, nil, nil)
	require.NoError(t, err)

	// Verify deltaOrder was unset
	retrieved, err = reader.GetByID(ctx, deltaHeaderID)
	require.NoError(t, err)
	require.Nil(t, retrieved.DeltaDisplayOrder, "DeltaDisplayOrder should be nil after unsetting")
}

func TestHeaderWriter_UpdateDelta_DeltaOrderMultiple(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)

	writer := NewHeaderWriterFromQueries(db)
	reader := NewHeaderReaderFromQueries(db)

	// Create parent HTTP request
	httpService := New(db, nil)
	parentHttpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          parentHttpID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Parent HTTP",
	})
	require.NoError(t, err)

	// Create parent header
	parentHeaderID := idwrap.NewNow()
	parentHeader := &mhttp.HTTPHeader{
		ID:      parentHeaderID,
		HttpID:  parentHttpID,
		Key:     "Content-Type",
		Value:   "application/json",
		Enabled: true,
	}
	err = writer.Create(ctx, parentHeader)
	require.NoError(t, err)

	// Create delta HTTP request
	deltaHttpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:           deltaHttpID,
		WorkspaceID:  idwrap.NewNow(),
		Name:         "Delta HTTP",
		IsDelta:      true,
		ParentHttpID: &parentHttpID,
	})
	require.NoError(t, err)

	// Create delta header
	deltaHeaderID := idwrap.NewNow()
	deltaKey := "Authorization"
	deltaValue := "Bearer token123"
	deltaEnabled := true
	deltaHeader := &mhttp.HTTPHeader{
		ID:                 deltaHeaderID,
		HttpID:             deltaHttpID,
		IsDelta:            true,
		ParentHttpHeaderID: &parentHeaderID,
		DeltaKey:           &deltaKey,
		DeltaValue:         &deltaValue,
		DeltaEnabled:       &deltaEnabled,
	}
	err = writer.Create(ctx, deltaHeader)
	require.NoError(t, err)

	// First update: set deltaOrder to 1.0
	deltaOrder1 := float32(1.0)
	err = writer.UpdateDelta(ctx, deltaHeaderID, nil, nil, nil, nil, &deltaOrder1)
	require.NoError(t, err)

	retrieved, err := reader.GetByID(ctx, deltaHeaderID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, float32(1.0), *retrieved.DeltaDisplayOrder, "First update should set order to 1.0")

	// Second update: change deltaOrder to 3.5
	deltaOrder2 := float32(3.5)
	err = writer.UpdateDelta(ctx, deltaHeaderID, nil, nil, nil, nil, &deltaOrder2)
	require.NoError(t, err)

	retrieved, err = reader.GetByID(ctx, deltaHeaderID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, float32(3.5), *retrieved.DeltaDisplayOrder, "Second update should change order to 3.5")

	// Third update: change deltaOrder to 0.25
	deltaOrder3 := float32(0.25)
	err = writer.UpdateDelta(ctx, deltaHeaderID, nil, nil, nil, nil, &deltaOrder3)
	require.NoError(t, err)

	retrieved, err = reader.GetByID(ctx, deltaHeaderID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, float32(0.25), *retrieved.DeltaDisplayOrder, "Third update should change order to 0.25")

	// Fourth update: unset deltaOrder
	err = writer.UpdateDelta(ctx, deltaHeaderID, nil, nil, nil, nil, nil)
	require.NoError(t, err)

	retrieved, err = reader.GetByID(ctx, deltaHeaderID)
	require.NoError(t, err)
	require.Nil(t, retrieved.DeltaDisplayOrder, "Fourth update should unset the order")

	// Fifth update: set deltaOrder again to 5.0
	deltaOrder5 := float32(5.0)
	err = writer.UpdateDelta(ctx, deltaHeaderID, nil, nil, nil, nil, &deltaOrder5)
	require.NoError(t, err)

	retrieved, err = reader.GetByID(ctx, deltaHeaderID)
	require.NoError(t, err)
	require.NotNil(t, retrieved.DeltaDisplayOrder)
	require.Equal(t, float32(5.0), *retrieved.DeltaDisplayOrder, "Fifth update should set order to 5.0")
}
