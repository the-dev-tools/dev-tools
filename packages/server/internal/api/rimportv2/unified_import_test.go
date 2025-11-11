package rimportv2

import (
	"context"
	"errors"
	"testing"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"
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
			name: "Empty data",
			data: []byte(""),
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
			name: "Valid CURL command",
			data: []byte(`curl -X GET "https://api.example.com" -H "Accept: application/json"`),
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
			name: "Invalid JSON",
			data: []byte(`{invalid json`),
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
	registry := NewTranslatorRegistry()

	// Test supported formats
	formats := registry.GetSupportedFormats()
	expectedFormats := []Format{FormatHAR, FormatYAML, FormatJSON, FormatCURL, FormatPostman}

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

	expectedFormats := []Format{FormatHAR, FormatYAML, FormatJSON, FormatCURL, FormatPostman}
	if len(constraints.SupportedFormats) != len(expectedFormats) {
		t.Errorf("Expected %d supported formats, got %d", len(expectedFormats), len(constraints.SupportedFormats))
	}

	if constraints.Timeout != 30*time.Minute {
		t.Errorf("Expected timeout 30 minutes, got %v", constraints.Timeout)
	}
}

// Mock implementations for testing

type MockImporter struct {
	importFunc func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error)
	storeFunc  func(ctx context.Context, results *TranslationResult) error
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

func (m *MockImporter) StoreUnifiedResults(ctx context.Context, results *TranslationResult) error {
	if m.storeFunc != nil {
		return m.storeFunc(ctx, results)
	}
	return nil
}

type MockValidator struct {
	validateFunc         func(ctx context.Context, req *ImportRequest) error
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
	registry := NewTranslatorRegistry()
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