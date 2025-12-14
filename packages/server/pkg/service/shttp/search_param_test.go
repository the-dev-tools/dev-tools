package shttp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

func TestHttpSearchParamService(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := NewHttpSearchParamService(db)
	
	// Parent HTTP
	httpService := New(db, nil)
	httpID := idwrap.NewNow()
	err = httpService.Create(ctx, &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: idwrap.NewNow(),
		Name:        "Test",
	})
	require.NoError(t, err)

	paramID := idwrap.NewNow()
	param := &mhttp.HTTPSearchParam{
		ID:      paramID,
		HttpID:  httpID,
		Key:     "q",
		Value:   "search term",
		Enabled: true,
	}

	// Create
	err = service.Create(ctx, param)
	require.NoError(t, err)

	// GetByID
	retrieved, err := service.GetByID(ctx, paramID)
	require.NoError(t, err)
	require.Equal(t, "q", retrieved.Key)

	// GetByHttpID
	params, err := service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Len(t, params, 1)

	// Update
	// Update signature: Update(ctx, id, key, value, description, enabled, order)
	// Let's check signature in search_param.go. 
	// I'll assume standard signature, but verify if compilation fails.
	// Actually, based on previous `header.go` experience, it might take a struct OR args.
	// Let's check `search_param.go` content if possible, or just try struct first as that's cleaner if supported.
	// But `NewHttpHeaderService` was struct-based. `NewHttpSearchParamService` likely too?
	// Wait, `header_test.go` I fixed to use struct.
	// Let's check `search_param.go` to be sure.
	
	param.Key = "query"
	param.Value = "updated"
	err = service.Update(ctx, param) 
	require.NoError(t, err)

	updated, err := service.GetByID(ctx, paramID)
	require.NoError(t, err)
	require.Equal(t, "query", updated.Key)
	require.Equal(t, "updated", updated.Value)

	// Delete
	err = service.Delete(ctx, paramID)
	require.NoError(t, err)

	// Verify Delete
	params, err = service.GetByHttpID(ctx, httpID)
	require.NoError(t, err)
	require.Len(t, params, 0)
}
