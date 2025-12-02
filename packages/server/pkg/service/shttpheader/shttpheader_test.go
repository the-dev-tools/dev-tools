package shttpheader

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttpheader"
)

// Helper functions for pointer creation
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func float32Ptr(f float32) *float32 {
	return &f
}

func TestHttpHeaderService_CreateGetUpdateDelete(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test data
	httpID := idwrap.NewNow()
	headerID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test header
	header := &mhttpheader.HttpHeader{
		ID:          headerID,
		HttpID:      httpID,
		Key:         "Content-Type",
		Value:       "application/json",
		Enabled:     true,
		Description: "Test header",
		Order:       1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Test Create
	err = service.Create(ctx, header)
	if err != nil {
		t.Fatalf("Failed to create HttpHeader: %v", err)
	}

	// Test GetByID
	retrieved, err := service.GetByID(ctx, headerID)
	if err != nil {
		t.Fatalf("Failed to get HttpHeader: %v", err)
	}

	if retrieved.ID != headerID {
		t.Errorf("Expected ID %s, got %s", headerID, retrieved.ID)
	}

	if retrieved.Key != "Content-Type" {
		t.Errorf("Expected Key Content-Type, got %s", retrieved.Key)
	}

	if retrieved.Value != "application/json" {
		t.Errorf("Expected Value application/json, got %s", retrieved.Value)
	}

	// Test GetByHttpID
	headers, err := service.GetByHttpID(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get HttpHeader by HttpID: %v", err)
	}

	if len(headers) != 1 {
		t.Errorf("Expected 1 header, got %d", len(headers))
	}

	// Test Update
	header.Key = "Authorization"
	header.Value = "Bearer token"
	err = service.Update(ctx, header)
	if err != nil {
		t.Fatalf("Failed to update HttpHeader: %v", err)
	}

	// Verify update
	updated, err := service.GetByID(ctx, headerID)
	if err != nil {
		t.Fatalf("Failed to get updated HttpHeader: %v", err)
	}

	if updated.Key != "Authorization" {
		t.Errorf("Expected Key Authorization, got %s", updated.Key)
	}

	if updated.Value != "Bearer token" {
		t.Errorf("Expected Value Bearer token, got %s", updated.Value)
	}

	// Test Delete
	err = service.Delete(ctx, headerID)
	if err != nil {
		t.Fatalf("Failed to delete HttpHeader: %v", err)
	}

	// Verify deletion
	_, err = service.GetByID(ctx, headerID)
	if err == nil {
		t.Errorf("Expected error when getting deleted HttpHeader, got nil")
	}

	if err != ErrNoHttpHeaderFound {
		t.Errorf("Expected ErrNoHttpHeaderFound, got %v", err)
	}
}

func TestHttpHeaderService_CreateBulk(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test data
	httpID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test headers for bulk operation
	var headers []mhttpheader.HttpHeader
	for i := 0; i < 15; i++ {
		headerID := idwrap.NewNow()
		headers = append(headers, mhttpheader.HttpHeader{
			ID:          headerID,
			Key:         "Header-" + string(rune('A'+i)),
			Value:       "Value-" + string(rune('A'+i)),
			Enabled:     true,
			Description: "Test header " + string(rune('A'+i)),
			Order:       float32(i),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	// Test CreateBulk
	err = service.CreateBulk(ctx, httpID, headers)
	if err != nil {
		t.Fatalf("Failed to create bulk HttpHeaders: %v", err)
	}

	// Verify all headers were created
	retrievedHeaders, err := service.GetByHttpID(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get HttpHeaders by HttpID: %v", err)
	}

	if len(retrievedHeaders) != 15 {
		t.Errorf("Expected 15 headers, got %d", len(retrievedHeaders))
	}

	// Test GetByIDs with actual header IDs
	var headerIDs []idwrap.IDWrap
	for _, header := range headers {
		headerIDs = append(headerIDs, header.ID)
	}
	headersByIDs, err := service.GetByIDs(ctx, headerIDs)
	if err != nil {
		t.Fatalf("Failed to get HttpHeaders by IDs: %v", err)
	}

	if len(headersByIDs) != 15 {
		t.Errorf("Expected 15 headers by IDs, got %d", len(headersByIDs))
	}
}

func TestHttpHeaderService_UpdateDelta(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test data
	httpID := idwrap.NewNow()
	headerID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test header
	header := &mhttpheader.HttpHeader{
		ID:          headerID,
		HttpID:      httpID,
		Key:         "Content-Type",
		Value:       "application/json",
		Enabled:     true,
		Description: "Test header",
		Order:       1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = service.Create(ctx, header)
	if err != nil {
		t.Fatalf("Failed to create HttpHeader: %v", err)
	}

	// Test UpdateDelta
	deltaKey := stringPtr("X-Custom-Header")
	deltaValue := stringPtr("custom-value")
	deltaEnabled := boolPtr(false)
	deltaDescription := stringPtr("Delta description")

	err = service.UpdateDelta(ctx, headerID, deltaKey, deltaValue, deltaDescription, deltaEnabled)
	if err != nil {
		t.Fatalf("Failed to update HttpHeader delta: %v", err)
	}

	// Verify delta update
	retrieved, err := service.GetByID(ctx, headerID)
	if err != nil {
		t.Fatalf("Failed to get HttpHeader after delta update: %v", err)
	}

	if retrieved.DeltaKey == nil || *retrieved.DeltaKey != "X-Custom-Header" {
		t.Errorf("Expected DeltaKey X-Custom-Header, got %v", retrieved.DeltaKey)
	}

	if retrieved.DeltaValue == nil || *retrieved.DeltaValue != "custom-value" {
		t.Errorf("Expected DeltaValue custom-value, got %v", retrieved.DeltaValue)
	}

	if retrieved.DeltaEnabled == nil || *retrieved.DeltaEnabled != false {
		t.Errorf("Expected DeltaEnabled false, got %v", retrieved.DeltaEnabled)
	}

	if retrieved.DeltaDescription == nil || *retrieved.DeltaDescription != "Delta description" {
		t.Errorf("Expected DeltaDescription 'Delta description', got %v", retrieved.DeltaDescription)
	}
}

func TestHttpHeaderService_UpdateOrder(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test data
	httpID := idwrap.NewNow()
	headerID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test header
	header := &mhttpheader.HttpHeader{
		ID:          headerID,
		HttpID:      httpID,
		Key:         "Content-Type",
		Value:       "application/json",
		Enabled:     true,
		Description: "Test header",
		Order:       1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	err = service.Create(ctx, header)
	if err != nil {
		t.Fatalf("Failed to create HttpHeader: %v", err)
	}

	// Test UpdateOrder
	newOrder := 2.0
	err = service.UpdateOrder(ctx, headerID, newOrder)
	if err != nil {
		t.Fatalf("Failed to update HttpHeader order: %v", err)
	}

	// Verify update
	updatedHeader, err := service.GetByID(ctx, headerID)
	if err != nil {
		t.Fatalf("Failed to get updated HttpHeader: %v", err)
	}

	if float64(updatedHeader.Order) != newOrder {
		t.Errorf("Expected Order %f, got %f", newOrder, updatedHeader.Order)
	}
}

func TestHttpHeaderService_GetByHttpIDOrdered(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test data
	httpID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create headers with different creation times to test ordering
	var headers []mhttpheader.HttpHeader
	for i := 0; i < 5; i++ {
		headerID := idwrap.NewNow()
		headers = append(headers, mhttpheader.HttpHeader{
			ID:          headerID,
			HttpID:      httpID,
			Key:         "Header-" + string(rune('A'+i)),
			Value:       "Value-" + string(rune('A'+i)),
			Enabled:     true,
			Description: "Test header " + string(rune('A'+i)),
			Order:       float32(i),
			CreatedAt:   now + int64(i), // Different creation times
			UpdatedAt:   now + int64(i),
		})
	}

	// Create headers with a small delay to ensure different timestamps
	for i, header := range headers {
		time.Sleep(time.Millisecond)
		header.CreatedAt = time.Now().Unix()
		header.UpdatedAt = header.CreatedAt
		err = service.Create(ctx, &header)
		if err != nil {
			t.Fatalf("Failed to create header %d: %v", i, err)
		}
	}

	// Test GetByHttpIDOrdered
	orderedHeaders, err := service.GetByHttpIDOrdered(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get ordered HttpHeaders: %v", err)
	}

	if len(orderedHeaders) != 5 {
		t.Errorf("Expected 5 ordered headers, got %d", len(orderedHeaders))
	}

	// Verify ordering (should be sorted by CreatedAt)
	for i := 1; i < len(orderedHeaders); i++ {
		if orderedHeaders[i-1].CreatedAt > orderedHeaders[i].CreatedAt {
			t.Errorf("Headers not properly ordered by CreatedAt")
			break
		}
	}
}

func TestHttpHeaderService_Serialization(t *testing.T) {
	// Test data
	headerID := idwrap.NewNow()
	httpID := idwrap.NewNow()
	parentID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test header with all fields
	header := mhttpheader.HttpHeader{
		ID:                 headerID,
		HttpID:             httpID,
		Key:                "Content-Type",
		Value:              "application/json",
		Enabled:            true,
		Description:        "Test header",
		Order:              1.0,
		ParentHttpHeaderID: &parentID,
		IsDelta:            true,
		DeltaKey:           stringPtr("X-Delta-Key"),
		DeltaValue:         stringPtr("delta-value"),
		DeltaEnabled:       boolPtr(false),
		DeltaDescription:   stringPtr("Delta description"),
		DeltaOrder:         float32Ptr(2.0),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Test SerializeModelToGen
	dbHeader := SerializeModelToGen(header)

	if dbHeader.ID != header.ID {
		t.Errorf("Serialization failed: ID mismatch")
	}

	if dbHeader.HeaderKey != header.Key {
		t.Errorf("Serialization failed: Key mismatch")
	}

	if dbHeader.HeaderValue != header.Value {
		t.Errorf("Serialization failed: Value mismatch")
	}

	// Test DeserializeGenToModel
	modelHeader := DeserializeGenToModel(dbHeader)

	if modelHeader.ID != header.ID {
		t.Errorf("Deserialization failed: ID mismatch")
	}

	if modelHeader.Key != header.Key {
		t.Errorf("Deserialization failed: Key mismatch")
	}

	if modelHeader.Value != header.Value {
		t.Errorf("Deserialization failed: Value mismatch")
	}
}

func TestHttpHeaderService_TX(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test TX wrapper
	txService := service.TX(nil) // nil tx for testing

	// The TX service should have the same type but different queries instance
	if txService.queries == service.queries {
		t.Errorf("TX service should have different queries instance")
	}
}

func TestHttpHeaderService_NewTX(t *testing.T) {
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

func TestHttpHeaderService_ErrorCases(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test Create with nil header
	err = service.Create(ctx, nil)
	if err == nil {
		t.Errorf("Expected error when creating nil header")
	}

	// Test Update with nil header
	err = service.Update(ctx, nil)
	if err == nil {
		t.Errorf("Expected error when updating nil header")
	}

	// Test GetByID with non-existent ID
	nonExistentID := idwrap.NewNow()
	_, err = service.GetByID(ctx, nonExistentID)
	if err == nil {
		t.Errorf("Expected error when getting non-existent header")
	}

	if err != ErrNoHttpHeaderFound {
		t.Errorf("Expected ErrNoHttpHeaderFound, got %v", err)
	}

	// Test GetByHttpID with non-existent HttpID
	nonExistentHttpID := idwrap.NewNow()
	headers, err := service.GetByHttpID(ctx, nonExistentHttpID)
	if err != nil {
		t.Errorf("Unexpected error when getting headers by non-existent HttpID: %v", err)
	}

	if len(headers) != 0 {
		t.Errorf("Expected 0 headers for non-existent HttpID, got %d", len(headers))
	}
}
