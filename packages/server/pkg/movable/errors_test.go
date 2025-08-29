package movable

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// MOVABLE ERROR TESTS
// =============================================================================

func TestMovableError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *MovableError
		expected string
	}{
		{
			name: "error without cause",
			err: &MovableError{
				Code:    "TEST_ERROR",
				Message: "test error message",
			},
			expected: "test error message",
		},
		{
			name: "error with cause",
			err: &MovableError{
				Code:    "TEST_ERROR",
				Message: "test error message",
				Cause:   errors.New("underlying cause"),
			},
			expected: "test error message: underlying cause",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("MovableError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMovableError_Unwrap(t *testing.T) {
	cause := errors.New("underlying cause")
	err := &MovableError{
		Code:  "TEST_ERROR",
		Cause: cause,
	}

	if unwrapped := err.Unwrap(); unwrapped != cause {
		t.Errorf("MovableError.Unwrap() = %v, want %v", unwrapped, cause)
	}
}

func TestMovableError_WithContext(t *testing.T) {
	err := &MovableError{
		Code:    "TEST_ERROR",
		Message: "test message",
	}

	err.WithContext("key1", "value1")
	err.WithContext("key2", 42)

	if err.Context["key1"] != "value1" {
		t.Errorf("Expected context key1 = 'value1', got %v", err.Context["key1"])
	}
	if err.Context["key2"] != 42 {
		t.Errorf("Expected context key2 = 42, got %v", err.Context["key2"])
	}
}

func TestMovableError_IsRetryable(t *testing.T) {
	retryableErr := &MovableError{Retryable: true}
	nonRetryableErr := &MovableError{Retryable: false}

	if !retryableErr.IsRetryable() {
		t.Error("Expected retryable error to return true")
	}
	if nonRetryableErr.IsRetryable() {
		t.Error("Expected non-retryable error to return false")
	}
}

// =============================================================================
// CONTEXT-SPECIFIC ERROR TESTS
// =============================================================================

func TestNewInvalidContextError(t *testing.T) {
	itemID := idwrap.NewNow()
	err := NewInvalidContextError("unknown_context", itemID)

	if err.Code != "INVALID_CONTEXT" {
		t.Errorf("Expected code INVALID_CONTEXT, got %s", err.Code)
	}
	if !strings.Contains(err.Message, "unknown_context") {
		t.Errorf("Expected message to contain context type, got %s", err.Message)
	}
	if err.Context["context_type"] != "unknown_context" {
		t.Errorf("Expected context_type in context, got %v", err.Context["context_type"])
	}
	if err.Context["item_id"] != itemID.String() {
		t.Errorf("Expected item_id in context, got %v", err.Context["item_id"])
	}
	if err.Retryable {
		t.Error("Expected invalid context error to be non-retryable")
	}
}

func TestNewCrossContextViolationError(t *testing.T) {
	itemID := idwrap.NewNow()
	err := NewCrossContextViolationError("source", "target", itemID)

	if err.Code != "CROSS_CONTEXT_VIOLATION" {
		t.Errorf("Expected code CROSS_CONTEXT_VIOLATION, got %s", err.Code)
	}
	if !strings.Contains(err.Message, "source") || !strings.Contains(err.Message, "target") {
		t.Errorf("Expected message to contain contexts, got %s", err.Message)
	}
	if err.Context["source_context"] != "source" {
		t.Errorf("Expected source_context in context, got %v", err.Context["source_context"])
	}
	if err.Context["target_context"] != "target" {
		t.Errorf("Expected target_context in context, got %v", err.Context["target_context"])
	}
	if err.Retryable {
		t.Error("Expected cross context violation to be non-retryable")
	}
}

// =============================================================================
// DELTA-SPECIFIC ERROR TESTS
// =============================================================================

func TestNewDeltaConflictError(t *testing.T) {
	deltaID := idwrap.NewNow()
	originID := idwrap.NewNow()
	err := NewDeltaConflictError(deltaID, originID, "version_mismatch")

	if err.Code != "DELTA_CONFLICT" {
		t.Errorf("Expected code DELTA_CONFLICT, got %s", err.Code)
	}
	if !strings.Contains(err.Message, "version_mismatch") {
		t.Errorf("Expected message to contain conflict type, got %s", err.Message)
	}
	if err.Context["delta_id"] != deltaID.String() {
		t.Errorf("Expected delta_id in context, got %v", err.Context["delta_id"])
	}
	if err.Context["origin_id"] != originID.String() {
		t.Errorf("Expected origin_id in context, got %v", err.Context["origin_id"])
	}
	if err.Context["conflict_type"] != "version_mismatch" {
		t.Errorf("Expected conflict_type in context, got %v", err.Context["conflict_type"])
	}
	if !err.Retryable {
		t.Error("Expected delta conflict to be retryable")
	}
}

func TestNewOrphanedDeltaError(t *testing.T) {
	deltaID := idwrap.NewNow()
	missingOriginID := idwrap.NewNow()
	err := NewOrphanedDeltaError(deltaID, missingOriginID)

	if err.Code != "ORPHANED_DELTA" {
		t.Errorf("Expected code ORPHANED_DELTA, got %s", err.Code)
	}
	if err.Context["delta_id"] != deltaID.String() {
		t.Errorf("Expected delta_id in context, got %v", err.Context["delta_id"])
	}
	if err.Context["missing_origin_id"] != missingOriginID.String() {
		t.Errorf("Expected missing_origin_id in context, got %v", err.Context["missing_origin_id"])
	}
	if err.Retryable {
		t.Error("Expected orphaned delta to be non-retryable")
	}
}

func TestNewCircularDeltaError(t *testing.T) {
	deltaChain := []idwrap.IDWrap{
		idwrap.NewNow(),
		idwrap.NewNow(),
		idwrap.NewNow(),
		idwrap.NewNow(), // Circular reference
	}
	err := NewCircularDeltaError(deltaChain)

	if err.Code != "CIRCULAR_DELTA" {
		t.Errorf("Expected code CIRCULAR_DELTA, got %s", err.Code)
	}
	
	chainFromContext, ok := err.Context["delta_chain"].([]string)
	if !ok {
		t.Errorf("Expected delta_chain in context as []string, got %T", err.Context["delta_chain"])
	}
	if len(chainFromContext) != len(deltaChain) {
		t.Errorf("Expected delta chain length %d, got %d", len(deltaChain), len(chainFromContext))
	}
	
	chainLength, ok := err.Context["chain_length"].(int)
	if !ok || chainLength != len(deltaChain) {
		t.Errorf("Expected chain_length %d, got %v", len(deltaChain), err.Context["chain_length"])
	}
	
	if err.Retryable {
		t.Error("Expected circular delta to be non-retryable")
	}
}

// =============================================================================
// SYNC-SPECIFIC ERROR TESTS
// =============================================================================

func TestNewSyncTimeoutError(t *testing.T) {
	itemID := idwrap.NewNow()
	timeout := 5 * time.Second
	err := NewSyncTimeoutError("test_operation", timeout, itemID)

	if err.Code != "SYNC_TIMEOUT" {
		t.Errorf("Expected code SYNC_TIMEOUT, got %s", err.Code)
	}
	if !strings.Contains(err.Message, "test_operation") {
		t.Errorf("Expected message to contain operation name, got %s", err.Message)
	}
	if !strings.Contains(err.Message, timeout.String()) {
		t.Errorf("Expected message to contain timeout, got %s", err.Message)
	}
	if err.Context["operation"] != "test_operation" {
		t.Errorf("Expected operation in context, got %v", err.Context["operation"])
	}
	if err.Context["timeout"] != timeout.String() {
		t.Errorf("Expected timeout in context, got %v", err.Context["timeout"])
	}
	if err.Context["item_id"] != itemID.String() {
		t.Errorf("Expected item_id in context, got %v", err.Context["item_id"])
	}
	if !err.Retryable {
		t.Error("Expected sync timeout to be retryable")
	}
}

// =============================================================================
// ERROR WRAPPING TESTS
// =============================================================================

func TestWrapDatabaseError(t *testing.T) {
	itemID := idwrap.NewNow()

	tests := []struct {
		name           string
		err            error
		expectedCode   string
		expectedRetryable bool
	}{
		{
			name:           "sql.ErrNoRows",
			err:            sql.ErrNoRows,
			expectedCode:   "ITEM_NOT_FOUND",
			expectedRetryable: false,
		},
		{
			name:           "sql.ErrTxDone",
			err:            sql.ErrTxDone,
			expectedCode:   "TRANSACTION_DONE",
			expectedRetryable: true,
		},
		{
			name:           "context.DeadlineExceeded",
			err:            context.DeadlineExceeded,
			expectedCode:   "DATABASE_TIMEOUT",
			expectedRetryable: true,
		},
		{
			name:           "generic error",
			err:            errors.New("generic database error"),
			expectedCode:   "DATABASE_ERROR",
			expectedRetryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := WrapDatabaseError(tt.err, "test_operation", itemID)
			
			if wrapped == nil {
				t.Fatal("Expected wrapped error, got nil")
			}
			if wrapped.Code != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, wrapped.Code)
			}
			if wrapped.Retryable != tt.expectedRetryable {
				t.Errorf("Expected retryable %v, got %v", tt.expectedRetryable, wrapped.Retryable)
			}
			if wrapped.Cause != tt.err {
				t.Errorf("Expected cause to be original error, got %v", wrapped.Cause)
			}
			if wrapped.Context["operation"] != "test_operation" {
				t.Errorf("Expected operation in context, got %v", wrapped.Context["operation"])
			}
			if wrapped.Context["item_id"] != itemID.String() {
				t.Errorf("Expected item_id in context, got %v", wrapped.Context["item_id"])
			}
		})
	}
}

func TestWrapDatabaseError_NilInput(t *testing.T) {
	itemID := idwrap.NewNow()
	wrapped := WrapDatabaseError(nil, "test_operation", itemID)
	if wrapped != nil {
		t.Errorf("Expected nil for nil input, got %v", wrapped)
	}
}

func TestWrapValidationError(t *testing.T) {
	originalErr := errors.New("field validation failed")
	wrapped := WrapValidationError(originalErr, "email", "invalid@")

	if wrapped == nil {
		t.Fatal("Expected wrapped error, got nil")
	}
	if wrapped.Code != "VALIDATION_ERROR" {
		t.Errorf("Expected code VALIDATION_ERROR, got %s", wrapped.Code)
	}
	if !strings.Contains(wrapped.Message, "email") {
		t.Errorf("Expected message to contain field name, got %s", wrapped.Message)
	}
	if wrapped.Context["field"] != "email" {
		t.Errorf("Expected field in context, got %v", wrapped.Context["field"])
	}
	if wrapped.Context["value"] != "invalid@" {
		t.Errorf("Expected value in context, got %v", wrapped.Context["value"])
	}
	if wrapped.Cause != originalErr {
		t.Errorf("Expected cause to be original error, got %v", wrapped.Cause)
	}
	if wrapped.Retryable {
		t.Error("Expected validation error to be non-retryable")
	}
}

func TestWrapValidationError_NilInput(t *testing.T) {
	wrapped := WrapValidationError(nil, "field", "value")
	if wrapped != nil {
		t.Errorf("Expected nil for nil input, got %v", wrapped)
	}
}

// =============================================================================
// RETRY MECHANISM TESTS
// =============================================================================

func TestRetryWithBackoff_Success(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		BackoffRate:  2.0,
	}

	attemptCount := 0
	operation := func() error {
		attemptCount++
		if attemptCount < 2 {
			return &MovableError{
				Code:      "TEMPORARY_ERROR",
				Retryable: true,
			}
		}
		return nil // Success on second attempt
	}

	err := RetryWithBackoff(ctx, config, operation)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}
}

func TestRetryWithBackoff_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	config := DefaultRetryConfig

	attemptCount := 0
	operation := func() error {
		attemptCount++
		return &MovableError{
			Code:      "NON_RETRYABLE_ERROR",
			Retryable: false,
		}
	}

	err := RetryWithBackoff(ctx, config, operation)
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt for non-retryable error, got %d", attemptCount)
	}
}

func TestRetryWithBackoff_MaxAttemptsExceeded(t *testing.T) {
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:  2,
		InitialDelay: 1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		BackoffRate:  2.0,
	}

	attemptCount := 0
	operation := func() error {
		attemptCount++
		return &MovableError{
			Code:      "PERSISTENT_ERROR",
			Retryable: true,
		}
	}

	err := RetryWithBackoff(ctx, config, operation)
	if err == nil {
		t.Error("Expected error after max attempts, got nil")
	}
	if attemptCount != 2 {
		t.Errorf("Expected 2 attempts, got %d", attemptCount)
	}
	if !strings.Contains(err.Error(), "operation failed after 2 attempts") {
		t.Errorf("Expected max attempts message, got: %s", err.Error())
	}
}

func TestRetryWithBackoff_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		BackoffRate:  2.0,
	}

	operation := func() error {
		return &MovableError{
			Code:      "RETRYABLE_ERROR",
			Retryable: true,
		}
	}

	// Cancel context after a short delay
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := RetryWithBackoff(ctx, config, operation)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

// =============================================================================
// FALLBACK STRATEGY TESTS
// =============================================================================

func TestExecuteWithFallback_PrimarySuccess(t *testing.T) {
	ctx := context.Background()
	
	primary := func() error {
		return nil // Success
	}
	
	fallbacks := []FallbackStrategy{
		{
			Name:     "fallback1",
			Priority: 1,
			Execute: func(ctx context.Context, originalErr error) error {
				t.Error("Fallback should not be executed when primary succeeds")
				return nil
			},
		},
	}

	err := ExecuteWithFallback(ctx, primary, fallbacks)
	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
}

func TestExecuteWithFallback_FallbackSuccess(t *testing.T) {
	ctx := context.Background()
	
	primaryErr := &MovableError{
		Code:      "PRIMARY_ERROR",
		Retryable: true,
	}
	
	primary := func() error {
		return primaryErr
	}
	
	fallbackExecuted := false
	fallbacks := []FallbackStrategy{
		{
			Name:     "fallback1",
			Priority: 1,
			Execute: func(ctx context.Context, originalErr error) error {
				fallbackExecuted = true
				if originalErr != primaryErr {
					t.Errorf("Expected original error in fallback, got: %v", originalErr)
				}
				return nil // Success
			},
		},
	}

	err := ExecuteWithFallback(ctx, primary, fallbacks)
	if err != nil {
		t.Errorf("Expected success from fallback, got error: %v", err)
	}
	if !fallbackExecuted {
		t.Error("Expected fallback to be executed")
	}
}

func TestExecuteWithFallback_NonRetryableError(t *testing.T) {
	ctx := context.Background()
	
	primaryErr := &MovableError{
		Code:      "NON_RETRYABLE_ERROR",
		Retryable: false,
	}
	
	primary := func() error {
		return primaryErr
	}
	
	fallbacks := []FallbackStrategy{
		{
			Name:     "fallback1",
			Priority: 1,
			Execute: func(ctx context.Context, originalErr error) error {
				t.Error("Fallback should not be executed for non-retryable error")
				return nil
			},
		},
	}

	err := ExecuteWithFallback(ctx, primary, fallbacks)
	if err != primaryErr {
		t.Errorf("Expected primary error to be returned, got: %v", err)
	}
}

// =============================================================================
// PARTIAL RESULT TESTS
// =============================================================================

func TestPartialResult(t *testing.T) {
	result := NewPartialResult[string]()

	// Test initial state
	if result.SuccessCount() != 0 {
		t.Errorf("Expected 0 success count, got %d", result.SuccessCount())
	}
	if result.FailureCount() != 0 {
		t.Errorf("Expected 0 failure count, got %d", result.FailureCount())
	}
	if result.HasFailures() {
		t.Error("Expected no failures initially")
	}

	// Add success
	result.AddSuccess("item1")
	result.AddSuccess("item2")

	if result.SuccessCount() != 2 {
		t.Errorf("Expected 2 success count, got %d", result.SuccessCount())
	}

	// Add failure
	itemID := idwrap.NewNow()
	failureErr := &MovableError{
		Code:    "TEST_ERROR",
		Message: "test failure",
	}
	result.AddFailure(itemID, failureErr)

	if result.FailureCount() != 1 {
		t.Errorf("Expected 1 failure count, got %d", result.FailureCount())
	}
	if !result.HasFailures() {
		t.Error("Expected failures to be present")
	}

	// Check failure details
	if len(result.Failed) != 1 {
		t.Fatalf("Expected 1 failed item, got %d", len(result.Failed))
	}
	if result.Failed[0].ItemID != itemID {
		t.Errorf("Expected failed item ID %s, got %s", itemID.String(), result.Failed[0].ItemID.String())
	}
	if result.Failed[0].Error != failureErr {
		t.Errorf("Expected failed item error %v, got %v", failureErr, result.Failed[0].Error)
	}
}

// =============================================================================
// TRANSACTION ROLLBACK HELPER TESTS
// =============================================================================

func TestTransactionRollbackHelper(t *testing.T) {
	// Note: We can't easily test with a real sql.Tx in unit tests
	// In practice, this would be tested in integration tests
	
	helper := NewTransactionRollbackHelper(nil)
	
	// Test operation tracking
	helper.AddOperation("insert_item")
	helper.AddOperation("update_position")
	
	if len(helper.operations) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(helper.operations))
	}
	if helper.operations[0] != "insert_item" {
		t.Errorf("Expected first operation 'insert_item', got '%s'", helper.operations[0])
	}
	if helper.operations[1] != "update_position" {
		t.Errorf("Expected second operation 'update_position', got '%s'", helper.operations[1])
	}

	// Test custom rollback function
	customRollbackCalled := false
	helper.SetCustomRollback(func() error {
		customRollbackCalled = true
		return nil
	})

	if helper.rollbackFunc == nil {
		t.Error("Expected custom rollback function to be set")
	}
	
	// We can't test the actual Rollback method without a real transaction,
	// but we can verify the setup is correct
	if !customRollbackCalled {
		// The custom rollback hasn't been called yet, which is correct
		// It would only be called during actual rollback
	}
}