package shttp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

func TestHttpAssertService(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := NewHttpAssertService(db)
	
	// Parent HTTP
	httpService := New(db, nil)
	httpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test",
	})
	require.NoError(t, err)

	assertID := idwrap.NewNow()
	assert := &mhttp.HTTPAssert{
		ID:      assertID,
		HttpID:  httpID,
		Value:   "response.status == 200",
		Enabled: true,
	}

	// Create
	err = service.Create(ctx, assert)
	require.NoError(t, err)

	// GetByID
	retrieved, err := service.GetByID(ctx, assertID)
	require.NoError(t, err)
	require.Equal(t, "response.status == 200", retrieved.Value)

	// GetByHttpID
	asserts, err := service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Len(t, asserts, 1)

	// Update
	assert.Value = "response.status == 201"
	err = service.Update(ctx, assert)
	require.NoError(t, err)

	updated, err := service.GetByID(ctx, assertID)
	require.NoError(t, err)
	require.Equal(t, "response.status == 201", updated.Value)

	// Delete
	err = service.Delete(ctx, assertID)
	require.NoError(t, err)

	// Verify Delete
	asserts, err = service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Len(t, asserts, 0)
}
