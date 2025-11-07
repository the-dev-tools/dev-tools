package rimportv2

import (
	"testing"
)

// TestNewValidator tests the validator constructor
func TestNewValidator(t *testing.T) {
	validator := NewValidator()
	if validator == nil {
		t.Fatal("NewValidator() returned nil")
	}
}

// TestNewHARTranslator tests the HAR translator constructor
func TestNewHARTranslator(t *testing.T) {
	translator := NewHARTranslator()
	if translator == nil {
		t.Fatal("NewHARTranslator() returned nil")
	}
}

// TestSimpleFlowGenerator tests the flow generator constructor
func TestSimpleFlowGenerator(t *testing.T) {
	generator := NewSimpleFlowGenerator()
	if generator == nil {
		t.Fatal("NewSimpleFlowGenerator() returned nil")
	}
}

// TestDefaultFlowGenerator tests the default flow generator constructor
func TestDefaultFlowGenerator(t *testing.T) {
	generator := NewDefaultFlowGenerator()
	if generator == nil {
		t.Fatal("NewDefaultFlowGenerator() returned nil")
	}
}

// TestErrorConstructors tests custom error constructors
func TestErrorConstructors(t *testing.T) {
	// Test ValidationError
	validationErr := NewValidationError("test", "value", "message")
	if validationErr == nil {
		t.Fatal("NewValidationError() returned nil")
	}
	expected := "validation failed for field 'test': message"
	if validationErr.Error() != expected {
		t.Errorf("ValidationError.Error() = %q, want %q", validationErr.Error(), expected)
	}

	// Test HARProcessingError
	harErr := NewHARProcessingError("test-step", nil)
	if harErr == nil {
		t.Fatal("NewHARProcessingError() returned nil")
	}
	if harErr.Step != "test-step" {
		t.Errorf("HARProcessingError.Step = %q, want %q", harErr.Step, "test-step")
	}

	// Test StorageError
	storageErr := NewStorageError("test-op", "test-entity", nil)
	if storageErr == nil {
		t.Fatal("NewStorageError() returned nil")
	}
	if storageErr.Operation != "test-op" {
		t.Errorf("StorageError.Operation = %q, want %q", storageErr.Operation, "test-op")
	}
	if storageErr.Entity != "test-entity" {
		t.Errorf("StorageError.Entity = %q, want %q", storageErr.Entity, "test-entity")
	}
}