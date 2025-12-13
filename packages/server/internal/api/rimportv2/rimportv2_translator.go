package rimportv2

import (
	"context"
	"encoding/json"
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/translate/harv2"
)

// newHARTranslator creates a new HAR translator (private method)
func newHARTranslator() *defaultHARTranslator {
	return &defaultHARTranslator{}
}

// defaultHARTranslator handles HAR file processing using the existing harv2 package (private struct)
type defaultHARTranslator struct{}

// convertHAR converts HAR data to modern models using the harv2 package (private method)
func (t *defaultHARTranslator) convertHAR(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	// Validate basic HAR structure before parsing
	if err := t.validateHARStructure(data); err != nil {
		return nil, err
	}

	// Parse HAR data from bytes
	har, err := harv2.ConvertRaw(data)
	if err != nil {
		return nil, fmt.Errorf("HAR conversion failed: %w", err)
	}

	// Use the existing harv2 package which already implements modern HAR translation
	// harv2.ConvertHAR returns HarResolved with modern mhttp.HTTP and mfile.File models
	resolved, err := harv2.ConvertHAR(har, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("HAR processing failed: %w", err)
	}

	return resolved, nil
}

// validateHARStructure validates basic HAR structure (private method)
func (t *defaultHARTranslator) validateHARStructure(data []byte) error {
	var har map[string]interface{}
	if err := json.Unmarshal(data, &har); err != nil {
		return ErrInvalidHARFormat
	}

	// Basic HAR structure validation
	log, ok := har["log"]
	if !ok {
		return ErrInvalidHARFormat
	}

	logMap, ok := log.(map[string]interface{})
	if !ok {
		return ErrInvalidHARFormat
	}

	if _, ok := logMap["entries"]; !ok {
		return ErrInvalidHARFormat
	}

	// Validate version field type - must be a string according to HAR spec
	if version, ok := logMap["version"]; ok {
		if _, ok := version.(string); !ok {
			return ErrInvalidHARFormat
		}
	}

	return nil
}

// NewHARTranslatorForTesting creates a new HAR translator for testing purposes
// This provides access to the HAR translator for test files while keeping the main implementation private
func NewHARTranslatorForTesting() *defaultHARTranslator {
	return newHARTranslator()
}

// ConvertHARForTesting exposes the ConvertHAR method for testing purposes
func (t *defaultHARTranslator) ConvertHAR(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	return t.convertHAR(ctx, data, workspaceID)
}
