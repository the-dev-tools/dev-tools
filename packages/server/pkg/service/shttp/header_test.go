package shttp

import (
	"context"
	"testing"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
)

func TestHttpHeaderService(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := NewHttpHeaderService(db)

	// Need a parent HTTP request
	httpService := New(db, nil)
	workspaceID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "Test",
	})
	require.NoError(t, err)

	headerID := idwrap.NewNow()
	header := &mhttp.HTTPHeader{
		ID:      headerID,
		HttpID:  httpID,
		Key:     "Content-Type",
		Value:   "application/json",
		Enabled: true,
	}

	// Create
	err = service.Create(ctx, header)
	require.NoError(t, err)

	// GetByID
	retrieved, err := service.GetByID(ctx, headerID)
	require.NoError(t, err)
	require.Equal(t, "Content-Type", retrieved.Key)

	// GetByHttpID
	headers, err := service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Len(t, headers, 1)

	// Update
	header.Key = "Accept"
	header.Value = "text/plain"
	header.Description = "Updated desc"
	err = service.Update(ctx, header)
	require.NoError(t, err)

	updated, err := service.GetByID(ctx, headerID)
	require.NoError(t, err)
	require.Equal(t, "Accept", updated.Key)
	require.Equal(t, "text/plain", updated.Value)

	// Delete
	err = service.Delete(ctx, headerID)
	require.NoError(t, err)

	// Verify Delete
	headers, err = service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Len(t, headers, 0)
}

func TestHttpHeaderService_Delta(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := NewHttpHeaderService(db)

	// Create Delta Request
	httpService := New(db, nil)
	deltaID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  idwrap.NewNow(),
		Name:         "Delta",
		IsDelta:      true,
		ParentHttpID: &idwrap.IDWrap{}, // Fake parent
	})
	require.NoError(t, err)

	// Create Delta Header
	headerID := idwrap.NewNow()
	deltaKey := "Delta-Key"
	deltaValue := "Delta-Value"
	deltaEnabled := true

	header := &mhttp.HTTPHeader{
		ID:           headerID,
		HttpID:       deltaID,
		IsDelta:      true,
		DeltaKey:     &deltaKey,
		DeltaValue:   &deltaValue,
		DeltaEnabled: &deltaEnabled,
		// Needs ParentHttpHeaderID constraint usually, but we bypass for unit test if DB constraint allows or we mock parent
		ParentHttpHeaderID: &idwrap.IDWrap{},
	}

	err = service.Create(ctx, header)
	require.NoError(t, err)

	// Update Delta
	newVal := "New Delta"
	err = service.UpdateDelta(ctx, headerID, nil, &newVal, nil, nil)
	require.NoError(t, err)

	updated, err := service.GetByID(ctx, headerID)
	require.NoError(t, err)
	require.Equal(t, "New Delta", *updated.DeltaValue)
}
