package shttpbodyform

import (
	"context"
	"testing"
	"time"

	"the-dev-tools/db/pkg/dbtest"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttpbodyform"
)

func TestHttpBodyFormService_CreateGetUpdateDelete(t *testing.T) {
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
	bodyFormID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP body form
	bodyForm := &mhttpbodyform.HttpBodyForm{
		ID:          bodyFormID,
		HttpID:      httpID,
		Key:         "username",
		Value:       "john_doe",
		Enabled:     true,
		Description: "Username field",
		Order:       1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Test Create
	err = service.CreateHttpBodyForm(ctx, bodyForm)
	if err != nil {
		t.Fatalf("Failed to create HTTP body form: %v", err)
	}

	// Test Get
	retrieved, err := service.GetHttpBodyForm(ctx, bodyFormID)
	if err != nil {
		t.Fatalf("Failed to get HTTP body form: %v", err)
	}

	if retrieved.ID != bodyFormID {
		t.Errorf("Expected ID %s, got %s", bodyFormID, retrieved.ID)
	}

	if retrieved.Key != "username" {
		t.Errorf("Expected Key username, got %s", retrieved.Key)
	}

	if retrieved.Value != "john_doe" {
		t.Errorf("Expected Value john_doe, got %s", retrieved.Value)
	}

	if !retrieved.Enabled {
		t.Errorf("Expected Enabled true, got %v", retrieved.Enabled)
	}

	if retrieved.Description != "Username field" {
		t.Errorf("Expected Description Username field, got %s", retrieved.Description)
	}

	if retrieved.Order != 1.0 {
		t.Errorf("Expected Order 1.0, got %f", retrieved.Order)
	}

	// Test Update
	bodyForm.Key = "email"
	bodyForm.Value = "john@example.com"
	bodyForm.Enabled = false
	bodyForm.Description = "Email field"
	err = service.UpdateHttpBodyForm(ctx, bodyForm)
	if err != nil {
		t.Fatalf("Failed to update HTTP body form: %v", err)
	}

	// Verify update
	updated, err := service.GetHttpBodyForm(ctx, bodyFormID)
	if err != nil {
		t.Fatalf("Failed to get updated HTTP body form: %v", err)
	}

	if updated.Key != "email" {
		t.Errorf("Expected Key email, got %s", updated.Key)
	}

	if updated.Value != "john@example.com" {
		t.Errorf("Expected Value john@example.com, got %s", updated.Value)
	}

	if updated.Enabled {
		t.Errorf("Expected Enabled false, got %v", updated.Enabled)
	}

	if updated.Description != "Email field" {
		t.Errorf("Expected Description Email field, got %s", updated.Description)
	}

	// Test Delete
	err = service.DeleteHttpBodyForm(ctx, bodyFormID)
	if err != nil {
		t.Fatalf("Failed to delete HTTP body form: %v", err)
	}

	// Verify deletion
	_, err = service.GetHttpBodyForm(ctx, bodyFormID)
	if err == nil {
		t.Errorf("Expected error when getting deleted HTTP body form, got nil")
	}

	if err != ErrNoHttpBodyFormFound {
		t.Errorf("Expected ErrNoHttpBodyFormFound, got %v", err)
	}
}

func TestHttpBodyFormService_GetByHttpID(t *testing.T) {
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

	// Create multiple test HTTP body forms
	bodyForms := []*mhttpbodyform.HttpBodyForm{
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         "username",
			Value:       "john_doe",
			Enabled:     true,
			Description: "Username field",
			Order:       1.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         "password",
			Value:       "secret123",
			Enabled:     true,
			Description: "Password field",
			Order:       2.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// Create body forms
	for _, bf := range bodyForms {
		err := service.CreateHttpBodyForm(ctx, bf)
		if err != nil {
			t.Fatalf("Failed to create HTTP body form: %v", err)
		}
	}

	// Test GetByHttpID
	retrieved, err := service.GetHttpBodyFormsByHttpID(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get HTTP body forms by HttpID: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 HTTP body forms, got %d", len(retrieved))
	}

	// Test with non-existent HttpID
	nonExistentID := idwrap.NewNow()
	retrieved, err = service.GetHttpBodyFormsByHttpID(ctx, nonExistentID)
	if err != nil {
		t.Errorf("Expected no error for non-existent HttpID, got %v", err)
	}

	if len(retrieved) != 0 {
		t.Errorf("Expected 0 HTTP body forms for non-existent HttpID, got %d", len(retrieved))
	}
}

func TestHttpBodyFormService_GetByHttpIDs(t *testing.T) {
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

	// Create test HTTP body forms for different HTTP IDs
	bodyForms := []*mhttpbodyform.HttpBodyForm{
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID1,
			Key:         "username",
			Value:       "john_doe",
			Enabled:     true,
			Description: "Username field",
			Order:       1.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID1,
			Key:         "password",
			Value:       "secret123",
			Enabled:     true,
			Description: "Password field",
			Order:       2.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:          idwrap.NewNow(),
			HttpID:      httpID2,
			Key:         "email",
			Value:       "test@example.com",
			Enabled:     true,
			Description: "Email field",
			Order:       1.0,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	// Create body forms
	for _, bf := range bodyForms {
		err := service.CreateHttpBodyForm(ctx, bf)
		if err != nil {
			t.Fatalf("Failed to create HTTP body form: %v", err)
		}
	}

	// Test GetHttpBodyFormsByHttpIDs - this method gets body forms by their IDs, not HTTP IDs
	// So we need to collect the body form IDs and test that functionality
	var bodyFormIDs []idwrap.IDWrap
	for _, bf := range bodyForms {
		bodyFormIDs = append(bodyFormIDs, bf.ID)
	}

	// Test GetHttpBodyFormsByHttpIDs with body form IDs
	result, err := service.GetHttpBodyFormsByHttpIDs(ctx, bodyFormIDs)
	if err != nil {
		t.Fatalf("Failed to get HTTP body forms by HttpIDs: %v", err)
	}

	// The result should be grouped by HttpID, not by body form ID
	if len(result) != 2 {
		t.Errorf("Expected 2 HttpIDs in result, got %d", len(result))
	}

	if len(result[httpID1]) != 2 {
		t.Errorf("Expected 2 HTTP body forms for HttpID1, got %d", len(result[httpID1]))
	}

	if len(result[httpID2]) != 1 {
		t.Errorf("Expected 1 HTTP body form for HttpID2, got %d", len(result[httpID2]))
	}

	// Test with empty HttpIDs
	emptyResult, err := service.GetHttpBodyFormsByHttpIDs(ctx, []idwrap.IDWrap{})
	if err != nil {
		t.Errorf("Expected no error for empty HttpIDs, got %v", err)
	}

	if len(emptyResult) != 0 {
		t.Errorf("Expected empty result for empty HttpIDs, got %d", len(emptyResult))
	}
}

func TestHttpBodyFormService_BulkOperations(t *testing.T) {
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

	// Create multiple test HTTP body forms for bulk operations
	bodyForms := make([]mhttpbodyform.HttpBodyForm, 15)
	for i := 0; i < 15; i++ {
		bodyForms[i] = mhttpbodyform.HttpBodyForm{
			ID:          idwrap.NewNow(),
			HttpID:      httpID,
			Key:         "field_" + string(rune('a'+i)),
			Value:       "value_" + string(rune('a'+i)),
			Enabled:     true,
			Description: "Field " + string(rune('a'+i)),
			Order:       float32(i + 1),
			CreatedAt:   now,
			UpdatedAt:   now,
		}
	}

	// Test CreateBulk
	err = service.CreateBulkHttpBodyForm(ctx, bodyForms)
	if err != nil {
		t.Fatalf("Failed to create bulk HTTP body forms: %v", err)
	}

	// Verify bulk creation
	retrieved, err := service.GetHttpBodyFormsByHttpID(ctx, httpID)
	if err != nil {
		t.Fatalf("Failed to get HTTP body forms by HttpID: %v", err)
	}

	if len(retrieved) != 15 {
		t.Errorf("Expected 15 HTTP body forms, got %d", len(retrieved))
	}
}

func TestHttpBodyFormService_DeltaOperations(t *testing.T) {
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
	bodyFormID := idwrap.NewNow()
	parentID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP body form with delta
	bodyForm := &mhttpbodyform.HttpBodyForm{
		ID:                   bodyFormID,
		HttpID:               httpID,
		Key:                  "username",
		Value:                "john_doe",
		Enabled:              true,
		Description:          "Username field",
		Order:                1.0,
		ParentHttpBodyFormID: &parentID,
		IsDelta:              true,
		DeltaKey:             stringPtr("email"),
		DeltaValue:           stringPtr("john@example.com"),
		DeltaEnabled:         boolPtr(false),
		DeltaDescription:     stringPtr("Email field"),
		DeltaOrder:           float32Ptr(2.0),
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	// Test Create
	err = service.CreateHttpBodyForm(ctx, bodyForm)
	if err != nil {
		t.Fatalf("Failed to create HTTP body form with delta: %v", err)
	}

	// Test UpdateDelta
	newDeltaKey := stringPtr("phone")
	newDeltaValue := stringPtr("123-456-7890")
	newDeltaEnabled := boolPtr(true)
	newDeltaDescription := stringPtr("Phone field")
	newDeltaOrder := float32Ptr(3.0)

	err = service.UpdateHttpBodyFormDelta(ctx, bodyFormID, newDeltaKey, newDeltaValue, newDeltaEnabled, newDeltaDescription, newDeltaOrder)
	if err != nil {
		t.Fatalf("Failed to update HTTP body form delta: %v", err)
	}

	// Verify delta update
	retrieved, err := service.GetHttpBodyForm(ctx, bodyFormID)
	if err != nil {
		t.Fatalf("Failed to get updated HTTP body form: %v", err)
	}

	if *retrieved.DeltaKey != "phone" {
		t.Errorf("Expected DeltaKey phone, got %s", *retrieved.DeltaKey)
	}

	if *retrieved.DeltaValue != "123-456-7890" {
		t.Errorf("Expected DeltaValue 123-456-7890, got %s", *retrieved.DeltaValue)
	}

	if !*retrieved.DeltaEnabled {
		t.Errorf("Expected DeltaEnabled true, got %v", *retrieved.DeltaEnabled)
	}

	if *retrieved.DeltaDescription != "Phone field" {
		t.Errorf("Expected DeltaDescription Phone field, got %s", *retrieved.DeltaDescription)
	}

	// Test ResetDelta
	err = service.ResetHttpBodyFormDelta(ctx, bodyFormID)
	if err != nil {
		t.Fatalf("Failed to reset HTTP body form delta: %v", err)
	}

	// Verify delta reset
	retrieved, err = service.GetHttpBodyForm(ctx, bodyFormID)
	if err != nil {
		t.Fatalf("Failed to get HTTP body form after delta reset: %v", err)
	}

	if retrieved.IsDelta {
		t.Errorf("Expected IsDelta false after reset, got %v", retrieved.IsDelta)
	}

	if retrieved.DeltaKey != nil {
		t.Errorf("Expected DeltaKey nil after reset, got %v", retrieved.DeltaKey)
	}
}

func TestHttpBodyFormService_OrderOperations(t *testing.T) {
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
	bodyFormID := idwrap.NewNow()
	now := time.Now().Unix()

	// Create test HTTP body form
	bodyForm := &mhttpbodyform.HttpBodyForm{
		ID:          bodyFormID,
		HttpID:      httpID,
		Key:         "username",
		Value:       "john_doe",
		Enabled:     true,
		Description: "Username field",
		Order:       1.0,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Create
	err = service.CreateHttpBodyForm(ctx, bodyForm)
	if err != nil {
		t.Fatalf("Failed to create HTTP body form: %v", err)
	}

	// Test UpdateOrder
	newOrder := float32(5.0)
	err = service.UpdateHttpBodyFormOrder(ctx, bodyFormID, httpID, newOrder)
	if err != nil {
		t.Fatalf("Failed to update HTTP body form order: %v", err)
	}

	// Verify order update
	retrieved, err := service.GetHttpBodyForm(ctx, bodyFormID)
	if err != nil {
		t.Fatalf("Failed to get HTTP body form after order update: %v", err)
	}

	if retrieved.Order != 5.0 {
		t.Errorf("Expected Order 5.0, got %f", retrieved.Order)
	}
}

func TestHttpBodyFormService_TransactionSupport(t *testing.T) {
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

func TestHttpBodyFormService_ErrorHandling(t *testing.T) {
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

	// Get non-existent HTTP body form
	_, err = service.GetHttpBodyForm(ctx, nonExistentID)
	if err == nil {
		t.Errorf("Expected error when getting non-existent HTTP body form")
	}

	if err != ErrNoHttpBodyFormFound {
		t.Errorf("Expected ErrNoHttpBodyFormFound, got %v", err)
	}

	// Update non-existent HTTP body form
	// Note: UPDATE operations in SQL typically succeed even if no rows are affected
	bodyForm := &mhttpbodyform.HttpBodyForm{
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

	err = service.UpdateHttpBodyForm(ctx, bodyForm)
	if err != nil {
		t.Errorf("Unexpected error when updating non-existent HTTP body form: %v", err)
	}

	// Delete non-existent HTTP body form
	// Note: DELETE operations in SQL typically succeed even if no rows are affected
	err = service.DeleteHttpBodyForm(ctx, nonExistentID)
	if err != nil {
		t.Errorf("Unexpected error when deleting non-existent HTTP body form: %v", err)
	}
}

func TestHttpBodyFormService_Serialization(t *testing.T) {
	now := time.Now().Unix()
	parentID := idwrap.NewNow()

	// Test SerializeModelToGen
	bodyForm := mhttpbodyform.HttpBodyForm{
		ID:                   idwrap.NewNow(),
		HttpID:               idwrap.NewNow(),
		Key:                  "username",
		Value:                "john_doe",
		Enabled:              true,
		Description:          "Username field",
		Order:                1.0,
		ParentHttpBodyFormID: &parentID,
		IsDelta:              true,
		DeltaKey:             stringPtr("email"),
		DeltaValue:           stringPtr("john@example.com"),
		DeltaEnabled:         boolPtr(false),
		DeltaDescription:     stringPtr("Email field"),
		DeltaOrder:           float32Ptr(2.0),
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	genBodyForm := SerializeModelToGen(bodyForm)

	if genBodyForm.ID != bodyForm.ID {
		t.Errorf("Expected ID %v, got %v", bodyForm.ID, genBodyForm.ID)
	}

	if genBodyForm.Key != bodyForm.Key {
		t.Errorf("Expected Key %s, got %s", bodyForm.Key, genBodyForm.Key)
	}

	if genBodyForm.Value != bodyForm.Value {
		t.Errorf("Expected Value %s, got %s", bodyForm.Value, genBodyForm.Value)
	}

	if genBodyForm.Enabled != bodyForm.Enabled {
		t.Errorf("Expected Enabled %v, got %v", bodyForm.Enabled, genBodyForm.Enabled)
	}

	if genBodyForm.IsDelta != bodyForm.IsDelta {
		t.Errorf("Expected IsDelta %v, got %v", bodyForm.IsDelta, genBodyForm.IsDelta)
	}

	// Test DeserializeGenToModel
	deserialized := DeserializeGenToModel(genBodyForm)

	if deserialized.ID != bodyForm.ID {
		t.Errorf("Expected ID %v, got %v", bodyForm.ID, deserialized.ID)
	}

	if deserialized.Key != bodyForm.Key {
		t.Errorf("Expected Key %s, got %s", bodyForm.Key, deserialized.Key)
	}

	if deserialized.Value != bodyForm.Value {
		t.Errorf("Expected Value %s, got %s", bodyForm.Value, deserialized.Value)
	}

	if deserialized.Enabled != bodyForm.Enabled {
		t.Errorf("Expected Enabled %v, got %v", bodyForm.Enabled, deserialized.Enabled)
	}

	if deserialized.IsDelta != bodyForm.IsDelta {
		t.Errorf("Expected IsDelta %v, got %v", bodyForm.IsDelta, deserialized.IsDelta)
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
