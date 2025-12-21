package rimportv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/db/pkg/sqlitemem"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/sfile"
	"the-dev-tools/server/pkg/service/shttp"
)

func TestDeduplicator_ResolveHTTP(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries := gen.New(db)
	httpSvc := shttp.New(queries, nil)
	fileSvc := sfile.New(queries, nil)
	dedup := NewDeduplicator(httpSvc, *fileSvc, nil)
	workspaceID := idwrap.NewNow()

	// Scenario 1: New Request (Creation)
	req1 := &mhttp.HTTP{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Url:         "https://example.com",
		Method:      "GET",
		Name:        "Request 1",
	}

	resultID1, isNew1, hash1, err := dedup.ResolveHTTP(ctx, req1, nil, nil, nil, nil, nil, "")
	require.NoError(t, err)
	require.True(t, isNew1)
	require.NotEmpty(t, hash1)
	require.Equal(t, req1.ID, resultID1)

	// Scenario 2: Identical Request (Deduplication)
	req2 := &mhttp.HTTP{
		ID:          idwrap.NewNow(), // Different ID
		WorkspaceID: workspaceID,
		Url:         "https://example.com",
		Method:      "GET",
		Name:        "Request 1",
	}

	resultID2, isNew2, _, err := dedup.ResolveHTTP(ctx, req2, nil, nil, nil, nil, nil, "")
	require.NoError(t, err)
	require.False(t, isNew2)
	require.Equal(t, req1.ID.String(), resultID2.String(), "Should return existing ID from req1")

	// Scenario 3: Different Workspace (No Deduplication)
	workspaceID2 := idwrap.NewNow()
	req3 := &mhttp.HTTP{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID2,
		Url:         "https://example.com",
		Method:      "GET",
		Name:        "Request 1",
	}

	resultID3, isNew3, _, err := dedup.ResolveHTTP(ctx, req3, nil, nil, nil, nil, nil, "")
	require.NoError(t, err)
	require.True(t, isNew3)
	require.NotEqual(t, req1.ID.String(), resultID3.String(), "Should create new request for different workspace")

	_ = db
}

func TestDeduplicator_ResolveFile(t *testing.T) {
	ctx := context.Background()
	db, cleanup, err := sqlitemem.NewSQLiteMem(ctx)
	require.NoError(t, err)
	defer cleanup()

	queries := gen.New(db)
	httpSvc := shttp.New(queries, nil)
	fileSvc := sfile.New(queries, nil)
	dedup := NewDeduplicator(httpSvc, *fileSvc, nil)
	workspaceID := idwrap.NewNow()

	// Scenario 1: New Folder (Creation)
	folder := &mfile.File{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "api",
		ContentType: mfile.ContentTypeFolder,
	}
	path := "/api"

		resultID, isNew, err := dedup.ResolveFile(ctx, folder, path)

		require.NoError(t, err)

		require.True(t, isNew)

		require.Equal(t, folder.ID, resultID)

	

		// Scenario 2: Cache Hit (Memory)

		// Should not hit DB

		resultIDCached, isNewCached, err := dedup.ResolveFile(ctx, folder, path)

		require.NoError(t, err)

		require.False(t, isNewCached)

		require.Equal(t, folder.ID, resultIDCached)

	

		// Scenario 3: New Deduplicator (DB Hit)

		dedup2 := NewDeduplicator(httpSvc, *fileSvc, nil)

		resultIDDB, isNewDB, err := dedup2.ResolveFile(ctx, folder, path)

		require.NoError(t, err)

		require.False(t, isNewDB)

		require.Equal(t, folder.ID, resultIDDB, "Should find existing ID in DB")

	}

	