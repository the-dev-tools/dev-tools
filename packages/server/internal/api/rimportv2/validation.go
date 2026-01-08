//nolint:revive // exported
package rimportv2

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mworkspace"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/suser"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sworkspace"
)

// DefaultValidator implements the Validator interface
type DefaultValidator struct {
	us         *suser.UserService
	userReader *sworkspace.UserReader
}

// NewValidator creates a new DefaultValidator with user service dependency
func NewValidator(us *suser.UserService, userReader *sworkspace.UserReader) *DefaultValidator {
	return &DefaultValidator{
		us:         us,
		userReader: userReader,
	}
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
	// Check if user reader is available (for testing scenarios)
	if v.userReader == nil {
		// In tests or when user service is not available, skip auth check
		// This should not happen in production
		return nil
	}

	// Check workspace ownership using existing auth middleware pattern
	userID, err := mwauth.GetContextUserID(ctx)
	if err != nil {
		return err
	}

	wsUser, err := v.userReader.GetWorkspaceUsersByWorkspaceIDAndUserID(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, sworkspace.ErrWorkspaceUserNotFound) {
			return ErrPermissionDenied
		}
		return err
	}

	if wsUser.Role < mworkspace.RoleUser {
		return ErrPermissionDenied
	}

	return nil
}

// ValidateDataSize validates the size of import data
func (v *DefaultValidator) ValidateDataSize(ctx context.Context, data []byte) error {
	constraints := DefaultConstraints()

	if len(data) > int(constraints.MaxDataSizeBytes) {
		return NewValidationError("data", fmt.Sprintf("data size %d bytes exceeds maximum allowed size %d bytes", len(data), constraints.MaxDataSizeBytes))
	}

	return nil
}

// ValidateFormatSupport validates that a format is supported
func (v *DefaultValidator) ValidateFormatSupport(ctx context.Context, format Format) error {
	constraints := DefaultConstraints()

	for _, supportedFormat := range constraints.SupportedFormats {
		if supportedFormat == format {
			return nil
		}
	}

	return NewValidationError("format", fmt.Sprintf("format %v is not supported", format))
}

// validateWorkspaceID validates the workspace ID
func (v *DefaultValidator) validateWorkspaceID(workspaceID idwrap.IDWrap) error {
	// Check if the ULID is zero (all zeros)
	if workspaceID.GetUlid().String() == "00000000000000000000000000" {
		return NewValidationError("workspaceId", "workspace ID cannot be empty")
	}
	return nil
}

// validateName validates the import name
func (v *DefaultValidator) validateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return NewValidationError("name", "name cannot be empty")
	}
	if len(name) > 255 {
		return NewValidationError("name", "name cannot exceed 255 characters")
	}
	return nil
}

// validateData validates that either data or textData is provided
func (v *DefaultValidator) validateData(data []byte, textData string) error {
	if len(data) == 0 && textData == "" {
		return NewValidationError("data", "either data or textData must be provided")
	}
	if len(data) > 0 {
		// Check data size constraints during data validation
		if err := v.ValidateDataSize(context.Background(), data); err != nil {
			return err
		}
		// For unified import, we don't enforce specific format validation here
		// The format detection will handle validation later
		return nil
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
				return NewValidationError("domainData", fmt.Sprintf("duplicate domain '%s' at index %d", dd.Domain, i))
			}
			seenDomains[dd.Domain] = true

			// Check for duplicate variables within the same domain
			varKey := fmt.Sprintf("%s:%s", dd.Domain, dd.Variable)
			if seenVariables[varKey] {
				return NewValidationError("domainData", fmt.Sprintf("duplicate variable '%s' for domain '%s' at index %d", dd.Variable, dd.Domain, i))
			}
			seenVariables[varKey] = true
		}
	}

	return nil
}

// validateSingleDomainData validates a single domain data entry.
// Variable can be empty - entries with empty variables are simply skipped when creating env vars.
func (v *DefaultValidator) validateSingleDomainData(dd ImportDomainData, index int) error {
	dd.Domain = strings.TrimSpace(dd.Domain)
	dd.Variable = strings.TrimSpace(dd.Variable)

	if dd.Domain == "" {
		return NewValidationError("domainData", fmt.Sprintf("domain cannot be empty at index %d", index))
	}

	if len(dd.Domain) > 253 {
		return NewValidationError("domainData", fmt.Sprintf("domain cannot exceed 253 characters at index %d", index))
	}

	if len(dd.Variable) > 100 {
		return NewValidationError("domainData", fmt.Sprintf("variable cannot exceed 100 characters at index %d", index))
	}

	return nil
}
