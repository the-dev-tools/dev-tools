package rimportv2

import (
	"context"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/translate/harv2"
)

// DefaultHARTranslator implements the HARTranslator interface using the existing harv2 package
type DefaultHARTranslator struct{}

// NewHARTranslator creates a new DefaultHARTranslator
func NewHARTranslator() *DefaultHARTranslator {
	return &DefaultHARTranslator{}
}

// ConvertHAR converts HAR data to modern models using the harv2 package
func (t *DefaultHARTranslator) ConvertHAR(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	// Parse HAR data from bytes
	har, err := harv2.ConvertRaw(data)
	if err != nil {
		return nil, err
	}

	// Use the existing harv2 package which already implements modern HAR translation
	// harv2.ConvertHAR returns HarResolved with modern mhttp.HTTP and mfile.File models
	return harv2.ConvertHAR(har, workspaceID)
}