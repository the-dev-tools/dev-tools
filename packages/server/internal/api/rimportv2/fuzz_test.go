package rimportv2

import (
	"context"
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// FuzzFormatDetector fuzzes the format detection logic to ensure it doesn't panic or hang
// on arbitrary inputs.
func FuzzFormatDetector(f *testing.F) {
	detector := NewFormatDetector()

	// Seed corpus
	f.Add([]byte(`{"log": {"entries": []}}`)) // HAR
	f.Add([]byte(`curl http://example.com`))  // Curl
	f.Add([]byte(`flows: []`))                // YAML
	f.Add([]byte(`random garbage data`))      // Garbage

	f.Fuzz(func(t *testing.T, data []byte) {
		// Should not panic or hang
		_ = detector.DetectFormat(data)
	})
}

// FuzzTranslatorRegistry fuzzes the translation logic
// Note: this mocks the services, so it primarily tests the parsing/translation logic
// of the individual translators (HAR, YAML, Curl, etc).
func FuzzTranslatorRegistry(f *testing.F) {
	// Use nil HTTP service for fuzzing as we don't want DB interaction
	registry := NewTranslatorRegistry(nil)
	ctx := context.Background()
	wsID := idwrap.NewNow()

	// Seed corpus
	f.Add([]byte(`{"log": {"entries": []}}`))
	f.Add([]byte(`{"invalid": "json"}`))
	f.Add([]byte(`not json at all`))
	f.Add([]byte(`curl -X GET https://example.com`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Should not panic or hang
		// We expect errors for random data, so we don't check err
		_, _ = registry.DetectAndTranslate(ctx, data, wsID)
	})
}
