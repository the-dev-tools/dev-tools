package rimportv2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"the-dev-tools/server/pkg/idwrap"
)

// DefaultValidator implements the Validator interface
type DefaultValidator struct{}

// NewValidator creates a new DefaultValidator
func NewValidator() *DefaultValidator {
	return &DefaultValidator{}
}

// ValidateImportRequest validates the incoming import request
func (v *DefaultValidator) ValidateImportRequest(ctx context.Context, req *ImportRequest) error {
	if err := v.validateWorkspaceID(req.WorkspaceID); err != nil {
		return err
	}

	if err := v.validateName(req.Name); err != nil {
		return err
	}

	if err := v.validateData(req.Data, req.TextData); err != nil {
		return err
	}

	if err := v.validateDomainData(req.DomainData); err != nil {
		return err
	}

	return nil
}

// ValidateWorkspaceAccess validates that the user has access to the workspace
func (v *DefaultValidator) ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	// TODO: Implement workspace access validation using existing auth middleware
	// This would typically check if the current user has access to the workspace
	return nil
}

// validateWorkspaceID validates the workspace ID
func (v *DefaultValidator) validateWorkspaceID(workspaceID idwrap.IDWrap) error {
	// Check if the ULID is zero (all zeros)
	if workspaceID.GetUlid().String() == "00000000000000000000000000" {
		return NewValidationError("workspaceId", workspaceID.String(), "workspace ID cannot be empty")
	}
	return nil
}

// validateName validates the import name
func (v *DefaultValidator) validateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewValidationError("name", name, "name cannot be empty")
	}
	if len(name) > 255 {
		return NewValidationError("name", name, "name cannot exceed 255 characters")
	}
	return nil
}

// validateData validates that either data or textData is provided
func (v *DefaultValidator) validateData(data []byte, textData string) error {
	if len(data) == 0 && textData == "" {
		return NewValidationError("data", string(data), "either data or textData must be provided")
	}
	if len(data) > 0 {
		return v.validateHARFormat(data)
	}
	return nil
}

// validateHARFormat validates that the data is valid JSON/HAR format
func (v *DefaultValidator) validateHARFormat(data []byte) error {
	var har map[string]interface{}
	if err := json.Unmarshal(data, &har); err != nil {
		return NewValidationErrorWithCause("data", string(data[:min(len(data), 100)]), fmt.Errorf("invalid JSON format: %w", err))
	}

	// Basic HAR structure validation
	if log, ok := har["log"]; !ok {
		return NewValidationError("data", string(data[:min(len(data), 100)]), "invalid HAR format: missing 'log' field")
	} else if logMap, ok := log.(map[string]interface{}); !ok {
		return NewValidationError("data", string(data[:min(len(data), 100)]), "invalid HAR format: 'log' must be an object")
	} else if _, ok := logMap["entries"]; !ok {
		return NewValidationError("data", string(data[:min(len(data), 100)]), "invalid HAR format: missing 'entries' field in log")
	}

	return nil
}

// validateDomainData validates the domain data configuration
func (v *DefaultValidator) validateDomainData(domainData []ImportDomainData) error {
	seenDomains := make(map[string]bool)
	seenVariables := make(map[string]bool)

	for i, dd := range domainData {
		if err := v.validateSingleDomainData(dd, i); err != nil {
			return err
		}

		// Check for duplicate domains
		if dd.Enabled {
			if seenDomains[dd.Domain] {
				return NewValidationError("domainData", dd.Domain, fmt.Sprintf("duplicate domain '%s' at index %d", dd.Domain, i))
			}
			seenDomains[dd.Domain] = true

			// Check for duplicate variables within the same domain
			varKey := fmt.Sprintf("%s:%s", dd.Domain, dd.Variable)
			if seenVariables[varKey] {
				return NewValidationError("domainData", dd.Variable, fmt.Sprintf("duplicate variable '%s' for domain '%s' at index %d", dd.Variable, dd.Domain, i))
			}
			seenVariables[varKey] = true
		}
	}

	return nil
}

// validateSingleDomainData validates a single domain data entry
func (v *DefaultValidator) validateSingleDomainData(dd ImportDomainData, index int) error {
	dd.Domain = strings.TrimSpace(dd.Domain)
	dd.Variable = strings.TrimSpace(dd.Variable)

	if dd.Domain == "" {
		return NewValidationError("domainData", dd.Domain, fmt.Sprintf("domain cannot be empty at index %d", index))
	}

	if dd.Variable == "" {
		return NewValidationError("domainData", dd.Variable, fmt.Sprintf("variable cannot be empty at index %d", index))
	}

	if len(dd.Domain) > 253 {
		return NewValidationError("domainData", dd.Domain, fmt.Sprintf("domain cannot exceed 253 characters at index %d", index))
	}

	if len(dd.Variable) > 100 {
		return NewValidationError("domainData", dd.Variable, fmt.Sprintf("variable cannot exceed 100 characters at index %d", index))
	}

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}