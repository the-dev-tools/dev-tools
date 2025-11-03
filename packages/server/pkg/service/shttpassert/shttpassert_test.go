package shttpassert

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttpassert"
)

func TestHttpAssertService_CreateGetUpdateDelete(t *testing.T) {
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
	assertID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP assert
	httpAssert := &mhttpassert.HttpAssert{
		ID:          assertID,
		HttpID:      httpID,
		Key:         "status_code",
		Value:       "200",
		Enabled:     true,
		Description: "Status code assertion",
		Order:       1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Test Create
	err = service.CreateHttpAssert(ctx, httpAssert)
	if err != nil {
		t.Fatalf("Failed to create HTTP assert: %v", err)
	}

	// Test Get
	retrieved, err := service.GetHttpAssert(ctx, assertID)
	if err != nil {
		t.Fatalf("Failed to get HTTP assert: %v", err)
	}

	if retrieved.ID != assertID {
		t.Errorf("Expected ID %s, got %s", assertID, retrieved.ID)
	}

	if retrieved.Key != "status_code" {
		t.Errorf("Expected Key status_code, got %s", retrieved.Key)
	}

	if retrieved.Value != "200" {
		t.Errorf("Expected Value 200, got %s", retrieved.Value)
	}

	if !retrieved.Enabled {
		t.Errorf("Expected Enabled true, got %v", retrieved.Enabled)
	}

	if retrieved.Description != "Status code assertion" {
		t.Errorf("Expected Description Status code assertion, got %s", retrieved.Description)
	}

	if retrieved.Order != 1.0 {
		t.Errorf("Expected Order 1.0, got %f", retrieved.Order)
	}

	// Test Update
	httpAssert.Key = "response_time"
	httpAssert.Value = "<1000"
	httpAssert.Enabled = false
	httpAssert.Description = "Response time assertion"
	err = service.UpdateHttpAssert(ctx, httpAssert)
	if err != nil {
		t.Fatalf("Failed to update HTTP assert: %v", err)
	}

	// Verify update
	updated, err := service.GetHttpAssert(ctx, assertID)
	if err != nil {
		t.Fatalf("Failed to get updated HTTP assert: %v", err)
	}

	if updated.Key != "response_time" {
		t.Errorf("Expected Key response_time, got %s", updated.Key)
	}

	if updated.Value != "<1000" {
		t.Errorf("Expected Value <1000, got %s", updated.Value)
	}

	if updated.Enabled {
		t.Errorf("Expected Enabled false, got %v", updated.Enabled)
	}

	if updated.Description != "Response time assertion" {
		t.Errorf("Expected Description Response time assertion, got %s", updated.Description)
	}

	// Test Delete
	err = service.DeleteHttpAssert(ctx, assertID)
	if err != nil {
		t.Fatalf("Failed to delete HTTP assert: %v", err)
	}

	// Verify deletion
	_, err = service.GetHttpAssert(ctx, assertID)
	if err == nil {
		t.Errorf("Expected error when getting deleted HTTP assert, got nil")
	}

	if err != ErrNoHttpAssertFound {
		t.Errorf("Expected ErrNoHttpAssertFound, got %v", err)
	}
}

func TestHttpAssertService_GetByHttpID(t *testing.T) {
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

	// Create multiple test HTTP asserts
	httpAsserts := []*mhttpassert.HttpAssert{
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         "status_code",
			Value:       "200",
			Enabled:     true,
			Description: "Status code assertion",
			Order:       1.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         "response_time",
			Value:       "<1000",
			Enabled:     true,
			Description: "Response time assertion",
			Order:       2.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// Create asserts
	for _, assert := range httpAsserts {
		err := service.CreateHttpAssert(ctx, assert)
		if err != nil {
			t.Fatalf("Failed to create HTTP assert: %v", err)
		}
	}

	// Test GetByHttpID
	retrieved, err := service.GetHttpAssertsByHttpID(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get HTTP asserts by HttpID: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 HTTP asserts, got %d", len(retrieved))
	}

	// Test with non-existent HttpID
	nonExistentID := idwrap.NewNow()
	retrieved, err = service.GetHttpAssertsByHttpID(ctx, nonExistentID)
	if err != nil {
		t.Errorf("Expected no error for non-existent HttpID, got %v", err)
	}

	if len(retrieved) != 0 {
		t.Errorf("Expected 0 HTTP asserts for non-existent HttpID, got %d", len(retrieved))
	}
}

func TestHttpAssertService_GetByHttpIDs(t *testing.T) {
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
	httpID1 := idwrap.NewNow()
	httpID2 := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP asserts for different HTTP IDs
	httpAsserts := []*mhttpassert.HttpAssert{
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID1,
			Key:         "status_code",
			Value:       "200",
			Enabled:     true,
			Description: "Status code assertion",
			Order:       1.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID1,
			Key:         "response_time",
			Value:       "<1000",
			Enabled:     true,
			Description: "Response time assertion",
			Order:       2.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID2,
			Key:         "content_type",
			Value:       "application/json",
			Enabled:     true,
			Description: "Content type assertion",
			Order:       1.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// Create asserts
	for _, assert := range httpAsserts {
		err := service.CreateHttpAssert(ctx, assert)
		if err != nil {
			t.Fatalf("Failed to create HTTP assert: %v", err)
		}
	}

	// Test GetByHttpIDs
	httpIDs := []idwrap.IDWrap{httpID1, httpID2}
	result, err := service.GetHttpAssertsByHttpIDs(ctx, httpIDs)
	if err != nil {
		t.Fatalf("Failed to get HTTP asserts by HttpIDs: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("Expected 2 HttpIDs in result, got %d", len(result))
	}

	if len(result[httpID1]) != 2 {
		t.Errorf("Expected 2 HTTP asserts for HttpID1, got %d", len(result[httpID1]))
	}

	if len(result[httpID2]) != 1 {
		t.Errorf("Expected 1 HTTP assert for HttpID2, got %d", len(result[httpID2]))
	}

	// Test with empty HttpIDs
	emptyResult, err := service.GetHttpAssertsByHttpIDs(ctx, []idwrap.IDWrap{})
	if err != nil {
		t.Errorf("Expected no error for empty HttpIDs, got %v", err)
	}

	if len(emptyResult) != 0 {
		t.Errorf("Expected empty result for empty HttpIDs, got %d", len(emptyResult))
	}
}

func TestHttpAssertService_BulkOperations(t *testing.T) {
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

	// Create multiple test HTTP asserts for bulk operations
	httpAsserts := make([]mhttpassert.HttpAssert, 15)
	for i := 0; i < 15; i++ {
		httpAsserts[i] = mhttpassert.HttpAssert{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         "assert_" + string(rune('a'+i)),
			Value:       "value_" + string(rune('a'+i)),
			Enabled:     true,
			Description: "Assert " + string(rune('a'+i)),
			Order:       float32(i + 1),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	// Test CreateBulk
	err = service.CreateBulkHttpAssert(ctx, httpAsserts)
	if err != nil {
		t.Fatalf("Failed to create bulk HTTP asserts: %v", err)
	}

	// Verify bulk creation
	retrieved, err := service.GetHttpAssertsByHttpID(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get HTTP asserts by HttpID: %v", err)
	}

	if len(retrieved) != 15 {
		t.Errorf("Expected 15 HTTP asserts, got %d", len(retrieved))
	}
}

func TestHttpAssertService_DeltaOperations(t *testing.T) {
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
	assertID := idwrap.NewNow()
	parentID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP assert with delta
	httpAssert := &mhttpassert.HttpAssert{
		ID:                 assertID,
		HttpID:             httpID,
		Key:                "status_code",
		Value:              "200",
		Enabled:            true,
		Description:        "Status code assertion",
		Order:              1.0,
		ParentHttpAssertID: &parentID,
		IsDelta:            true,
		DeltaKey:           stringPtr("response_time"),
		DeltaValue:         stringPtr("<1000"),
		DeltaEnabled:       boolPtr(false),
		DeltaDescription:   stringPtr("Response time assertion"),
		DeltaOrder:         float32Ptr(2.0),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Test Create
	err = service.CreateHttpAssert(ctx, httpAssert)
	if err != nil {
		t.Fatalf("Failed to create HTTP assert with delta: %v", err)
	}

	// Test UpdateDelta
	newDeltaKey := stringPtr("content_type")
	newDeltaValue := stringPtr("application/json")
	newDeltaEnabled := boolPtr(true)
	newDeltaDescription := stringPtr("Content type assertion")
	newDeltaOrder := float32Ptr(3.0)

	err = service.UpdateHttpAssertDelta(ctx, assertID, newDeltaKey, newDeltaValue, newDeltaEnabled, newDeltaDescription, newDeltaOrder)
	if err != nil {
		t.Fatalf("Failed to update HTTP assert delta: %v", err)
	}

	// Verify delta update
	retrieved, err := service.GetHttpAssert(ctx, assertID)
	if err != nil {
		t.Fatalf("Failed to get updated HTTP assert: %v", err)
	}

	if *retrieved.DeltaKey != "content_type" {
		t.Errorf("Expected DeltaKey content_type, got %s", *retrieved.DeltaKey)
	}

	if *retrieved.DeltaValue != "application/json" {
		t.Errorf("Expected DeltaValue application/json, got %s", *retrieved.DeltaValue)
	}

	if !*retrieved.DeltaEnabled {
		t.Errorf("Expected DeltaEnabled true, got %v", *retrieved.DeltaEnabled)
	}

	if *retrieved.DeltaDescription != "Content type assertion" {
		t.Errorf("Expected DeltaDescription Content type assertion, got %s", *retrieved.DeltaDescription)
	}

	// Test ResetDelta
	err = service.ResetHttpAssertDelta(ctx, assertID)
	if err != nil {
		t.Fatalf("Failed to reset HTTP assert delta: %v", err)
	}

	// Verify delta reset
	retrieved, err = service.GetHttpAssert(ctx, assertID)
	if err != nil {
		t.Fatalf("Failed to get HTTP assert after delta reset: %v", err)
	}

	if retrieved.IsDelta {
		t.Errorf("Expected IsDelta false after reset, got %v", retrieved.IsDelta)
	}

	if retrieved.DeltaKey != nil {
		t.Errorf("Expected DeltaKey nil after reset, got %v", retrieved.DeltaKey)
	}
}

func TestHttpAssertService_OrderOperations(t *testing.T) {
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
	assertID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP assert
	httpAssert := &mhttpassert.HttpAssert{
		ID:          assertID,
		HttpID:      httpID,
		Key:         "status_code",
		Value:       "200",
		Enabled:     true,
		Description: "Status code assertion",
		Order:       1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create
	err = service.CreateHttpAssert(ctx, httpAssert)
	if err != nil {
		t.Fatalf("Failed to create HTTP assert: %v", err)
	}

	// Test UpdateOrder
	newOrder := float32(5.0)
	err = service.UpdateHttpAssertOrder(ctx, assertID, httpID, newOrder)
	if err != nil {
		t.Fatalf("Failed to update HTTP assert order: %v", err)
	}

	// Verify order update
	retrieved, err := service.GetHttpAssert(ctx, assertID)
	if err != nil {
		t.Fatalf("Failed to get HTTP assert after order update: %v", err)
	}

	if retrieved.Order != 5.0 {
		t.Errorf("Expected Order 5.0, got %f", retrieved.Order)
	}
}

func TestHttpAssertService_TransactionSupport(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test transaction wrapper
	txService := service.TX(nil)
	// Since TX returns a service, not a pointer, we can't compare to nil
	// Just verify it doesn't panic
	_ = txService

	// Test NewTX
	testDB, err := dbtest.GetTestDB(ctx)
	if err != nil {
		t.Fatalf("Failed to get test DB: %v", err)
	}
	defer testDB.Close()

	tx, err := testDB.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	newTxService, err := NewTX(ctx, tx)
	if err != nil {
		t.Fatalf("Failed to create TX service: %v", err)
	}

	if newTxService == nil {
		t.Errorf("Expected non-nil NewTX service")
	}
}

func TestHttpAssertService_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Create in-memory database for testing
	db, err := dbtest.GetTestPreparedQueries(ctx)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// Create service
	service := New(db)

	// Test operations with non-existent IDs
	nonExistentID := idwrap.NewNow()

	// Get non-existent HTTP assert
	_, err = service.GetHttpAssert(ctx, nonExistentID)
	if err == nil {
		t.Errorf("Expected error when getting non-existent HTTP assert")
	}

	if err != ErrNoHttpAssertFound {
		t.Errorf("Expected ErrNoHttpAssertFound, got %v", err)
	}

	// Update non-existent HTTP assert
	httpAssert := &mhttpassert.HttpAssert{
		ID:          nonExistentID,
		HttpID:      idwrap.NewNow(),
		Key:         "test",
		Value:       "test",
		Enabled:     true,
		Description: "test",
		Order:       1.0,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	err = service.UpdateHttpAssert(ctx, httpAssert)
	if err == nil {
		t.Errorf("Expected error when updating non-existent HTTP assert")
	}

	// Delete non-existent HTTP assert
	err = service.DeleteHttpAssert(ctx, nonExistentID)
	if err == nil {
		t.Errorf("Expected error when deleting non-existent HTTP assert")
	}
}

func TestHttpAssertService_Serialization(t *testing.T) {
	now := time.Now().Unix()
	parentID := idwrap.NewNow()

	// Test SerializeModelToGen
	httpAssert := mhttpassert.HttpAssert{
		ID:                 idwrap.NewNow(),
		HttpID:             idwrap.NewNow(),
		Key:                "status_code",
		Value:              "200",
		Enabled:            true,
		Description:        "Status code assertion",
		Order:              1.0,
		ParentHttpAssertID: &parentID,
		IsDelta:            true,
		DeltaKey:           stringPtr("response_time"),
		DeltaValue:         stringPtr("<1000"),
		DeltaEnabled:       boolPtr(false),
		DeltaDescription:   stringPtr("Response time assertion"),
		DeltaOrder:         float32Ptr(2.0),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	genHttpAssert := SerializeModelToGen(httpAssert)

	if genHttpAssert.ID != httpAssert.ID {
		t.Errorf("Expected ID %v, got %v", httpAssert.ID, genHttpAssert.ID)
	}

	if genHttpAssert.Key != httpAssert.Key {
		t.Errorf("Expected Key %s, got %s", httpAssert.Key, genHttpAssert.Key)
	}

	if genHttpAssert.Value != httpAssert.Value {
		t.Errorf("Expected Value %s, got %s", httpAssert.Value, genHttpAssert.Value)
	}

	if genHttpAssert.Enabled != httpAssert.Enabled {
		t.Errorf("Expected Enabled %v, got %v", httpAssert.Enabled, genHttpAssert.Enabled)
	}

	if genHttpAssert.IsDelta != httpAssert.IsDelta {
		t.Errorf("Expected IsDelta %v, got %v", httpAssert.IsDelta, genHttpAssert.IsDelta)
	}

	// Test DeserializeGenToModel
	deserialized := DeserializeGenToModel(genHttpAssert)

	if deserialized.ID != httpAssert.ID {
		t.Errorf("Expected ID %v, got %v", httpAssert.ID, deserialized.ID)
	}

	if deserialized.Key != httpAssert.Key {
		t.Errorf("Expected Key %s, got %s", httpAssert.Key, deserialized.Key)
	}

	if deserialized.Value != httpAssert.Value {
		t.Errorf("Expected Value %s, got %s", httpAssert.Value, deserialized.Value)
	}

	if deserialized.Enabled != httpAssert.Enabled {
		t.Errorf("Expected Enabled %v, got %v", httpAssert.Enabled, deserialized.Enabled)
	}

	if deserialized.IsDelta != httpAssert.IsDelta {
		t.Errorf("Expected IsDelta %v, got %v", httpAssert.IsDelta, deserialized.IsDelta)
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func float32Ptr(f float32) *float32 {
	return &f
}
