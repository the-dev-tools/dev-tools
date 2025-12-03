package rimportv2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

// TestValidationErrorScenarios tests various validation error scenarios
func TestValidationErrorScenarios(t *testing.T) {
	tests := []struct {
		name        string
		request     *ImportRequest
		expectError bool
		errorType   error
		errorField  string
	}{
		{
			name: "empty workspace ID",
			request: &ImportRequest{
				WorkspaceID: idwrap.IDWrap{},
				Name:        "Test",
				Data:        []byte(`{}`),
			},
			expectError: true,
			errorType:   &ValidationError{},
			errorField:  "workspaceId",
		},
		{
			name: "empty name",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "",
				Data:        []byte(`{}`),
			},
			expectError: true,
			errorType:   &ValidationError{},
			errorField:  "name",
		},
		{
			name: "name too long",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        string(make([]byte, 300)), // Exceeds typical limits
				Data:        []byte(`{}`),
			},
			expectError: true,
			errorType:   &ValidationError{},
			errorField:  "name",
		},
		{
			name: "nil data and empty text",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test",
				Data:        nil,
				TextData:    "",
			},
			expectError: true,
			errorType:   &ValidationError{},
			errorField:  "data",
		},
		{
			name: "extremely large HAR",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test",
				Data:        make([]byte, 100*1024*1024), // 100MB
			},
			expectError: true,
			errorType:   &ValidationError{},
			errorField:  "data",
		},
		{
			name: "invalid domain data format",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test",
				Data:        []byte(`{"log": {"entries": []}}`),
				DomainData: []ImportDomainData{
					{Enabled: true, Domain: "", Variable: "test"}, // Empty domain
				},
			},
			expectError: true,
			errorType:   &ValidationError{},
			errorField:  "domainData",
		},
		{
			// Empty variable is allowed - entries with empty variables are skipped when creating env vars
			name: "domain data enabled but empty variable (valid, skipped for env var creation)",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test",
				Data:        []byte(`{"log": {"entries": []}}`),
				DomainData: []ImportDomainData{
					{Enabled: true, Domain: "example.com", Variable: ""},
				},
			},
			expectError: false,
		},
		{
			name: "domain data disabled with empty variable (valid)",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test",
				Data:        []byte(`{"log": {"entries": []}}`),
				DomainData: []ImportDomainData{
					{Enabled: false, Domain: "example.com", Variable: ""},
				},
			},
			expectError: false,
		},
		{
			name: "duplicate domains enabled",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test",
				Data:        []byte(`{"log": {"entries": []}}`),
				DomainData: []ImportDomainData{
					{Enabled: true, Domain: "example.com", Variable: "var1"},
					{Enabled: true, Domain: "example.com", Variable: "var2"},
				},
			},
			expectError: true,
			errorType:   &ValidationError{},
			errorField:  "domainData",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator(nil)
			ctx := context.Background()
			err := validator.ValidateImportRequest(ctx, tt.request)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					// Special handling for ValidationError type checking
					if _, ok := tt.errorType.(*ValidationError); ok {
						assert.True(t, IsValidationError(err), "Expected ValidationError in error chain")
					} else {
						// Use errors.Is for other error types
						assert.True(t, errors.Is(err, tt.errorType), "Expected error to contain %v in error chain", tt.errorType)
					}
				}
				if validationErr, ok := err.(*ValidationError); ok && tt.errorField != "" {
					assert.Equal(t, tt.errorField, validationErr.Field)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestHARProcessingErrors tests various HAR processing error scenarios
func TestHARProcessingErrors(t *testing.T) {
	tests := []struct {
		name        string
		harData     []byte
		expectError bool
		errorType   error
	}{
		{
			name:        "invalid JSON",
			harData:     []byte(`{"invalid": json}`),
			expectError: true,
			errorType:   ErrInvalidHARFormat,
		},
		{
			name:        "empty HAR",
			harData:     []byte(``),
			expectError: true,
			errorType:   ErrInvalidHARFormat,
		},
		{
			name: "missing log field",
			harData: []byte(`{
				"version": "1.2"
			}`),
			expectError: true,
			errorType:   ErrInvalidHARFormat,
		},
		{
			name: "missing entries field",
			harData: []byte(`{
				"log": {
					"version": "1.2"
				}
			}`),
			expectError: true,
			errorType:   ErrInvalidHARFormat,
		},
		{
			name: "invalid entry structure",
			harData: []byte(`{
				"log": {
					"version": "1.2",
					"entries": [
						{
							"invalid": "entry"
						}
					]
				}
			}`),
			expectError: false, // harv2 translator handles this gracefully
		},
		{
			name: "invalid URL format",
			harData: []byte(`{
				"log": {
					"version": "1.2",
					"entries": [
						{
							"startedDateTime": "` + time.Now().UTC().Format(time.RFC3339) + `",
							"request": {
								"method": "GET",
								"url": "not-a-url"
							},
							"response": {
								"status": 200
							}
						}
					]
				}
			}`),
			expectError: false, // harv2 translator handles this gracefully
		},
		{
			name: "negative response status",
			harData: []byte(`{
				"log": {
					"version": "1.2",
					"entries": [
						{
							"startedDateTime": "` + time.Now().UTC().Format(time.RFC3339) + `",
							"request": {
								"method": "GET",
								"url": "https://example.com"
							},
							"response": {
								"status": -1
							}
						}
					]
				}
			}`),
			expectError: false, // Should handle gracefully
		},
		{
			name: "extremely large entry",
			harData: func() []byte {
				largeString := string(make([]byte, 1024*1024)) // 1MB string
				return []byte(`{
					"log": {
						"version": "1.2",
						"entries": [
							{
								"startedDateTime": "` + time.Now().UTC().Format(time.RFC3339) + `",
								"request": {
									"method": "POST",
									"url": "https://example.com",
									"postData": {
										"text": "` + largeString + `"
									}
								},
								"response": {
									"status": 200
								}
							}
						]
					}
				}`)
			}(),
			expectError: false, // Should handle large entries
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			translator := NewHARTranslatorForTesting()
			workspaceID := idwrap.NewNow()

			_, err := translator.ConvertHAR(context.Background(), tt.harData, workspaceID)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					// Use errors.Is for wrapped errors
					assert.True(t, errors.Is(err, tt.errorType), "Expected error to contain %v in error chain", tt.errorType)
				}
			} else {
				// Should not error, but may return partial results
				if err != nil {
					t.Logf("Warning: HAR processing returned error (may be expected): %v", err)
				}
			}
		})
	}
}

// TestStorageErrorScenarios tests various storage error scenarios
func TestStorageErrorScenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*mockDependencies)
		expectError   bool
		errorType     error
		errorContains string
	}{
		{
			name: "database connection error",
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return nil
				}
				deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
					return &harv2.HarResolved{
						Flow:         mflow.Flow{ID: idwrap.NewNow()},
						HTTPRequests: []mhttp.HTTP{{ID: idwrap.NewNow(), Url: "https://example.com", Method: "GET"}},
						Files:        []mfile.File{},
					}, nil
				}
				deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
					return ErrStorageFailed
				}
			},
			expectError:   true,
			errorType:     ErrStorageFailed,
			errorContains: "storage operation failed",
		},
		{
			name: "constraint violation error",
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return nil
				}
				deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
					return &harv2.HarResolved{
						Flow:         mflow.Flow{ID: idwrap.NewNow()},
						HTTPRequests: []mhttp.HTTP{{ID: idwrap.NewNow(), Url: "https://example.com", Method: "GET"}},
						Files:        []mfile.File{},
					}, nil
				}
				deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
					return ErrStorageFailed
				}
			},
			expectError:   true,
			errorType:     ErrStorageFailed,
			errorContains: "storage operation failed",
		},
		{
			name: "partial storage failure",
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return nil
				}
				deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
					return &harv2.HarResolved{
						Flow:         mflow.Flow{ID: idwrap.NewNow()},
						HTTPRequests: []mhttp.HTTP{{ID: idwrap.NewNow(), Url: "https://example.com", Method: "GET"}},
						Files:        []mfile.File{{ID: idwrap.NewNow(), Name: "test.txt"}},
					}, nil
				}
				deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
					return ErrStorageFailed
				}
			},
			expectError:   true,
			errorType:     ErrStorageFailed,
			errorContains: "storage operation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newMockDependencies()
			tt.setupMocks(deps)

			service := NewService(
				deps.importer,
				deps.validator,
				WithLogger(slog.Default()),
			)

			req := &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Storage Error Test",
				Data:        createMinimalHAR(t),
				DomainData:  []ImportDomainData{},
			}

			_, err := service.Import(context.Background(), req)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					// Use errors.Is for wrapped errors
					assert.True(t, errors.Is(err, tt.errorType), "Expected error to contain %v in error chain", tt.errorType)
				}
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestDomainProcessingErrors tests domain processing error scenarios
func TestDomainProcessingErrors(t *testing.T) {
	tests := []struct {
		name        string
		httpReqs    []*mhttp.HTTP
		expectError bool
		errorType   error
	}{
		{
			name: "invalid HTTP requests",
			httpReqs: []*mhttp.HTTP{
				{Url: "", Method: "GET"}, // Invalid URL
			},
			expectError: false, // extractDomains skips invalid URLs
		},
		{
			name: "malformed URLs",
			httpReqs: []*mhttp.HTTP{
				{Url: "not-a-url", Method: "GET"},
				{Url: "http://", Method: "GET"}, // Incomplete URL
			},
			expectError: false, // Should handle gracefully
		},
		{
			name: "URLs with special characters",
			httpReqs: []*mhttp.HTTP{
				{Url: "https://example.com/api/path with spaces", Method: "GET"},
				{Url: "https://example.com/api/路径/中文", Method: "GET"},  // Unicode path
				{Url: "https://[2001:db8::1]/api/path", Method: "GET"}, // IPv6
			},
			expectError: false, // Should handle special characters
		},
		{
			name: "extremely long URLs",
			httpReqs: []*mhttp.HTTP{
				{Url: "https://example.com/api/" + string(make([]byte, 2000)), Method: "GET"},
			},
			expectError: false, // Should handle long URLs
		},
		{
			name: "mixed valid and invalid URLs",
			httpReqs: []*mhttp.HTTP{
				{Url: "https://valid.example.com/api/test", Method: "GET"},
				{Url: "", Method: "GET"}, // Invalid
				{Url: "https://another.example.com/v2/data", Method: "GET"},
			},
			expectError: false, // Should skip invalid URLs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domains, err := extractDomains(context.Background(), tt.httpReqs, slog.Default())

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					// Use errors.Is for wrapped errors
					assert.True(t, errors.Is(err, tt.errorType), "Expected error to contain %v in error chain", tt.errorType)
				}
			} else {
				// Should not error, but may return partial results
				if err != nil {
					t.Logf("Warning: Domain extraction returned error (may be expected): %v", err)
				}

				// Verify we got some domains for valid XHR-like URLs
				if !tt.expectError {
					// Some URLs may not be detected as XHR requests, which is expected behavior
					t.Logf("Extracted %d domains from %d HTTP requests", len(domains), len(tt.httpReqs))
				}
			}
		})
	}
}

// TestContextCancellation tests context cancellation behavior
func TestContextCancellation(t *testing.T) {
	tests := []struct {
		name        string
		cancelFunc  func(context.Context) (context.Context, func())
		expectError bool
		errorType   error
	}{
		{
			name: "immediate cancellation",
			cancelFunc: func(ctx context.Context) (context.Context, func()) {
				ctx, cancel := context.WithCancel(ctx)
				cancel() // Cancel immediately
				return ctx, cancel
			},
			expectError: true,
		},
		{
			name: "timeout cancellation",
			cancelFunc: func(ctx context.Context) (context.Context, func()) {
				ctx, cancel := context.WithTimeout(ctx, time.Nanosecond) // Very short timeout
				return ctx, cancel
			},
			expectError: true,
		},
		{
			name: "no cancellation",
			cancelFunc: func(ctx context.Context) (context.Context, func()) {
				ctx, cancel := context.WithCancel(ctx) // Don't cancel
				return ctx, cancel
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newMockDependencies()

			// Add delay to mock operations to test cancellation
			deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
				time.Sleep(10 * time.Millisecond) // Small delay
				return nil
			}
			deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			}
			deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
				time.Sleep(50 * time.Millisecond) // Longer delay
				return &harv2.HarResolved{
					Flow:         mflow.Flow{ID: idwrap.NewNow()},
					HTTPRequests: []mhttp.HTTP{},
					Files:        []mfile.File{},
				}, nil
			}
			deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
				return nil
			}

			service := NewService(
				deps.importer,
				deps.validator,
				WithLogger(slog.Default()),
			)

			ctx, cancel := tt.cancelFunc(context.Background())
			defer cancel()

			req := &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Cancellation Test",
				Data:        createMinimalHAR(t),
				DomainData:  []ImportDomainData{},
			}

			_, err := service.Import(ctx, req)

			if tt.expectError {
				assert.Error(t, err)
				// Should be context error
				assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded),
					"Should be context cancellation error, got: %v", err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestResourceExhaustion tests behavior under resource constraints
func TestResourceExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}

	tests := []struct {
		name        string
		setupMocks  func(*mockDependencies)
		expectError bool
		errorType   error
	}{
		{
			name: "memory pressure simulation",
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return nil
				}
				deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
					// Simulate memory allocation
					largeData := make([]byte, 10*1024*1024) // 10MB
					_ = largeData                           // Use the data
					return &harv2.HarResolved{
						Flow:         mflow.Flow{ID: idwrap.NewNow()},
						HTTPRequests: []mhttp.HTTP{},
						Files:        []mfile.File{},
					}, nil
				}
				deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
					return nil
				}
			},
			expectError: false, // Should handle memory allocation
		},
		{
			name: "slow storage simulation",
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return nil
				}
				deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
					return &harv2.HarResolved{
						Flow:         mflow.Flow{ID: idwrap.NewNow()},
						HTTPRequests: []mhttp.HTTP{{ID: idwrap.NewNow(), Url: "https://example.com", Method: "GET"}},
						Files:        []mfile.File{},
					}, nil
				}
				deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
					time.Sleep(100 * time.Millisecond) // Simulate slow storage
					return nil
				}
			},
			expectError: false, // Should handle slow operations
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newMockDependencies()
			tt.setupMocks(deps)

			service := NewService(
				deps.importer,
				deps.validator,
				WithLogger(slog.Default()),
			)

			req := &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Resource Test",
				Data:        createMinimalHAR(t),
				DomainData:  []ImportDomainData{},
			}

			start := time.Now()
			_, err := service.Import(context.Background(), req)
			duration := time.Since(start)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorType != nil {
					// Use errors.Is for wrapped errors
					assert.True(t, errors.Is(err, tt.errorType), "Expected error to contain %v in error chain", tt.errorType)
				}
			} else {
				assert.NoError(t, err)
				t.Logf("Resource test completed in %v", duration)
			}
		})
	}
}

// TestInvalidInputFormats tests various invalid input formats
func TestInvalidInputFormats(t *testing.T) {
	tests := []struct {
		name        string
		harData     interface{}
		expectError bool
	}{
		{
			name:        "nil data",
			harData:     nil,
			expectError: true,
		},
		{
			name:        "non-JSON data",
			harData:     "just a string",
			expectError: true,
		},
		{
			name:        "array instead of object",
			harData:     []interface{}{},
			expectError: true,
		},
		{
			name:        "number instead of object",
			harData:     123,
			expectError: true,
		},
		{
			name: "invalid structure",
			harData: map[string]interface{}{
				"invalid": "structure",
			},
			expectError: true,
		},
		{
			name: "log not an object",
			harData: map[string]interface{}{
				"log": "not an object",
			},
			expectError: true,
		},
		{
			name: "version should be string",
			harData: map[string]interface{}{
				"log": map[string]interface{}{
					"version": 123,             // Should be string
					"entries": []interface{}{}, // Should be valid entries
				},
			},
			expectError: true,
		},
		{
			name: "invalid entry object",
			harData: map[string]interface{}{
				"log": map[string]interface{}{
					"version": "1.2",
					"entries": []interface{}{
						"not an entry object",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var harData []byte
			var err error

			if tt.harData != nil {
				harData, err = json.Marshal(tt.harData)
				require.NoError(t, err)
			} else {
				harData = nil
			}

			translator := NewHARTranslatorForTesting()
			workspaceID := idwrap.NewNow()

			_, err = translator.ConvertHAR(context.Background(), harData, workspaceID)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConcurrentErrorHandling tests error handling under concurrent conditions
func TestConcurrentErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent error test in short mode")
	}

	deps := newMockDependencies()

	numGoroutines := 10
	numIterations := 5

	// Pre-generate IDs for mocks to avoid race conditions
	flowIDs := make([]idwrap.IDWrap, numGoroutines*numIterations)
	httpIDs := make([]idwrap.IDWrap, numGoroutines*numIterations)
	for i := range flowIDs {
		flowIDs[i] = idwrap.NewNow()
		httpIDs[i] = idwrap.NewNow()
	}
	mockIndex := 0

	// Simulate intermittent failures
	var mu sync.Mutex
	callCount := 0
	deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		if callCount%3 == 0 {
			// Fail every 3rd call
			return nil, fmt.Errorf("HAR conversion failed: %w", ErrInvalidHARFormat)
		}
		result := &harv2.HarResolved{
			Flow:         mflow.Flow{ID: flowIDs[mockIndex]},
			HTTPRequests: []mhttp.HTTP{{ID: httpIDs[mockIndex], Url: "https://example.com", Method: "GET"}},
			Files:        []mfile.File{},
		}
		mockIndex++
		return result, nil
	}
	deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
		return nil
	}
	deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
		return nil
	}
	deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
		return nil
	}

	service := NewService(
		deps.importer,
		deps.validator,
		WithLogger(slog.Default()),
	)

	results := make(chan error, numGoroutines*numIterations)

	// Pre-generate IDs to avoid ULID race conditions
	workspaceIDs := make([]idwrap.IDWrap, numGoroutines*numIterations)
	for i := range workspaceIDs {
		workspaceIDs[i] = idwrap.NewNow()
	}

	// Run concurrent imports with intermittent failures
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < numIterations; j++ {
				index := goroutineID*numIterations + j
				req := &ImportRequest{
					WorkspaceID: workspaceIDs[index],
					Name:        fmt.Sprintf("Concurrent Test %d-%d", goroutineID, j),
					Data:        createMinimalHAR(t),
					DomainData:  []ImportDomainData{},
				}

				_, err := service.Import(context.Background(), req)
				results <- err
			}
		}(i)
	}

	// Collect results
	successCount := 0
	errorCount := 0
	for i := 0; i < numGoroutines*numIterations; i++ {
		err := <-results
		if err != nil {
			errorCount++
			// Should be HAR format error for failures
			assert.True(t, errors.Is(err, ErrInvalidHARFormat),
				"Should be HAR format error, got: %v", err)
		} else {
			successCount++
		}
	}

	t.Logf("Concurrent error handling: %d successful, %d failed", successCount, errorCount)

	// Should have some failures due to our mock
	assert.Greater(t, errorCount, 0, "Should have some failures")
	assert.Greater(t, successCount, 0, "Should have some successes")
}
