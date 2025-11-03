package shttp

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

func TestHttpService(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Failed to create HTTP: %v", err)
	}

	// Test Get
	retrieved, err := service.Get(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get HTTP: %v", err)
	}

	if retrieved.ID != httpID {
		t.Errorf("Expected ID %s, got %s", httpID, retrieved.ID)
	}

	if retrieved.Name != "Test HTTP" {
		t.Errorf("Expected Name Test HTTP, got %s", retrieved.Name)
	}

	if retrieved.Url != "https://api.example.com/test" {
		t.Errorf("Expected Url https://api.example.com/test, got %s", retrieved.Url)
	}

	if retrieved.Method != "GET" {
		t.Errorf("Expected Method GET, got %s", retrieved.Method)
	}

	if retrieved.Description != "Test HTTP endpoint" {
		t.Errorf("Expected Description Test HTTP endpoint, got %s", retrieved.Description)
	}

	// Test GetByWorkspaceID
	https, err := service.GetByWorkspaceID(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Failed to get HTTPs by WorkspaceID: %v", err)
	}

	if len(https) != 1 {
		t.Errorf("Expected 1 HTTP, got %d", len(https))
	}

	// Test Update
	http.Name = "Updated HTTP"
	http.Url = "https://api.example.com/updated"
	http.Method = "PUT"
	http.Description = "Updated description"
	err = service.Update(ctx, http)
	if err != nil {
		t.Fatalf("Failed to update HTTP: %v", err)
	}

	// Verify update
	updated, err := service.Get(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get updated HTTP: %v", err)
	}

	if updated.Name != "Updated HTTP" {
		t.Errorf("Expected Name Updated HTTP, got %s", updated.Name)
	}

	if updated.Url != "https://api.example.com/updated" {
		t.Errorf("Expected Url https://api.example.com/updated, got %s", updated.Url)
	}

	if updated.Method != "PUT" {
		t.Errorf("Expected Method PUT, got %s", updated.Method)
	}

	if updated.Description != "Updated description" {
		t.Errorf("Expected Description Updated description, got %s", updated.Description)
	}

	// Test Delete
	err = service.Delete(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to delete HTTP: %v", err)
	}

	// Verify deletion
	_, err = service.Get(ctx, httpID)
	if err == nil {
		t.Errorf("Expected error when getting deleted HTTP, got nil")
	}

	if err != ErrNoHTTPFound {
		t.Errorf("Expected ErrNoHTTPFound, got %v", err)
	}
}

func TestHttpService_TX(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	logger := slog.Default()
	service := New(db, logger)

	// Test TX wrapper
	txService := service.TX(nil) // nil tx for testing

	// The TX service should have the same type but different queries instance
	if txService.queries == service.queries {
		t.Errorf("TX service should have different queries instance")
	}
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
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	dbHttp := ConvertToDBHTTP(http)

	if dbHttp.ID != http.ID {
		t.Errorf("Expected ID %v, got %v", http.ID, dbHttp.ID)
	}

	if dbHttp.Name != http.Name {
		t.Errorf("Expected Name %s, got %s", http.Name, dbHttp.Name)
	}

	if dbHttp.Url != http.Url {
		t.Errorf("Expected Url %s, got %s", http.Url, dbHttp.Url)
	}

	if dbHttp.Method != http.Method {
		t.Errorf("Expected Method %s, got %s", http.Method, dbHttp.Method)
	}

	if dbHttp.Description != http.Description {
		t.Errorf("Expected Description %s, got %s", http.Description, dbHttp.Description)
	}

	if dbHttp.IsDelta != http.IsDelta {
		t.Errorf("Expected IsDelta %v, got %v", http.IsDelta, dbHttp.IsDelta)
	}
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
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	http := ConvertToModelHTTP(dbHttp)

	if http.ID != dbHttp.ID {
		t.Errorf("Expected ID %v, got %v", dbHttp.ID, http.ID)
	}

	if http.Name != dbHttp.Name {
		t.Errorf("Expected Name %s, got %s", dbHttp.Name, http.Name)
	}

	if http.Url != dbHttp.Url {
		t.Errorf("Expected Url %s, got %s", dbHttp.Url, http.Url)
	}

	if http.Method != dbHttp.Method {
		t.Errorf("Expected Method %s, got %s", dbHttp.Method, http.Method)
	}

	if http.Description != dbHttp.Description {
		t.Errorf("Expected Description %s, got %s", dbHttp.Description, http.Description)
	}

	if http.IsDelta != dbHttp.IsDelta {
		t.Errorf("Expected IsDelta %v, got %v", dbHttp.IsDelta, http.IsDelta)
	}
}

func TestHttpService_GetWorkspaceID(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
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
	if err != nil {
		t.Fatalf("Failed to create HTTP: %v", err)
	}

	// Test GetWorkspaceID
	retrievedWorkspaceID, err := service.GetWorkspaceID(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get workspace ID: %v", err)
	}

	if retrievedWorkspaceID != workspaceID {
		t.Errorf("Expected workspace ID %v, got %v", workspaceID, retrievedWorkspaceID)
	}

	// Test with non-existent HTTP
	nonExistentID := idwrap.NewNow()
	_, err = service.GetWorkspaceID(ctx, nonExistentID)
	if err == nil {
		t.Errorf("Expected error when getting workspace ID for non-existent HTTP")
	}

	if err != ErrNoHTTPFound {
		t.Errorf("Expected ErrNoHTTPFound, got %v", err)
	}
}

func TestHttpService_NewTX(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	testDB, err := dbtest.GetTestDB(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer testDB.Close()

	// Start a transaction
	tx, err := testDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Test NewTX with real transaction
	txService, err := NewTX(ctx, tx)
	if err != nil {
		t.Fatalf("Failed to create TX service: %v", err)
	}

	if txService == nil {
		t.Fatal("Expected non-nil TX service")
	}
}

func TestHttpService_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	logger := slog.Default()
	service := New(db, logger)

	// Test operations with non-existent IDs
	nonExistentID := idwrap.NewNow()

	// Get non-existent HTTP
	_, err = service.Get(ctx, nonExistentID)
	if err == nil {
		t.Errorf("Expected error when getting non-existent HTTP")
	}

	if err != ErrNoHTTPFound {
		t.Errorf("Expected ErrNoHTTPFound, got %v", err)
	}

	// Get workspace ID for non-existent HTTP
	_, err = service.GetWorkspaceID(ctx, nonExistentID)
	if err == nil {
		t.Errorf("Expected error when getting workspace ID for non-existent HTTP")
	}

	if err != ErrNoHTTPFound {
		t.Errorf("Expected ErrNoHTTPFound, got %v", err)
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
