package rexportv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"the-dev-tools/server/pkg/idwrap"
)

// TestNewValidator tests the validator constructor
func TestNewValidator(t *testing.T) {
	validator := NewValidator(nil)
	assert.NotNil(t, validator)
}

// TestSimpleValidator_ValidateExportRequest tests export request validation
func TestSimpleValidator_ValidateExportRequest(t *testing.T) {
	validator := NewValidator(nil)

	tests := []struct {
		name        string
		req         *ExportRequest
		expectError bool
		errorField  string
	}{
		{
			name: "valid request",
			req: &ExportRequest{
				WorkspaceID: idwrap.NewNow(),
				FileIDs:     []idwrap.IDWrap{idwrap.NewNow()},
				Format:      ExportFormat_YAML,
				Simplified:  false,
			},
			expectError: false,
		},
		{
			name:        "nil request",
			req:         nil,
			expectError: true,
			errorField:  "request",
		},
		{
			name: "empty workspace ID",
			req: &ExportRequest{
				WorkspaceID: idwrap.IDWrap{},
				Format:     ExportFormat_YAML,
			},
			expectError: true,
			errorField:  "workspaceId",
		},
		{
			name: "unsupported format",
			req: &ExportRequest{
				WorkspaceID: idwrap.NewNow(),
				Format:     ExportFormat("UNSUPPORTED"),
			},
			expectError: true,
			errorField:  "format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateExportRequest(context.Background(), tt.req)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorField != "" {
					assert.Contains(t, err.Error(), tt.errorField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidationError_Creation tests validation error creation
func TestValidationError_Creation(t *testing.T) {
	field := "testField"
	message := "test message"

	err := NewValidationError(field, message)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), field)
	assert.Contains(t, err.Error(), message)
}