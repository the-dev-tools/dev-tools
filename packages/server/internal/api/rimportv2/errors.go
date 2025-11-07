package rimportv2

import (
	"errors"
	"fmt"
)

// Common errors for the rimportv2 service
var (
	ErrInvalidHARFormat    = errors.New("invalid HAR format")
	ErrWorkspaceNotFound   = errors.New("workspace not found")
	ErrPermissionDenied    = errors.New("permission denied")
	ErrStorageFailed       = errors.New("storage operation failed")
	ErrFlowGenerationFailed = errors.New("flow generation failed")
	ErrDomainProcessingFailed = errors.New("domain processing failed")
)

// ValidationError represents an input validation error
type ValidationError struct {
	Field   string
	Value   string
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Errorf("validation failed for field '%s': %w", e.Field, e.Err).Error()
	}
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// HARProcessingError represents an error during HAR file processing
type HARProcessingError struct {
	Step  string
	Cause error
}

func (e *HARProcessingError) Error() string {
	return fmt.Errorf("HAR processing failed at step '%s': %w", e.Step, e.Cause).Error()
}

func (e *HARProcessingError) Unwrap() error {
	return e.Cause
}

// StorageError represents an error during database storage operations
type StorageError struct {
	Operation string
	Entity    string
	Cause     error
}

func (e *StorageError) Error() string {
	return fmt.Errorf("storage operation '%s' failed for entity '%s': %w", e.Operation, e.Entity, e.Cause).Error()
}

func (e *StorageError) Unwrap() error {
	return e.Cause
}

// NewValidationError creates a new validation error
func NewValidationError(field, value, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewValidationErrorWithCause creates a new validation error with an underlying cause
func NewValidationErrorWithCause(field, value string, cause error) *ValidationError {
	return &ValidationError{
		Field: field,
		Value: value,
		Err:   cause,
	}
}

// NewHARProcessingError creates a new HAR processing error
func NewHARProcessingError(step string, cause error) *HARProcessingError {
	return &HARProcessingError{
		Step:  step,
		Cause: cause,
	}
}

// NewStorageError creates a new storage error
func NewStorageError(operation, entity string, cause error) *StorageError {
	return &StorageError{
		Operation: operation,
		Entity:    entity,
		Cause:     cause,
	}
}