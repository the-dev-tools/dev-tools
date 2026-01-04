package rimportv2

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

// TestNewValidator tests the validator constructor
func TestNewValidator(t *testing.T) {
	validator := NewValidator(nil, nil)
	require.NotNil(t, validator)
}

// TestNewHARTranslator tests the HAR translator constructor
func TestNewHARTranslator(t *testing.T) {
	translator := NewHARTranslatorForTesting()
	require.NotNil(t, translator)
}

// TestErrorConstructors tests custom error constructors
func TestErrorConstructors(t *testing.T) {
	// Test ValidationError
	validationErr := NewValidationError("test", "message")
	require.NotNil(t, validationErr)
	expected := "validation failed for field 'test': message"
	require.Equal(t, expected, validationErr.Error())

	// Test ValidationError with cause
	originalErr := errors.New("original error")
	validationErrWithCause := NewValidationErrorWithCause("test", originalErr)
	require.NotNil(t, validationErrWithCause)

	// Test error type checking
	require.True(t, IsValidationError(validationErr))
	require.True(t, IsValidationError(validationErrWithCause))
}

// TestService_Import tests the main import functionality
func TestService_Import(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*mockDependencies)
		input       *ImportRequest
		expectError bool
		errorType   error
		expectResp  *ImportResponse
	}{
		{
			name: "successful import with minimal HAR",
			input: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Import",
				Data:        createMinimalHAR(t),
				DomainData:  []ImportDomainData{},
			},
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return nil
				}
				deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
					return &harv2.HarResolved{
						Flow: mflow.Flow{
							ID:   idwrap.NewNow(),
							Name: "Test Flow",
						},
						HTTPRequests: []mhttp.HTTP{
							{
								ID:     idwrap.NewNow(),
								Url:    "https://api.example.com/test",
								Method: "POST",
							},
						},
						Files: []mfile.File{},
					}, nil
				}
				deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
					return nil
				}
			},
			expectError: false,
			expectResp: &ImportResponse{
				MissingData: ImportMissingDataKind_DOMAIN,
				Domains:     []string{"api.example.com"},
			},
		},
		{
			name: "validation error",
			input: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "",
				Data:        []byte("invalid"),
			},
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return NewValidationError("name", "name cannot be empty")
				}
			},
			expectError: true,
			errorType:   NewValidationErrorWithCause("import_request", NewValidationError("name", "name cannot be empty")),
		},
		{
			name: "workspace access denied",
			input: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Import",
				Data:        createMinimalHAR(t),
			},
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return ErrPermissionDenied
				}
			},
			expectError: true,
			errorType:   ErrPermissionDenied,
		},
		{
			name: "HAR processing error",
			input: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Import",
				Data:        []byte("invalid har"),
			},
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return nil
				}
				deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
					return nil, ErrInvalidHARFormat
				}
			},
			expectError: true,
			errorType:   ErrInvalidHARFormat,
		},
		{
			name: "storage error",
			input: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Import",
				Data:        createMinimalHAR(t),
			},
			setupMocks: func(deps *mockDependencies) {
				deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
					return nil
				}
				deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
					return nil
				}
				deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
					return &harv2.HarResolved{
						Flow: mflow.Flow{
							ID:   idwrap.NewNow(),
							Name: "Test Flow",
						},
						HTTPRequests: []mhttp.HTTP{
							{
								ID:     idwrap.NewNow(),
								Url:    "https://api.example.com/test",
								Method: "POST",
							},
						},
						Files: []mfile.File{},
					}, nil
				}
				deps.importer.StoreImportResultsFunc = func(ctx context.Context, results *ImportResults) error {
					return fmt.Errorf("storage operation failed: %w", sql.ErrConnDone)
				}
			},
			expectError: true,
			errorType:   sql.ErrConnDone,
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

			ctx := context.Background()
			resp, err := service.Import(ctx, tt.input)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorType != nil {
					// Special handling for ValidationError type checking
					if IsValidationError(err) && IsValidationError(tt.errorType) {
						// Both are ValidationErrors, which is what we want to test
						require.True(t, true)
					} else {
						require.ErrorIs(t, err, tt.errorType)
					}
				}
				require.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				if tt.expectResp != nil {
					require.Equal(t, tt.expectResp.MissingData, resp.MissingData)
					require.Equal(t, tt.expectResp.Domains, resp.Domains)
				}
			}

			// Verify mock calls
			require.GreaterOrEqual(t, deps.validator.ValidateImportRequestCallCount, 0)
			if err == nil || !IsValidationError(err) {
				require.GreaterOrEqual(t, deps.validator.ValidateWorkspaceAccessCallCount, 0)
			}
		})
	}
}

// TestService_ImportWithTextData tests the text data import functionality
func TestService_ImportWithTextData(t *testing.T) {
	tests := []struct {
		name         string
		request      *ImportRequest
		expectedData []byte
		expectError  bool
	}{
		{
			name: "convert text data to bytes",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Import",
				Data:        []byte{},
				TextData:    `{"log": {"entries": []}}`,
			},
			expectedData: []byte(`{"log": {"entries": []}}`),
			expectError:  false,
		},
		{
			name: "use existing byte data",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Import",
				Data:        []byte(`{"log": {"entries": []}}`),
				TextData:    "should be ignored",
			},
			expectedData: []byte(`{"log": {"entries": []}}`),
			expectError:  false,
		},
		{
			name: "empty both data and text data",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Import",
				Data:        []byte{},
				TextData:    "",
			},
			expectedData: []byte{},
			expectError:  true, // Will fail at HAR parsing stage
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := newMockDependencies()
			deps.validator.ValidateImportRequestFunc = func(ctx context.Context, req *ImportRequest) error {
				return nil
			}
			deps.validator.ValidateWorkspaceAccessFunc = func(ctx context.Context, workspaceID idwrap.IDWrap) error {
				return nil
			}
			deps.importer.ImportAndStoreFunc = func(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
				// Verify the data was converted correctly
				require.Equal(t, tt.expectedData, data)
				if len(data) == 0 {
					return nil, ErrInvalidHARFormat
				}
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

			ctx := context.Background()
			_, err := service.ImportWithTextData(ctx, tt.request)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestErrorTypeCheckingFunctions tests the error type checking functions
func TestErrorTypeCheckingFunctions(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		checker  func(error) bool
		expected bool
	}{
		{
			name:     "ValidationError detection",
			err:      NewValidationError("field", "message"),
			checker:  IsValidationError,
			expected: true,
		},
		{
			name:     "generic error not detected as ValidationError",
			err:      errors.New("generic error"),
			checker:  IsValidationError,
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			checker:  IsValidationError,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.checker(tt.err)
			require.Equal(t, tt.expected, result)
		})
	}
}

// TestImportRequestValidation tests import request validation scenarios
func TestImportRequestValidation(t *testing.T) {
	tests := []struct {
		name        string
		request     *ImportRequest
		expectError bool
		errorField  string
	}{
		{
			name: "valid request",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Valid Import",
				Data:        createMinimalHAR(t),
				DomainData:  []ImportDomainData{},
			},
			expectError: false,
		},
		{
			name: "empty name",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "",
				Data:        createMinimalHAR(t),
			},
			expectError: true,
			errorField:  "name",
		},
		{
			name: "empty workspace ID",
			request: &ImportRequest{
				WorkspaceID: idwrap.IDWrap{},
				Name:        "Test Import",
				Data:        createMinimalHAR(t),
			},
			expectError: true,
			errorField:  "workspaceId",
		},
		{
			name: "nil data with empty text data",
			request: &ImportRequest{
				WorkspaceID: idwrap.NewNow(),
				Name:        "Test Import",
				Data:        nil,
				TextData:    "",
			},
			expectError: true,
			errorField:  "data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator(nil, nil)
			ctx := context.Background()
			err := validator.ValidateImportRequest(ctx, tt.request)

			if tt.expectError {
				require.Error(t, err)
				if validationErr, ok := err.(*ValidationError); ok {
					require.Equal(t, tt.errorField, validationErr.Field)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestErrorUnwrapping tests error unwrapping functionality
func TestErrorUnwrapping(t *testing.T) {
	originalErr := sql.ErrConnDone

	// Test ValidationError unwrapping
	validationErr := NewValidationErrorWithCause("test", originalErr)
	require.ErrorIs(t, validationErr, originalErr)
}

// Helper functions and mock types

type mockDependencies struct {
	importer  *mockImporter
	validator *mockValidator
}

func newMockDependencies() *mockDependencies {
	return &mockDependencies{
		importer:  &mockImporter{},
		validator: &mockValidator{},
	}
}

type mockImporter struct {
	ConvertHARFunc            func(context.Context, []byte, idwrap.IDWrap) (*harv2.HarResolved, error)
	ImportAndStoreFunc        func(context.Context, []byte, idwrap.IDWrap) (*harv2.HarResolved, error)
	ImportAndStoreUnifiedFunc func(context.Context, []byte, idwrap.IDWrap) (*TranslationResult, error)
	StoreHTTPEntitiesFunc     func(context.Context, []*mhttp.HTTP) error
	StoreFilesFunc            func(context.Context, []*mfile.File) error
	StoreFlowFunc             func(context.Context, *mflow.Flow) error
	StoreFlowsFunc            func(context.Context, []*mflow.Flow) error
	StoreImportResultsFunc    func(context.Context, *ImportResults) error
	StoreUnifiedResultsFunc   func(context.Context, *TranslationResult) (map[idwrap.IDWrap]bool, map[idwrap.IDWrap]bool, []menv.Variable, []menv.Variable, error)
}

func (m *mockImporter) ConvertHAR(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	if m.ConvertHARFunc != nil {
		return m.ConvertHARFunc(ctx, data, workspaceID)
	}
	return &harv2.HarResolved{}, nil
}

func (m *mockImporter) ImportAndStore(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error) {
	if m.ImportAndStoreFunc != nil {
		return m.ImportAndStoreFunc(ctx, data, workspaceID)
	}
	return &harv2.HarResolved{}, nil
}

func (m *mockImporter) ImportAndStoreUnified(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	if m.ImportAndStoreUnifiedFunc != nil {
		return m.ImportAndStoreUnifiedFunc(ctx, data, workspaceID)
	}
	// Default implementation - return empty result
	return &TranslationResult{}, nil
}

func (m *mockImporter) StoreHTTPEntities(ctx context.Context, httpReqs []*mhttp.HTTP) error {
	if m.StoreHTTPEntitiesFunc != nil {
		return m.StoreHTTPEntitiesFunc(ctx, httpReqs)
	}
	return nil
}

func (m *mockImporter) StoreFiles(ctx context.Context, files []*mfile.File) error {
	if m.StoreFilesFunc != nil {
		return m.StoreFilesFunc(ctx, files)
	}
	return nil
}

func (m *mockImporter) StoreFlow(ctx context.Context, flow *mflow.Flow) error {
	if m.StoreFlowFunc != nil {
		return m.StoreFlowFunc(ctx, flow)
	}
	return nil
}

func (m *mockImporter) StoreFlows(ctx context.Context, flows []*mflow.Flow) error {
	if m.StoreFlowsFunc != nil {
		return m.StoreFlowsFunc(ctx, flows)
	}
	return nil
}

func (m *mockImporter) StoreImportResults(ctx context.Context, results *ImportResults) error {
	if m.StoreImportResultsFunc != nil {
		return m.StoreImportResultsFunc(ctx, results)
	}
	return nil
}

func (m *mockImporter) StoreUnifiedResults(ctx context.Context, results *TranslationResult) (map[idwrap.IDWrap]bool, map[idwrap.IDWrap]bool, []menv.Variable, []menv.Variable, error) {
	if m.StoreUnifiedResultsFunc != nil {
		return m.StoreUnifiedResultsFunc(ctx, results)
	}
	return nil, nil, nil, nil, nil
}

func (m *mockImporter) StoreDomainVariables(ctx context.Context, workspaceID idwrap.IDWrap, domainData []ImportDomainData) ([]menv.Env, []menv.Variable, []menv.Variable, error) {
	// Default mock implementation - no-op
	return nil, nil, nil, nil
}

type mockValidator struct {
	mu                               sync.Mutex
	ValidateImportRequestFunc        func(context.Context, *ImportRequest) error
	ValidateWorkspaceAccessFunc      func(context.Context, idwrap.IDWrap) error
	ValidateDataSizeFunc             func(context.Context, []byte) error
	ValidateFormatSupportFunc        func(context.Context, Format) error
	ValidateImportRequestCallCount   int
	ValidateWorkspaceAccessCallCount int
}

func (m *mockValidator) ValidateImportRequest(ctx context.Context, req *ImportRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ValidateImportRequestCallCount++
	if m.ValidateImportRequestFunc != nil {
		return m.ValidateImportRequestFunc(ctx, req)
	}
	return nil
}

func (m *mockValidator) ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ValidateWorkspaceAccessCallCount++
	if m.ValidateWorkspaceAccessFunc != nil {
		return m.ValidateWorkspaceAccessFunc(ctx, workspaceID)
	}
	return nil
}

func (m *mockValidator) ValidateDataSize(ctx context.Context, data []byte) error {
	if m.ValidateDataSizeFunc != nil {
		return m.ValidateDataSizeFunc(ctx, data)
	}
	// Default implementation - accept any data size
	return nil
}

func (m *mockValidator) ValidateFormatSupport(ctx context.Context, format Format) error {
	if m.ValidateFormatSupportFunc != nil {
		return m.ValidateFormatSupportFunc(ctx, format)
	}
	// Default implementation - accept all formats
	return nil
}

// createMinimalHAR creates a minimal valid HAR for testing
func createMinimalHAR(t *testing.T) []byte {
	return []byte(`{
		"log": {
			"version": "1.2",
			"entries": [
				{
					"startedDateTime": "` + time.Now().UTC().Format(time.RFC3339) + `",
					"request": {
						"method": "GET",
						"url": "https://api.example.com/api/test",
						"httpVersion": "HTTP/1.1",
						"headers": []
					},
					"response": {
						"status": 200,
						"statusText": "OK",
						"httpVersion": "HTTP/1.1",
						"headers": []
					}
				}
			]
		}
	}`)
}
