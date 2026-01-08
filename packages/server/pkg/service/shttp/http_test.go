package shttp

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/dbtest"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"

	"github.com/stretchr/testify/require"
)

func TestHttpService(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	// Create service
	logger := slog.Default()
	service := New(db, logger)

	// Test data
	workspaceID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP
	http := &mhttp.HTTP{
		ID:               httpID,
		WorkspaceID:      workspaceID,
		Name:             "Test HTTP",
		Url:              "https://api.example.com/test",
		Method:           "GET",
		Description:      "Test HTTP endpoint",
		ParentHttpID:     nil,
		IsDelta:          false,
		DeltaName:        nil,
		DeltaUrl:         nil,
		DeltaMethod:      nil,
		DeltaDescription: nil,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	// Test Create
	err = service.Create(ctx, http)
	require.NoError(t, err)

	// Test Get
	retrieved, err := service.Get(ctx, httpID)
	require.NoError(t, err)

	require.Equal(t, httpID, retrieved.ID)
	require.Equal(t, "Test HTTP", retrieved.Name)
	require.Equal(t, "https://api.example.com/test", retrieved.Url)
	require.Equal(t, "GET", retrieved.Method)
	require.Equal(t, "Test HTTP endpoint", retrieved.Description)

	// Test GetByWorkspaceID
	https, err := service.GetByWorkspaceID(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, https, 1)

	// Test Update
	http.Name = "Updated HTTP"
	http.Url = "https://api.example.com/updated"
	http.Method = "PUT"
	http.Description = "Updated description"
	err = service.Update(ctx, http)
	require.NoError(t, err)

	// Verify update
	updated, err := service.Get(ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, "Updated HTTP", updated.Name)
	require.Equal(t, "https://api.example.com/updated", updated.Url)
	require.Equal(t, "PUT", updated.Method)
	require.Equal(t, "Updated description", updated.Description)

	// Test Delete
	err = service.Delete(ctx, httpID)
	require.NoError(t, err)

	// Verify deletion
	_, err = service.Get(ctx, httpID)
	require.Error(t, err, "Expected error when getting deleted HTTP")
	require.Equal(t, ErrNoHTTPFound, err)
}

func TestHttpService_TX(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	// Create service
	logger := slog.Default()
	service := New(db, logger)

	// Test TX wrapper
	txService := service.TX(nil) // nil tx for testing

	// The TX service should have the same type but different queries instance
	require.NotEqual(t, service.queries, txService.queries)
}

func TestHttpService_ConvertToDBHTTP(t *testing.T) {
	now := time.Now().Unix()
	parentID := idwrap.NewNow()
	http := mhttp.HTTP{
		ID:               idwrap.NewNow(),
		WorkspaceID:      idwrap.NewNow(),
		Name:             "Test HTTP",
		Url:              "https://api.example.com/test",
		Method:           "GET",
		Description:      "Test description",
		ParentHttpID:     &parentID,
		IsDelta:          true,
		DeltaName:        stringPtr("Delta Name"),
		DeltaUrl:         stringPtr("https://delta.example.com"),
		DeltaMethod:      stringPtr("POST"),
		DeltaDescription: stringPtr("Delta description"),
		LastRunAt:        int64Ptr(now),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	dbHttp := ConvertToDBHTTP(http)

	require.Equal(t, http.ID, dbHttp.ID)
	require.Equal(t, http.Name, dbHttp.Name)
	require.Equal(t, http.Url, dbHttp.Url)
	require.Equal(t, http.Method, dbHttp.Method)
	require.Equal(t, http.Description, dbHttp.Description)
	require.Equal(t, http.IsDelta, dbHttp.IsDelta)
	require.Equal(t, *http.LastRunAt, dbHttp.LastRunAt)
}

func TestHttpService_ConvertToModelHTTP(t *testing.T) {
	now := time.Now().Unix()
	parentID := idwrap.NewNow()
	dbHttp := gen.Http{
		ID:               idwrap.NewNow(),
		WorkspaceID:      idwrap.NewNow(),
		Name:             "Test HTTP",
		Url:              "https://api.example.com/test",
		Method:           "GET",
		Description:      "Test description",
		ParentHttpID:     &parentID,
		IsDelta:          true,
		DeltaName:        stringPtr("Delta Name"),
		DeltaUrl:         stringPtr("https://delta.example.com"),
		DeltaMethod:      stringPtr("POST"),
		DeltaDescription: stringPtr("Delta description"),
		LastRunAt:        now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	http := ConvertToModelHTTP(dbHttp)

	require.Equal(t, dbHttp.ID, http.ID)
	require.Equal(t, dbHttp.Name, http.Name)
	require.Equal(t, dbHttp.Url, http.Url)
	require.Equal(t, dbHttp.Method, http.Method)
	require.Equal(t, dbHttp.Description, http.Description)
	require.Equal(t, dbHttp.IsDelta, http.IsDelta)
	require.Equal(t, dbHttp.LastRunAt.(int64), *http.LastRunAt)
}

func TestHttpService_GetWorkspaceID(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	// Create service
	logger := slog.Default()
	service := New(db, logger)

	// Test data
	workspaceID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP
	http := &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "Test HTTP",
		Url:         "https://api.example.com/test",
		Method:      "GET",
		Description: "Test HTTP endpoint",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = service.Create(ctx, http)
	require.NoError(t, err)

	// Test GetWorkspaceID
	retrievedWorkspaceID, err := service.GetWorkspaceID(ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, workspaceID, retrievedWorkspaceID)

	// Test with non-existent HTTP
	nonExistentID := idwrap.NewNow()
	_, err = service.GetWorkspaceID(ctx, nonExistentID)
	require.Error(t, err, "Expected error when getting workspace ID for non-existent HTTP")
	require.Equal(t, ErrNoHTTPFound, err)
}

func TestHttpService_NewTX(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	testDB, err := dbtest.GetTestDB(ctx)
	require.NoError(t, err)
	defer testDB.Close()

	// Start a transaction
	tx, err := testDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	// Test NewTX with real transaction
	txService, err := NewTX(ctx, tx)
	require.NoError(t, err)
	require.NotNil(t, txService)
}

func TestHttpService_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	// Create service
	logger := slog.Default()
	service := New(db, logger)

	// Test operations with non-existent IDs
	nonExistentID := idwrap.NewNow()

	// Get non-existent HTTP
	_, err = service.Get(ctx, nonExistentID)
	require.Error(t, err, "Expected error when getting non-existent HTTP")
	require.Equal(t, ErrNoHTTPFound, err)

	// Get workspace ID for non-existent HTTP
	_, err = service.GetWorkspaceID(ctx, nonExistentID)
	require.Error(t, err, "Expected error when getting workspace ID for non-existent HTTP")
	require.Equal(t, ErrNoHTTPFound, err)
}

// Helper function
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func TestHttpService_Upsert(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := New(db, slog.Default())
	httpID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	http := &mhttp.HTTP{
		ID:          httpID,
		WorkspaceID: workspaceID,
		Name:        "Original Name",
		Url:         "http://example.com",
		Method:      "GET",
	}

	// Upsert (Create)
	err = service.Upsert(ctx, http)
	require.NoError(t, err)

	retrieved, err := service.Get(ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, "Original Name", retrieved.Name)

	// Upsert (Update)
	http.Name = "Updated Name"
	err = service.Upsert(ctx, http)
	require.NoError(t, err)

	retrieved, err = service.Get(ctx, httpID)
	require.NoError(t, err)
	require.Equal(t, "Updated Name", retrieved.Name)
}

func TestHttpService_Deltas(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := New(db, slog.Default())
	workspaceID := idwrap.NewNow()
	baseID := idwrap.NewNow()

	// Create Base
	baseHttp := &mhttp.HTTP{
		ID:          baseID,
		WorkspaceID: workspaceID,
		Name:        "Base",
		IsDelta:     false,
	}
	err = service.Create(ctx, baseHttp)
	require.NoError(t, err)

	// Create Delta
	deltaID := idwrap.NewNow()
	deltaName := "Delta"
	deltaHttp := &mhttp.HTTP{
		ID:           deltaID,
		WorkspaceID:  workspaceID,
		Name:         "Delta Base",
		ParentHttpID: &baseID,
		IsDelta:      true,
		DeltaName:    &deltaName,
	}
	err = service.Create(ctx, deltaHttp)
	require.NoError(t, err)

	// Test GetDeltasByWorkspaceID
	deltas, err := service.GetDeltasByWorkspaceID(ctx, workspaceID)
	require.NoError(t, err)
	require.Len(t, deltas, 1)
	require.Equal(t, deltaID, deltas[0].ID)
	require.True(t, deltas[0].IsDelta)

	// Test GetDeltasByParentID
	parentDeltas, err := service.GetDeltasByParentID(ctx, baseID)
	require.NoError(t, err)
	require.Len(t, parentDeltas, 1)
	require.Equal(t, deltaID, parentDeltas[0].ID)
}

func TestHttpService_FindByURLAndMethod(t *testing.T) {
	ctx := context.Background()
	db, err := dbtest.GetTestPreparedQueries(ctx)
	require.NoError(t, err)
	defer db.Close()

	service := New(db, slog.Default())
	workspaceID := idwrap.NewNow()

	http := &mhttp.HTTP{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        "Find Me",
		Url:         "http://find.me",
		Method:      "POST",
	}
	err = service.Create(ctx, http)
	require.NoError(t, err)

	// Found
	found, err := service.FindByURLAndMethod(ctx, workspaceID, "http://find.me", "POST")
	require.NoError(t, err)
	require.Equal(t, http.ID, found.ID)

	// Not Found
	_, err = service.FindByURLAndMethod(ctx, workspaceID, "http://find.me", "GET")
	require.Error(t, err)
	require.Equal(t, ErrNoHTTPFound, err)
}
