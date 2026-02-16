package rimportv2

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mfile"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/harv2"
)

// TestFormatDetector tests the format detection functionality
func TestFormatDetector(t *testing.T) {
	detector := NewFormatDetector()

	tests := []struct {
		name           string
		data           []byte
		expectedFormat Format
		minConfidence  float64
		shouldError    bool
	}{
		{
			name:           "Empty data",
			data:           []byte(""),
			expectedFormat: FormatUnknown,
			minConfidence:  0.9,
			shouldError:    false,
		},
		{
			name: "Valid HAR data",
			data: []byte(`{
				"log": {
					"entries": [
						{
							"request": {"method": "GET", "url": "https://api.example.com"},
							"response": {"status": 200}
						}
					]
				}
			}`),
			expectedFormat: FormatHAR,
			minConfidence:  0.7,
			shouldError:    false,
		},
		{
			name: "Valid YAML flow data",
			data: []byte(`flows:
  - name: test flow
    steps:
      - request:
          name: test request
          method: GET
          url: https://api.example.com`),
			expectedFormat: FormatYAML,
			minConfidence:  0.5,
			shouldError:    false,
		},
		{
			name:           "Valid CURL command",
			data:           []byte(`curl -X GET "https://api.example.com" -H "Accept: application/json"`),
			expectedFormat: FormatCURL,
			minConfidence:  0.7,
			shouldError:    false,
		},
		{
			name: "Valid Postman collection",
			data: []byte(`{
				"info": {
					"name": "Test Collection",
					"schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
				},
				"item": [
					{
						"request": {
							"method": "GET",
							"url": "https://api.example.com"
						}
					}
				]
			}`),
			expectedFormat: FormatPostman,
			minConfidence:  0.7,
			shouldError:    false,
		},
		{
			name:           "Invalid JSON",
			data:           []byte(`{invalid json`),
			expectedFormat: FormatUnknown,
			minConfidence:  0.0,
			shouldError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.DetectFormat(tt.data)

			if result.Format != tt.expectedFormat {
				t.Errorf("Expected format %v, got %v", tt.expectedFormat, result.Format)
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("Expected confidence >= %.2f, got %.2f", tt.minConfidence, result.Confidence)
			}

			if tt.shouldError && result.Reason == "" {
				t.Error("Expected error but got none")
			}

			t.Logf("Detection result: format=%v, confidence=%.2f, reason=%s",
				result.Format, result.Confidence, result.Reason)
		})
	}
}

// TestTranslatorRegistry tests the translator registry functionality
func TestTranslatorRegistry(t *testing.T) {
	registry := NewTranslatorRegistry(nil)

	// Test supported formats
	formats := registry.GetSupportedFormats()
	expectedFormats := []Format{FormatHAR, FormatYAML, FormatJSON, FormatCURL, FormatPostman, FormatOpenAPI}

	if len(formats) != len(expectedFormats) {
		t.Errorf("Expected %d formats, got %d", len(expectedFormats), len(formats))
	}

	// Test format validation
	validHAR := []byte(`{
		"log": {
			"version": "1.2",
			"creator": {"name": "test", "version": "1.0"},
			"entries": [
				{
					"request": {"method": "GET", "url": "https://api.example.com"},
					"response": {"status": 200}
				}
			]
		}
	}`)
	if err := registry.ValidateFormat(validHAR, FormatHAR); err != nil {
		t.Errorf("HAR validation failed: %v", err)
	}

	// Test unsupported format
	if err := registry.ValidateFormat(validHAR, FormatUnknown); err == nil {
		t.Error("Expected validation error for unknown format")
	}
}

// TestServiceUnifiedImport tests the unified import functionality
func TestServiceUnifiedImport(t *testing.T) {
	// Create mock dependencies
	importer := &MockImporter{}
	validator := &MockValidator{}

	service := NewService(importer, validator)

	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name        string
		req         *ImportRequest
		expectError bool
	}{
		{
			name: "Valid HAR import",
			req: &ImportRequest{
				WorkspaceID: workspaceID,
				Name:        "Test HAR Import",
				Data: []byte(`{
					"log": {
						"entries": [
							{
								"request": {"method": "GET", "url": "https://api.example.com"},
								"response": {"status": 200}
							}
						]
					}
				}`),
			},
			expectError: false,
		},
		{
			name: "Empty data",
			req: &ImportRequest{
				WorkspaceID: workspaceID,
				Name:        "Empty Import",
				Data:        []byte{},
			},
			expectError: true,
		},
		{
			name: "Invalid workspace ID",
			req: &ImportRequest{
				WorkspaceID: idwrap.IDWrap{},
				Name:        "Invalid Workspace",
				Data:        []byte(`{"test": "data"}`),
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock successful validation
			validator.validateFunc = func(ctx context.Context, req *ImportRequest) error {
				if req.WorkspaceID.Compare(idwrap.IDWrap{}) == 0 {
					return ErrWorkspaceNotFound
				}
				return nil
			}

			validator.validateWorkspaceFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
				if workspaceID.Compare(idwrap.IDWrap{}) == 0 {
					return ErrWorkspaceNotFound
				}
				return nil
			}

			// Skip actual storage test for now since it requires real services
			if len(tt.req.Data) > 0 {
				result, err := service.DetectFormat(ctx, tt.req.Data)
				if err != nil && !tt.expectError {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != nil {
					t.Logf("Format detected: %s with confidence %.2f", result.Format, result.Confidence)
				}
			}
		})
	}
}

// TestValidation tests the enhanced validation functionality
func TestValidation(t *testing.T) {
	service := NewService(&MockImporter{}, &MockValidator{})
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	tests := []struct {
		name        string
		req         *ImportRequest
		expectError bool
		errorType   error
	}{
		{
			name: "Valid request",
			req: &ImportRequest{
				WorkspaceID: workspaceID,
				Name:        "Valid Request",
				Data:        []byte(`{"test": "data"}`),
			},
			expectError: false,
		},
		{
			name: "Request with domain data",
			req: &ImportRequest{
				WorkspaceID: workspaceID,
				Name:        "Request with Domains",
				Data:        []byte(`{"test": "data"}`),
				DomainData: []ImportDomainData{
					{Enabled: true, Domain: "api.example.com", Variable: "api_host"},
				},
			},
			expectError: false,
		},
		{
			name: "Invalid domain data - empty domain",
			req: &ImportRequest{
				WorkspaceID: workspaceID,
				Name:        "Invalid Domain",
				Data:        []byte(`{"test": "data"}`),
				DomainData: []ImportDomainData{
					{Enabled: true, Domain: "", Variable: "var"},
				},
			},
			expectError: true,
		},
		{
			name: "Duplicate domain configuration",
			req: &ImportRequest{
				WorkspaceID: workspaceID,
				Name:        "Duplicate Domain",
				Data:        []byte(`{"test": "data"}`),
				DomainData: []ImportDomainData{
					{Enabled: true, Domain: "api.example.com", Variable: "api_host"},
					{Enabled: true, Domain: "api.example.com", Variable: "api_base"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sanitized, err := service.ValidateAndSanitizeRequest(ctx, tt.req)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorType != nil && !errors.Is(err, tt.errorType) {
					t.Errorf("Expected error type %T, got %T", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if sanitized == nil {
					t.Error("Expected sanitized request but got nil")
				}
			}
		})
	}
}

// TestConstraints tests the import constraints functionality
func TestConstraints(t *testing.T) {
	constraints := DefaultConstraints()

	if constraints.MaxDataSizeBytes != 50*1024*1024 {
		t.Errorf("Expected max data size 50MB, got %d bytes", constraints.MaxDataSizeBytes)
	}

	expectedFormats := []Format{FormatHAR, FormatYAML, FormatJSON, FormatCURL, FormatPostman, FormatOpenAPI}
	if len(constraints.SupportedFormats) != len(expectedFormats) {
		t.Errorf("Expected %d supported formats, got %d", len(expectedFormats), len(constraints.SupportedFormats))
	}

	if constraints.Timeout != 30*time.Minute {
		t.Errorf("Expected timeout 30 minutes, got %v", constraints.Timeout)
	}
}

// Mock implementations for testing

type MockImporter struct {
	importFunc func(context.Context, []byte, idwrap.IDWrap) (*TranslationResult, error)
	storeFunc  func(context.Context, *TranslationResult) (map[idwrap.IDWrap]bool, map[idwrap.IDWrap]bool, []menv.Variable, []menv.Variable, error)
}

func (m *MockImporter) ImportAndStore(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	// Mock implementation
	return &harv2.HarResolved{}, nil
}

func (m *MockImporter) ImportAndStoreUnified(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	if m.importFunc != nil {
		return m.importFunc(ctx, data, workspaceID)
	}
	// Mock implementation
	return &TranslationResult{}, nil
}

func (m *MockImporter) StoreHTTPEntities(ctx context.Context, httpReqs []*mhttp.HTTP) error {
	return nil
}

func (m *MockImporter) StoreFiles(ctx context.Context, files []*mfile.File) error {
	return nil
}

func (m *MockImporter) StoreFlow(ctx context.Context, flow *mflow.Flow) error {
	return nil
}

func (m *MockImporter) StoreFlows(ctx context.Context, flows []*mflow.Flow) error {
	return nil
}

func (m *MockImporter) StoreImportResults(ctx context.Context, results *ImportResults) error {
	return nil
}

func (m *MockImporter) StoreUnifiedResults(ctx context.Context, results *TranslationResult) (map[idwrap.IDWrap]bool, map[idwrap.IDWrap]bool, []menv.Variable, []menv.Variable, error) {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, results)
	}
	return nil, nil, nil, nil, nil
}

func (m *MockImporter) StoreDomainVariables(ctx context.Context, workspaceID idwrap.IDWrap, domainData []ImportDomainData) ([]menv.Env, []menv.Variable, []menv.Variable, error) {
	// Default mock implementation - no-op
	return nil, nil, nil, nil
}

type MockValidator struct {
	validateFunc          func(ctx context.Context, req *ImportRequest) error
	validateWorkspaceFunc func(ctx context.Context, workspaceID idwrap.IDWrap) error
	validateDataSizeFunc  func(ctx context.Context, data []byte) error
	validateFormatFunc    func(ctx context.Context, format Format) error
}

func (m *MockValidator) ValidateImportRequest(ctx context.Context, req *ImportRequest) error {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, req)
	}
	return nil
}

func (m *MockValidator) ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	if m.validateWorkspaceFunc != nil {
		return m.validateWorkspaceFunc(ctx, workspaceID)
	}
	return nil
}

func (m *MockValidator) ValidateDataSize(ctx context.Context, data []byte) error {
	if m.validateDataSizeFunc != nil {
		return m.validateDataSizeFunc(ctx, data)
	}
	return nil
}

func (m *MockValidator) ValidateFormatSupport(ctx context.Context, format Format) error {
	if m.validateFormatFunc != nil {
		return m.validateFormatFunc(ctx, format)
	}
	return nil
}

// TestApplyDomainReplacements tests the domain replacement functionality
func TestApplyDomainReplacements(t *testing.T) {
	tests := []struct {
		name         string
		httpRequests []mhttp.HTTP
		domainData   []ImportDomainData
		expectedURLs []string
	}{
		{
			name: "Replace single domain",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com/users"},
				{Url: "https://api.example.com/posts"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
			},
			expectedURLs: []string{
				"{{API_HOST}}/users",
				"{{API_HOST}}/posts",
			},
		},
		{
			name: "Replace multiple domains",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com/users"},
				{Url: "https://auth.example.com/login"},
				{Url: "https://api.example.com/posts"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
				{Enabled: true, Domain: "auth.example.com", Variable: "AUTH_HOST"},
			},
			expectedURLs: []string{
				"{{API_HOST}}/users",
				"{{AUTH_HOST}}/login",
				"{{API_HOST}}/posts",
			},
		},
		{
			name: "Skip disabled domain mappings",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com/users"},
				{Url: "https://skip.example.com/data"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
				{Enabled: false, Domain: "skip.example.com", Variable: "SKIP_HOST"},
			},
			expectedURLs: []string{
				"{{API_HOST}}/users",
				"https://skip.example.com/data", // unchanged
			},
		},
		{
			name: "Skip empty variable mappings",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com/users"},
				{Url: "https://novar.example.com/data"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
				{Enabled: true, Domain: "novar.example.com", Variable: ""},
			},
			expectedURLs: []string{
				"{{API_HOST}}/users",
				"https://novar.example.com/data", // unchanged
			},
		},
		{
			name: "Preserve query parameters",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com/search?q=test&limit=10"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
			},
			expectedURLs: []string{
				"{{API_HOST}}/search?q=test&limit=10",
			},
		},
		{
			name: "Handle URL without path",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com"},
				{Url: "https://api.example.com/"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
			},
			expectedURLs: []string{
				"{{API_HOST}}",
				"{{API_HOST}}",
			},
		},
		{
			name: "No domain mappings - unchanged",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com/users"},
			},
			domainData: []ImportDomainData{},
			expectedURLs: []string{
				"https://api.example.com/users",
			},
		},
		{
			name: "Domain with port",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com:8080/users"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
			},
			expectedURLs: []string{
				"{{API_HOST}}/users",
			},
		},
		{
			name: "Unmatched domain - unchanged",
			httpRequests: []mhttp.HTTP{
				{Url: "https://other.example.com/data"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
			},
			expectedURLs: []string{
				"https://other.example.com/data",
			},
		},
		{
			name: "Preserve template variables in path (from depfinder)",
			httpRequests: []mhttp.HTTP{
				{Url: "https://api.example.com/api/categories/{{ request_5.response.body.id }}"},
				{Url: "https://api.example.com/users/{{userId}}/posts"},
			},
			domainData: []ImportDomainData{
				{Enabled: true, Domain: "api.example.com", Variable: "API_HOST"},
			},
			expectedURLs: []string{
				"{{API_HOST}}/api/categories/{{ request_5.response.body.id }}",
				"{{API_HOST}}/users/{{userId}}/posts",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyDomainReplacements(tt.httpRequests, tt.domainData)

			if len(result) != len(tt.expectedURLs) {
				t.Fatalf("Expected %d results, got %d", len(tt.expectedURLs), len(result))
			}

			for i, expectedURL := range tt.expectedURLs {
				if result[i].Url != expectedURL {
					t.Errorf("Request %d: expected URL %q, got %q", i, expectedURL, result[i].Url)
				}
			}
		})
	}
}

// BenchmarkFormatDetection benchmarks the format detection performance
func BenchmarkFormatDetection(b *testing.B) {
	detector := NewFormatDetector()

	harData := []byte(`{
		"log": {
			"entries": [
				{
					"request": {"method": "GET", "url": "https://api.example.com"},
					"response": {"status": 200}
				}
			]
		}
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = detector.DetectFormat(harData)
	}
}

// BenchmarkTranslatorRegistry benchmarks the translator registry performance
func BenchmarkTranslatorRegistry(b *testing.B) {
	registry := NewTranslatorRegistry(nil)
	ctx := context.Background()
	workspaceID := idwrap.NewNow()

	harData := []byte(`{
		"log": {
			"entries": [
				{
					"request": {"method": "GET", "url": "https://api.example.com"},
					"response": {"status": 200}
				}
			]
		}
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = registry.DetectAndTranslate(ctx, harData, workspaceID)
	}
}
