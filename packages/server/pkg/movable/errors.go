package movable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"the-dev-tools/server/pkg/idwrap"
)

// =============================================================================
// BASE ERROR TYPES
// =============================================================================

// MovableError provides structured error information with context
type MovableError struct {
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Cause     error                  `json:"-"`
	Timestamp time.Time              `json:"timestamp"`
	Retryable bool                   `json:"retryable"`
}

func (e *MovableError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *MovableError) Unwrap() error {
	return e.Cause
}

func (e *MovableError) WithContext(key string, value interface{}) *MovableError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

func (e *MovableError) IsRetryable() bool {
	return e.Retryable
}

// =============================================================================
// LEGACY ERRORS (maintained for compatibility)
// =============================================================================

var (
	ErrItemNotFound        = errors.New("item not found")
	ErrTargetNotFound      = errors.New("target item not found")
	ErrInvalidPosition     = errors.New("invalid position")
	ErrCircularReference   = errors.New("circular reference detected")
	ErrIncompatibleType    = errors.New("incompatible list type")
	ErrEmptyItemID         = errors.New("item ID cannot be empty")
	ErrEmptyTargetID       = errors.New("target ID cannot be empty")
	ErrSelfReference       = errors.New("cannot move item to itself")
	ErrNoParent           = errors.New("item has no parent")
	ErrInvalidListType    = errors.New("invalid list type")
	ErrPositionOutOfRange = errors.New("position out of range")
	ErrDatabaseTransaction = errors.New("database transaction failed")
)

// =============================================================================
// CONTEXT-SPECIFIC ERROR TYPES
// =============================================================================

// ErrInvalidContext indicates an unknown or invalid context type
var ErrInvalidContext = &MovableError{
	Code:      "INVALID_CONTEXT",
	Message:   "unknown or invalid context type",
	Retryable: false,
	Timestamp: time.Now(),
}

// ErrCrossContextViolation indicates an invalid cross-context operation
var ErrCrossContextViolation = &MovableError{
	Code:      "CROSS_CONTEXT_VIOLATION",
	Message:   "invalid cross-context operation attempted",
	Retryable: false,
	Timestamp: time.Now(),
}

// NewInvalidContextError creates a context-specific invalid context error
func NewInvalidContextError(contextType string, itemID idwrap.IDWrap) *MovableError {
	return &MovableError{
		Code:    "INVALID_CONTEXT",
		Message: fmt.Sprintf("invalid context type '%s' for item", contextType),
		Context: map[string]interface{}{
			"context_type": contextType,
			"item_id":     itemID.String(),
		},
		Retryable: false,
		Timestamp: time.Now(),
	}
}

// NewCrossContextViolationError creates a cross-context violation error
func NewCrossContextViolationError(sourceContext, targetContext string, itemID idwrap.IDWrap) *MovableError {
	return &MovableError{
		Code:    "CROSS_CONTEXT_VIOLATION",
		Message: fmt.Sprintf("cannot move item from context '%s' to context '%s'", sourceContext, targetContext),
		Context: map[string]interface{}{
			"source_context": sourceContext,
			"target_context": targetContext,
			"item_id":       itemID.String(),
		},
		Retryable: false,
		Timestamp: time.Now(),
	}
}

// =============================================================================
// DELTA-SPECIFIC ERROR TYPES
// =============================================================================

// ErrDeltaConflict indicates a conflict in delta relationships
var ErrDeltaConflict = &MovableError{
	Code:      "DELTA_CONFLICT",
	Message:   "conflict detected in delta relationship",
	Retryable: true,
	Timestamp: time.Now(),
}

// ErrOrphanedDelta indicates a delta without a valid origin
var ErrOrphanedDelta = &MovableError{
	Code:      "ORPHANED_DELTA",
	Message:   "delta exists without valid origin reference",
	Retryable: false,
	Timestamp: time.Now(),
}

// ErrCircularDelta indicates a circular reference in delta chain
var ErrCircularDelta = &MovableError{
	Code:      "CIRCULAR_DELTA",
	Message:   "circular reference detected in delta chain",
	Retryable: false,
	Timestamp: time.Now(),
}

// NewDeltaConflictError creates a delta conflict error
func NewDeltaConflictError(deltaID, originID idwrap.IDWrap, conflictType string) *MovableError {
	return &MovableError{
		Code:    "DELTA_CONFLICT",
		Message: fmt.Sprintf("delta conflict: %s", conflictType),
		Context: map[string]interface{}{
			"delta_id":      deltaID.String(),
			"origin_id":     originID.String(),
			"conflict_type": conflictType,
		},
		Retryable: true,
		Timestamp: time.Now(),
	}
}

// NewOrphanedDeltaError creates an orphaned delta error
func NewOrphanedDeltaError(deltaID idwrap.IDWrap, missingOriginID idwrap.IDWrap) *MovableError {
	return &MovableError{
		Code:    "ORPHANED_DELTA",
		Message: "delta references non-existent origin",
		Context: map[string]interface{}{
			"delta_id":          deltaID.String(),
			"missing_origin_id": missingOriginID.String(),
		},
		Retryable: false,
		Timestamp: time.Now(),
	}
}

// NewCircularDeltaError creates a circular delta error
func NewCircularDeltaError(deltaChain []idwrap.IDWrap) *MovableError {
	chainStrings := make([]string, len(deltaChain))
	for i, id := range deltaChain {
		chainStrings[i] = id.String()
	}
	
	return &MovableError{
		Code:    "CIRCULAR_DELTA",
		Message: "circular reference detected in delta chain",
		Context: map[string]interface{}{
			"delta_chain": chainStrings,
			"chain_length": len(deltaChain),
		},
		Retryable: false,
		Timestamp: time.Now(),
	}
}

// =============================================================================
// SYNC-SPECIFIC ERROR TYPES
// =============================================================================

// ErrSyncTimeout indicates a synchronization operation timed out
var ErrSyncTimeout = &MovableError{
	Code:      "SYNC_TIMEOUT",
	Message:   "synchronization operation timed out",
	Retryable: true,
	Timestamp: time.Now(),
}

// NewSyncTimeoutError creates a sync timeout error
func NewSyncTimeoutError(operation string, timeout time.Duration, itemID idwrap.IDWrap) *MovableError {
	return &MovableError{
		Code:    "SYNC_TIMEOUT",
		Message: fmt.Sprintf("sync operation '%s' timed out after %v", operation, timeout),
		Context: map[string]interface{}{
			"operation": operation,
			"timeout":   timeout.String(),
			"item_id":   itemID.String(),
		},
		Retryable: true,
		Timestamp: time.Now(),
	}
}

// =============================================================================
// ERROR WRAPPING UTILITIES
// =============================================================================

// WrapDatabaseError wraps database errors with structured context
func WrapDatabaseError(err error, operation string, itemID idwrap.IDWrap) *MovableError {
	if err == nil {
		return nil
	}
	
	code := "DATABASE_ERROR"
	retryable := false
	
	// Categorize database errors
	if errors.Is(err, sql.ErrNoRows) {
		code = "ITEM_NOT_FOUND"
	} else if errors.Is(err, sql.ErrTxDone) {
		code = "TRANSACTION_DONE"
		retryable = true
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = "DATABASE_TIMEOUT"
		retryable = true
	}
	
	return &MovableError{
		Code:    code,
		Message: fmt.Sprintf("database operation '%s' failed", operation),
		Context: map[string]interface{}{
			"operation": operation,
			"item_id":   itemID.String(),
		},
		Cause:     err,
		Retryable: retryable,
		Timestamp: time.Now(),
	}
}

// WrapValidationError wraps validation errors with context
func WrapValidationError(err error, field string, value interface{}) *MovableError {
	if err == nil {
		return nil
	}
	
	return &MovableError{
		Code:    "VALIDATION_ERROR",
		Message: fmt.Sprintf("validation failed for field '%s'", field),
		Context: map[string]interface{}{
			"field": field,
			"value": value,
		},
		Cause:     err,
		Retryable: false,
		Timestamp: time.Now(),
	}
}

// =============================================================================
// RECOVERY MECHANISMS
// =============================================================================

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	BackoffRate  float64
	Timeout      time.Duration
}

// DefaultRetryConfig provides sensible defaults
var DefaultRetryConfig = RetryConfig{
	MaxAttempts:  3,
	InitialDelay: 100 * time.Millisecond,
	MaxDelay:     5 * time.Second,
	BackoffRate:  2.0,
	Timeout:      30 * time.Second,
}

// RetryWithBackoff executes a function with exponential backoff retry
func RetryWithBackoff(ctx context.Context, config RetryConfig, operation func() error) error {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 1
	}
	
	var lastErr error
	delay := config.InitialDelay
	
	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Execute operation
		if err := operation(); err == nil {
			return nil // Success
		} else {
			lastErr = err
			
			// Check if error is retryable
			if movableErr, ok := err.(*MovableError); ok && !movableErr.IsRetryable() {
				return err // Don't retry non-retryable errors
			}
		}
		
		// Don't sleep after the last attempt
		if attempt < config.MaxAttempts {
			// Apply exponential backoff
			if delay > config.MaxDelay {
				delay = config.MaxDelay
			}
			
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			
			delay = time.Duration(float64(delay) * config.BackoffRate)
		}
	}
	
	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// FallbackStrategy defines fallback behavior when operations fail
type FallbackStrategy struct {
	Name     string
	Priority int
	Execute  func(ctx context.Context, originalErr error) error
}

// ExecuteWithFallback tries the primary operation, then fallback strategies
func ExecuteWithFallback(ctx context.Context, primary func() error, fallbacks []FallbackStrategy) error {
	// Try primary operation
	if err := primary(); err == nil {
		return nil
	} else if movableErr, ok := err.(*MovableError); ok && !movableErr.IsRetryable() {
		return err // Don't try fallbacks for non-retryable errors
	} else {
		primaryErr := err
		
		// Try fallback strategies in priority order
		for _, fallback := range fallbacks {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			
			if err := fallback.Execute(ctx, primaryErr); err == nil {
				return nil // Fallback succeeded
			}
		}
		
		return fmt.Errorf("primary operation and all fallbacks failed: %w", primaryErr)
	}
}

// =============================================================================
// PARTIAL SUCCESS HANDLING
// =============================================================================

// PartialResult represents the result of an operation that may partially succeed
type PartialResult[T any] struct {
	Success   []T                        `json:"success"`
	Failed    []PartialFailure           `json:"failed"`
	Metadata  map[string]interface{}     `json:"metadata,omitempty"`
	Timestamp time.Time                  `json:"timestamp"`
}

// PartialFailure represents a failed item in a partial operation
type PartialFailure struct {
	ItemID idwrap.IDWrap `json:"item_id"`
	Error  *MovableError `json:"error"`
}

// NewPartialResult creates a new partial result
func NewPartialResult[T any]() *PartialResult[T] {
	return &PartialResult[T]{
		Success:   make([]T, 0),
		Failed:    make([]PartialFailure, 0),
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// AddSuccess adds a successful result
func (pr *PartialResult[T]) AddSuccess(item T) {
	pr.Success = append(pr.Success, item)
}

// AddFailure adds a failed result
func (pr *PartialResult[T]) AddFailure(itemID idwrap.IDWrap, err *MovableError) {
	pr.Failed = append(pr.Failed, PartialFailure{
		ItemID: itemID,
		Error:  err,
	})
}

// HasFailures returns true if there are any failures
func (pr *PartialResult[T]) HasFailures() bool {
	return len(pr.Failed) > 0
}

// SuccessCount returns the number of successful operations
func (pr *PartialResult[T]) SuccessCount() int {
	return len(pr.Success)
}

// FailureCount returns the number of failed operations
func (pr *PartialResult[T]) FailureCount() int {
	return len(pr.Failed)
}

// =============================================================================
// TRANSACTION ROLLBACK HELPERS
// =============================================================================

// TransactionRollbackHelper provides utilities for transaction management
type TransactionRollbackHelper struct {
	tx           *sql.Tx
	rollbackFunc func() error
	operations   []string
}

// NewTransactionRollbackHelper creates a new rollback helper
func NewTransactionRollbackHelper(tx *sql.Tx) *TransactionRollbackHelper {
	return &TransactionRollbackHelper{
		tx:         tx,
		operations: make([]string, 0),
	}
}

// AddOperation records an operation for rollback context
func (trh *TransactionRollbackHelper) AddOperation(operation string) {
	trh.operations = append(trh.operations, operation)
}

// SetCustomRollback sets a custom rollback function
func (trh *TransactionRollbackHelper) SetCustomRollback(rollbackFunc func() error) {
	trh.rollbackFunc = rollbackFunc
}

// Rollback performs transaction rollback with detailed error context
func (trh *TransactionRollbackHelper) Rollback(originalErr error) error {
	// Execute custom rollback if available
	if trh.rollbackFunc != nil {
		if rollbackErr := trh.rollbackFunc(); rollbackErr != nil {
			return &MovableError{
				Code:    "CUSTOM_ROLLBACK_FAILED",
				Message: "custom rollback operation failed",
				Context: map[string]interface{}{
					"operations": trh.operations,
				},
				Cause:     rollbackErr,
				Retryable: false,
				Timestamp: time.Now(),
			}
		}
	}
	
	// Standard transaction rollback
	if rollbackErr := trh.tx.Rollback(); rollbackErr != nil {
		return &MovableError{
			Code:    "TRANSACTION_ROLLBACK_FAILED",
			Message: "database transaction rollback failed",
			Context: map[string]interface{}{
				"original_error": originalErr.Error(),
				"operations":     trh.operations,
			},
			Cause:     rollbackErr,
			Retryable: false,
			Timestamp: time.Now(),
		}
	}
	
	return originalErr // Return the original error that caused the rollback
}