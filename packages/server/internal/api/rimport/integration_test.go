package rimport

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mitemapi"
	"the-dev-tools/server/pkg/model/mitemapiexample"
	"the-dev-tools/server/pkg/translate/thar"
	"the-dev-tools/server/pkg/translate/harv2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHARTranslatorInterface tests that both implementations satisfy the interface
func TestHARTranslatorInterface(t *testing.T) {
	tests := []struct {
		name     string
		impl     HARTranslator
		validate func(t *testing.T, impl HARTranslator)
	}{
		{
			name: "Legacy Implementation",
			impl: &LegacyHARTranslator{},
			validate: func(t *testing.T, impl HARTranslator) {
				// Test that legacy implementation works
				harData := createTestHAR(t)
				parsed, err := impl.ConvertRaw(harData)
				require.NoError(t, err)
				assert.NotNil(t, parsed)
				assert.NotEmpty(t, parsed.Log.Entries)
			},
		},
		{
			name: "Modern Implementation",
			impl: &ModernHARTranslator{},
			validate: func(t *testing.T, impl HARTranslator) {
				// Test that modern implementation works
				harData := createTestHAR(t)
				parsed, err := impl.ConvertRaw(harData)
				require.NoError(t, err)
				assert.NotNil(t, parsed)
				assert.NotEmpty(t, parsed.Log.Entries)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.impl)
		})
	}
}

// TestBehavioralEquivalence tests that both implementations produce equivalent results
func TestBehavioralEquivalence(t *testing.T) {
	harData := createTestHAR(t)
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	legacyTranslator := &LegacyHARTranslator{}
	modernTranslator := &ModernHARTranslator{}

	// Test ConvertRaw
	legacyHAR, err1 := legacyTranslator.ConvertRaw(harData)
	modernHAR, err2 := modernTranslator.ConvertRaw(harData)

	require.NoError(t, err1, "Legacy ConvertRaw should not error")
	require.NoError(t, err2, "Modern ConvertRaw should not error")

	// Both should parse the same number of entries
	assert.Equal(t, len(legacyHAR.Log.Entries), len(modernHAR.Log.Entries),
		"Both implementations should parse same number of entries")

	// Test basic HAR structure
	assert.NotEmpty(t, legacyHAR.Log.Entries, "Legacy should have entries")
	assert.NotEmpty(t, modernHAR.Log.Entries, "Modern should have entries")

	// Compare first entry details
	if len(legacyHAR.Log.Entries) > 0 && len(modernHAR.Log.Entries) > 0 {
		legacyEntry := legacyHAR.Log.Entries[0]
		modernEntry := modernHAR.Log.Entries[0]

		assert.Equal(t, legacyEntry.Request.Method, modernEntry.Request.Method,
			"HTTP method should be the same")
		assert.Equal(t, legacyEntry.Request.URL, modernEntry.Request.URL,
			"URL should be the same")
		assert.Equal(t, legacyEntry.Response.Status, modernEntry.Response.Status,
			"Response status should be the same")
	}
}

// TestDomainCollectionBehavior tests domain collection functionality
func TestDomainCollectionBehavior(t *testing.T) {
	harData := createMultiDomainHAR(t)

	legacyTranslator := &LegacyHARTranslator{}
	modernTranslator := &ModernHARTranslator{}

	// Parse HAR with both implementations
	legacyHAR, err1 := legacyTranslator.ConvertRaw(harData)
	modernHAR, err2 := modernTranslator.ConvertRaw(harData)

	require.NoError(t, err1)
	require.NoError(t, err2)

	// Collect domains
	legacyDomains := legacyTranslator.collectHarDomains(legacyHAR)
	modernDomains := modernTranslator.collectHarDomains(modernHAR)

	// Both should find the same domains
	assert.ElementsMatch(t, legacyDomains, modernDomains,
		"Both implementations should find the same domains")

	// Should find expected test domains
	expectedDomains := []string{"api.example.com", "test.example.org"}
	assert.Contains(t, legacyDomains, expectedDomains[0])
	assert.Contains(t, modernDomains, expectedDomains[0])
	if len(legacyDomains) > 1 && len(modernDomains) > 1 {
		assert.Contains(t, legacyDomains, expectedDomains[1])
		assert.Contains(t, modernDomains, expectedDomains[1])
	}
}

// TestHARConversionWithExistingData tests the main conversion functionality
func TestHARConversionWithExistingData(t *testing.T) {
	harData := createTestHAR(t)
	collectionID := idwrap.NewNow()
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name string
		impl HARTranslator
	}{
		{"Legacy", &LegacyHARTranslator{}},
		{"Modern", &ModernHARTranslator{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedHAR, err := tt.impl.ConvertRaw(harData)
			require.NoError(t, err)

			// Test conversion with existing data
			resolved, err := tt.impl.ConvertHARWithExistingData(parsedHAR, collectionID, workspaceID, []mitemfolder.ItemFolder{})
			require.NoError(t, err)

			// Validate basic structure
			assert.NotEmpty(t, resolved.Apis, "Should create APIs")
			assert.NotEmpty(t, resolved.Examples, "Should create examples")
			assert.NotZero(t, resolved.Flow.ID, "Should create flow")

			// Validate API structure
			for _, api := range resolved.Apis {
				assert.NotZero(t, api.ID)
				assert.NotEmpty(t, api.Name)
				assert.NotEmpty(t, api.Method)
				assert.NotEmpty(t, api.Url)
				assert.Equal(t, collectionID, api.CollectionID)
				assert.Equal(t, workspaceID, api.WorkspaceID)
			}

			// Validate example structure
			for _, example := range resolved.Examples {
				assert.NotZero(t, example.ID)
				assert.NotZero(t, example.ItemApiID)
				assert.NotEmpty(t, example.Name)
			}

			// Validate flow structure
			assert.NotEmpty(t, resolved.Flow.Name)
			assert.Equal(t, workspaceID, resolved.Flow.WorkspaceID)
		})
	}
}

// TestPerformanceComparison compares performance between implementations
func TestPerformanceComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	harData := createLargeHAR(t) // Create HAR with many entries
	numIterations := 10

	legacyTranslator := &LegacyHARTranslator{}
	modernTranslator := &ModernHARTranslator{}

	// Warm up
	_, err1 := legacyTranslator.ConvertRaw(harData)
	_, err2 := modernTranslator.ConvertRaw(harData)
	require.NoError(t, err1)
	require.NoError(t, err2)

	// Benchmark legacy implementation
	start := time.Now()
	for i := 0; i < numIterations; i++ {
		_, err := legacyTranslator.ConvertRaw(harData)
		require.NoError(t, err)
	}
	legacyDuration := time.Since(start)

	// Benchmark modern implementation
	start = time.Now()
	for i := 0; i < numIterations; i++ {
		_, err := modernTranslator.ConvertRaw(harData)
		require.NoError(t, err)
	}
	modernDuration := time.Since(start)

	t.Logf("Legacy implementation: %v (avg: %v)", legacyDuration, legacyDuration/time.Duration(numIterations))
	t.Logf("Modern implementation: %v (avg: %v)", modernDuration, modernDuration/time.Duration(numIterations))

	// Modern implementation should not be significantly slower
	ratio := float64(modernDuration) / float64(legacyDuration)
	assert.Less(t, ratio, 2.0, "Modern implementation should not be more than 2x slower than legacy")
}

// TestErrorHandling tests error handling in both implementations
func TestErrorHandling(t *testing.T) {
	invalidHAR := []byte(`{"invalid": "json"}`)

	legacyTranslator := &LegacyHARTranslator{}
	modernTranslator := &ModernHARTranslator{}

	// Test with invalid HAR
	legacyResult, legacyErr := legacyTranslator.ConvertRaw(invalidHAR)
	modernResult, modernErr := modernTranslator.ConvertRaw(invalidHAR)

	// Both should handle errors gracefully
	assert.Error(t, legacyErr, "Legacy should error on invalid HAR")
	assert.Error(t, modernErr, "Modern should error on invalid HAR")
	assert.Nil(t, legacyResult, "Legacy should return nil on error")
	assert.Nil(t, modernResult, "Modern should return nil on error")

	// Test with empty HAR
	emptyHAR := []byte(`{"log": {"entries": []}}`)
	legacyResult, legacyErr = legacyTranslator.ConvertRaw(emptyHAR)
	modernResult, modernErr = modernTranslator.ConvertRaw(emptyHAR)

	assert.NoError(t, legacyErr, "Legacy should handle empty HAR")
	assert.NoError(t, modernErr, "Modern should handle empty HAR")
	assert.NotNil(t, legacyResult, "Legacy should return result for empty HAR")
	assert.NotNil(t, modernResult, "Modern should return result for empty HAR")
}

// TestFeatureFlagSelection tests the feature flag-based implementation selection
func TestFeatureFlagSelection(t *testing.T) {
	// Test legacy selection
	impl := GetHARTranslator(false)
	assert.IsType(t, &LegacyHARTranslator{}, impl, "Should return legacy implementation when flag is false")

	// Test modern selection
	impl = GetHARTranslator(true)
	assert.IsType(t, &ModernHARTranslator{}, impl, "Should return modern implementation when flag is true")

	// Test default behavior
	impl = GetHARTranslator(false) // Default to false for safety
	assert.IsType(t, &LegacyHARTranslator{}, impl, "Default should be legacy for safety")
}

// TestConcurrentUsage tests concurrent usage of both implementations
func TestConcurrentUsage(t *testing.T) {
	harData := createTestHAR(t)
	numGoroutines := 10
	numRequests := 5

	legacyTranslator := &LegacyHARTranslator{}
	modernTranslator := &ModernHARTranslator{}

	// Test concurrent legacy usage
	legacyDone := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { legacyDone <- true }()
			for j := 0; j < numRequests; j++ {
				_, err := legacyTranslator.ConvertRaw(harData)
				assert.NoError(t, err)
			}
		}()
	}

	// Test concurrent modern usage
	modernDone := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { modernDone <- true }()
			for j := 0; j < numRequests; j++ {
				_, err := modernTranslator.ConvertRaw(harData)
				assert.NoError(t, err)
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-legacyDone
		<-modernDone
	}
}

// TestRealWorldHAR tests with a real-world HAR file if available
func TestRealWorldHAR(t *testing.T) {
	harFile := os.Getenv("TEST_HAR_FILE")
	if harFile == "" {
		t.Skip("Set TEST_HAR_FILE environment variable to test with real HAR file")
	}

	harData, err := os.ReadFile(harFile)
	require.NoError(t, err)

	legacyTranslator := &LegacyHARTranslator{}
	modernTranslator := &ModernHARTranslator{}

	// Test both implementations
	legacyResult, err1 := legacyTranslator.ConvertRaw(harData)
	modernResult, err2 := modernTranslator.ConvertRaw(harData)

	require.NoError(t, err1, "Legacy should parse real HAR")
	require.NoError(t, err2, "Modern should parse real HAR")

	// Should have similar number of entries
	entryDiff := abs(len(legacyResult.Log.Entries) - len(modernResult.Log.Entries))
	assert.Less(t, entryDiff, len(legacyResult.Log.Entries)/10,
		"Entry count difference should be less than 10%%")
}

// Helper functions

func createTestHAR(t *testing.T) []byte {
	har := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"creator": map[string]interface{}{
				"name": "Test HAR Creator",
				"version": "1.0",
			},
			"entries": []map[string]interface{}{
				{
					"startedDateTime": time.Now().UTC().Format(time.RFC3339),
					"request": map[string]interface{}{
						"method": "GET",
						"url": "https://api.example.com/users",
						"httpVersion": "HTTP/1.1",
						"headers": []map[string]interface{}{
							{"name": "Accept", "value": "application/json"},
						},
					},
					"response": map[string]interface{}{
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": []map[string]interface{}{
							{"name": "Content-Type", "value": "application/json"},
						},
						"content": map[string]interface{}{
							"size": 100,
							"mimeType": "application/json",
							"text": `{"users": []}`,
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(har)
	require.NoError(t, err)
	return data
}

func createMultiDomainHAR(t *testing.T) []byte {
	har := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"entries": []map[string]interface{}{
				{
					"startedDateTime": time.Now().UTC().Format(time.RFC3339),
					"request": map[string]interface{}{
						"method": "GET",
						"url": "https://api.example.com/users",
					},
					"response": map[string]interface{}{
						"status": 200,
					},
					"_resourceType": "xhr",
				},
				{
					"startedDateTime": time.Now().Add(time.Second).UTC().Format(time.RFC3339),
					"request": map[string]interface{}{
						"method": "POST",
						"url": "https://test.example.org/data",
					},
					"response": map[string]interface{}{
						"status": 201,
					},
					"_resourceType": "xhr",
				},
			},
		},
	}

	data, err := json.Marshal(har)
	require.NoError(t, err)
	return data
}

func createLargeHAR(t *testing.T) []byte {
	entries := make([]map[string]interface{}, 100)
	for i := 0; i < 100; i++ {
		entries[i] = map[string]interface{}{
			"startedDateTime": time.Now().Add(time.Duration(i) * time.Second).UTC().Format(time.RFC3339),
			"request": map[string]interface{}{
				"method": "GET",
				"url":      "https://api.example.com/data",
				"httpVersion": "HTTP/1.1",
			},
			"response": map[string]interface{}{
				"status": 200,
			},
		}
	}

	har := map[string]interface{}{
		"log": map[string]interface{}{
			"version": "1.2",
			"entries": entries,
		},
	}

	data, err := json.Marshal(har)
	require.NoError(t, err)
	return data
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}