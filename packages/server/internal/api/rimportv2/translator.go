package rimportv2

import (
	"context"
	"encoding/json"
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/translate/harv2"
)

// DefaultHARTranslator handles HAR file processing using the existing harv2 package
type DefaultHARTranslator struct{}

// NewHARTranslator creates a new DefaultHARTranslator
func NewHARTranslator() *DefaultHARTranslator {
	return &DefaultHARTranslator{}
}

// ConvertHAR converts HAR data to modern models using the harv2 package
func (t *DefaultHARTranslator) ConvertHAR(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
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

// validateHARStructure validates basic HAR structure
func (t *DefaultHARTranslator) validateHARStructure(data []byte) error {
	var har map[string]interface{}
	if err := json.Unmarshal(data, &har); err != nil {
		return ErrInvalidHARFormat
	}

	// Basic HAR structure validation
	if log, ok := har["log"]; !ok {
		return ErrInvalidHARFormat
	} else if logMap, ok := log.(map[string]interface{}); !ok {
		return ErrInvalidHARFormat
	} else if _, ok := logMap["entries"]; !ok {
		return ErrInvalidHARFormat
	}

	return nil
}
