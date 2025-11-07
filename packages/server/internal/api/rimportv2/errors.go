package rimportv2

import (
	"errors"
	"fmt"
)

// Common errors for the rimportv2 service
var (
	ErrInvalidHARFormat  = errors.New("invalid HAR format")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrStorageFailed     = errors.New("storage operation failed")
	ErrWorkspaceNotFound = errors.New("workspace not found")
)

// ValidationError represents an input validation error
type ValidationError struct {
	Field   string
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

// NewValidationError creates a new validation error
func NewValidationError(field, message string) error {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrorWithCause creates a new validation error with an underlying cause
func NewValidationErrorWithCause(field string, cause error) error {
	return &ValidationError{
		Field: field,
		Err:   cause,
	}
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}
