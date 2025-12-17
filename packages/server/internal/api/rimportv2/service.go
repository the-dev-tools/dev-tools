//nolint:revive // exported
// Package rimportv2 provides a modern HAR import service with TypeSpec compliance.
// It implements a simple, maintainable architecture with dependency injection for core services,
// functional options pattern for configuration, and comprehensive error handling for local development tool workflows.
package rimportv2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/model/mnnode"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
	"the-dev-tools/server/pkg/model/mvar"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

// Error types moved from errors.go

// Common errors for the rimportv2 service
var (
	ErrInvalidHARFormat  = errors.New("invalid HAR format")
	ErrPermissionDenied  = errors.New("permission denied")
	ErrStorageFailed     = errors.New("storage operation failed")
	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrFormatDetection   = errors.New("format detection failed")
	ErrUnsupportedFormat = errors.New("unsupported format")
	ErrInvalidData       = errors.New("invalid data provided")
	ErrTranslationFailed = errors.New("translation failed")
	ErrValidationFailed  = errors.New("validation failed")
	ErrEmptyData         = errors.New("empty data provided")
	ErrDataTooLarge      = errors.New("data exceeds size limit")
	ErrTimeout           = errors.New("operation timed out")
)

// ValidationError represents an input validation error
type ValidationError struct {
	Field   string
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Errorf("validation failed for field '%s': %w", e.Field, e.Err).Error()
	}
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) error {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewValidationErrorWithCause creates a new validation error with an underlying cause
func NewValidationErrorWithCause(field string, cause error) error {
	return &ValidationError{
		Field: field,
		Err:   cause,
	}
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	var validationErr *ValidationError
	return errors.As(err, &validationErr)
}

// Interface definitions moved from interfaces.go

// Importer handles the complete import pipeline: format detection, processing and storage
type Importer interface {
	// Process and store HAR data with modern models (legacy compatibility)
	ImportAndStore(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*harv2.HarResolved, error)
	// Process and store any supported format with automatic detection
	ImportAndStoreUnified(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error)
	// Store individual entity types
	StoreHTTPEntities(ctx context.Context, httpReqs []*mhttp.HTTP) error
	StoreFiles(ctx context.Context, files []*mfile.File) error
	StoreFlow(ctx context.Context, flow *mflow.Flow) error
	StoreFlows(ctx context.Context, flows []*mflow.Flow) error
	// Store complete import results atomically
	StoreImportResults(ctx context.Context, results *ImportResults) error
	StoreUnifiedResults(ctx context.Context, results *TranslationResult) error
	// Store domain-to-variable mappings to all existing environments
	// Returns created environments (if a default was created), created variables, and updated variables
	StoreDomainVariables(ctx context.Context, workspaceID idwrap.IDWrap, domainData []ImportDomainData) (createdEnvs []menv.Env, createdVars []mvar.Var, updatedVars []mvar.Var, err error)
}

// Validator handles input validation for import requests
type Validator interface {
	ValidateImportRequest(ctx context.Context, req *ImportRequest) error
	ValidateWorkspaceAccess(ctx context.Context, workspaceID idwrap.IDWrap) error
	ValidateDataSize(ctx context.Context, data []byte) error
	ValidateFormatSupport(ctx context.Context, format Format) error
}

// ImportConstraints defines validation constraints for import operations
type ImportConstraints struct {
	MaxDataSizeBytes int64         // Maximum size of import data
	SupportedFormats []Format      // List of supported formats
	AllowedMimeTypes []string      // Allowed MIME types for file uploads
	Timeout          time.Duration // Operation timeout
}

// DefaultConstraints returns sensible default constraints
func DefaultConstraints() *ImportConstraints {
	return &ImportConstraints{
		MaxDataSizeBytes: 50 * 1024 * 1024, // 50MB
		SupportedFormats: []Format{FormatHAR, FormatYAML, FormatJSON, FormatCURL, FormatPostman},
		AllowedMimeTypes: []string{
			"application/json",
			"application/har",
			"text/yaml",
			"application/x-yaml",
			"text/plain",
			"application/octet-stream",
		},
		Timeout: 30 * time.Minute,
	}
}

// ImportResults represents the complete results of an import operation
type ImportResults struct {
	Flow     *mflow.Flow
	HTTPReqs []*mhttp.HTTP
	Files    []*mfile.File

	HTTPHeaders        []*mhttp.HTTPHeader
	HTTPSearchParams   []*mhttp.HTTPSearchParam
	HTTPBodyForms      []*mhttp.HTTPBodyForm
	HTTPBodyUrlEncoded []*mhttp.HTTPBodyUrlencoded
	HTTPBodyRaws       []*mhttp.HTTPBodyRaw
	HTTPAsserts        []*mhttp.HTTPAssert

	// Flow-specific entities
	Nodes        []mnnode.MNode
	RequestNodes []mnrequest.MNRequest
	NoOpNodes    []mnnoop.NoopNode
	Edges        []edge.Edge

	// Environment variables created during import (for domain-to-variable mappings)
	CreatedEnvs []menv.Env
	CreatedVars []mvar.Var
	UpdatedVars []mvar.Var

	Domains     []string
	WorkspaceID idwrap.IDWrap
	MissingData ImportMissingDataKind
}

// ImportRequest represents the incoming import request with domain data
type ImportRequest struct {
	WorkspaceID           idwrap.IDWrap
	Name                  string
	Data                  []byte
	TextData              string
	DomainData            []ImportDomainData
	DomainDataWasProvided bool // True if domainData was explicitly provided (even if empty array)
}

// ImportResponse represents the response to an import request
type ImportResponse struct {
	MissingData ImportMissingDataKind
	Domains     []string
}

// ImportMissingDataKind represents the type of missing data
type ImportMissingDataKind int32

const (
	ImportMissingDataKind_UNSPECIFIED ImportMissingDataKind = 0
	ImportMissingDataKind_DOMAIN      ImportMissingDataKind = 1
)

// ImportDomainData represents domain variable configuration
type ImportDomainData struct {
	Enabled  bool
	Domain   string
	Variable string
}

// ServiceOption configures the Service during construction
type ServiceOption func(*Service)

// WithTimeout sets the processing timeout for HAR operations
func WithTimeout(timeout time.Duration) ServiceOption {
	return func(s *Service) {
		s.timeout = timeout
	}
}

// WithLogger sets a custom logger
func WithLogger(logger *slog.Logger) ServiceOption {
	return func(s *Service) {
		s.logger = logger
	}
}

// WithHTTPService sets the HTTP service for the service (required for HAR import overwrite detection)
func WithHTTPService(httpService *shttp.HTTPService) ServiceOption {
	return func(s *Service) {
		// Re-initialize the translator registry with the HTTP service
		s.translatorRegistry = NewTranslatorRegistry(httpService)
	}
}

// Service implements the main business logic for unified import
type Service struct {
	importer           Importer
	validator          Validator
	translatorRegistry *TranslatorRegistry
	logger             *slog.Logger
	timeout            time.Duration
}

// NewService creates a new Service with dependency injection and optional configuration
// Required dependencies: importer and validator
// Optional dependencies can be configured using ServiceOption functions
func NewService(importer Importer, validator Validator, opts ...ServiceOption) *Service {
	// Set sensible defaults
	service := &Service{
		importer:           importer,
		validator:          validator,
		translatorRegistry: NewTranslatorRegistry(nil), // Auto-initialize translator registry without HTTP service (will be overridden if provided)
		timeout:            30 * time.Minute,           // Default timeout for import processing
		logger:             slog.Default(),             // Default logger
	}

	// Apply functional options
	for _, opt := range opts {
		opt(service)
	}

	return service
}

// createFlow creates a simple flow from HTTP requests
func createFlow(ctx context.Context, workspaceID idwrap.IDWrap, name string, httpReqs []*mhttp.HTTP) (*mflow.Flow, error) {
	flow := &mflow.Flow{
		ID:          idwrap.NewNow(),
		WorkspaceID: workspaceID,
		Name:        name,
	}

	return flow, nil
}

// Import processes a HAR file and stores the results using modern models
func (s *Service) Import(ctx context.Context, req *ImportRequest) (*ImportResults, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	s.logger.Info("Starting HAR import",
		"workspace_id", req.WorkspaceID,
		"name", req.Name,
		"data_size", len(req.Data))

	// Validate the import request
	if err := s.validator.ValidateImportRequest(ctx, req); err != nil {
		return nil, NewValidationErrorWithCause("import_request", err)
	}

	// Validate workspace access
	if err := s.validator.ValidateWorkspaceAccess(ctx, req.WorkspaceID); err != nil {
		return nil, err // Return the original error for workspace access issues
	}

	// Process HAR data using the importer
	harResolved, err := s.importer.ImportAndStore(ctx, req.Data, req.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("HAR processing failed: %w", err)
	}

	s.logger.Info("HAR processing completed",
		"workspace_id", req.WorkspaceID,
		"http_requests", len(harResolved.HTTPRequests),
		"files", len(harResolved.Files))

	// Create flow from imported HTTP requests
	httpReqsPtr := make([]*mhttp.HTTP, len(harResolved.HTTPRequests))
	for i := range harResolved.HTTPRequests {
		httpReqsPtr[i] = &harResolved.HTTPRequests[i]
	}

	flow, err := createFlow(ctx, req.WorkspaceID, req.Name, httpReqsPtr)
	if err != nil {
		return nil, fmt.Errorf("flow creation failed: %w", err)
	}

	s.logger.Info("Flow generation completed",
		"workspace_id", req.WorkspaceID,
		"flow_id", flow.ID,
		"flow_name", flow.Name)

	// Prepare import results for storage
	filesPtr := make([]*mfile.File, len(harResolved.Files))
	for i := range harResolved.Files {
		filesPtr[i] = &harResolved.Files[i]
	}

	headersPtr := make([]*mhttp.HTTPHeader, len(harResolved.HTTPHeaders))
	for i := range harResolved.HTTPHeaders {
		headersPtr[i] = &harResolved.HTTPHeaders[i]
	}

	paramsPtr := make([]*mhttp.HTTPSearchParam, len(harResolved.HTTPSearchParams))
	for i := range harResolved.HTTPSearchParams {
		paramsPtr[i] = &harResolved.HTTPSearchParams[i]
	}

	bodyFormsPtr := make([]*mhttp.HTTPBodyForm, len(harResolved.HTTPBodyForms))
	for i := range harResolved.HTTPBodyForms {
		bodyFormsPtr[i] = &harResolved.HTTPBodyForms[i]
	}

	bodyUrlEncodedPtr := make([]*mhttp.HTTPBodyUrlencoded, len(harResolved.HTTPBodyUrlEncoded))
	for i := range harResolved.HTTPBodyUrlEncoded {
		bodyUrlEncodedPtr[i] = &harResolved.HTTPBodyUrlEncoded[i]
	}

	bodyRawsPtr := make([]*mhttp.HTTPBodyRaw, len(harResolved.HTTPBodyRaws))
	for i := range harResolved.HTTPBodyRaws {
		bodyRawsPtr[i] = &harResolved.HTTPBodyRaws[i]
	}

	assertsPtr := make([]*mhttp.HTTPAssert, len(harResolved.HTTPAsserts))
	for i := range harResolved.HTTPAsserts {
		assertsPtr[i] = &harResolved.HTTPAsserts[i]
	}

	// Extract domains from HTTP requests
	domains, err := extractDomains(ctx, httpReqsPtr, s.logger)
	if err != nil {
		return nil, fmt.Errorf("domain extraction failed: %w", err)
	}

	results := &ImportResults{
		Flow:               flow,
		HTTPReqs:           httpReqsPtr,
		Files:              filesPtr,
		HTTPHeaders:        headersPtr,
		HTTPSearchParams:   paramsPtr,
		HTTPBodyForms:      bodyFormsPtr,
		HTTPBodyUrlEncoded: bodyUrlEncodedPtr,
		HTTPBodyRaws:       bodyRawsPtr,
		HTTPAsserts:        assertsPtr,
		Nodes:              harResolved.Nodes,
		RequestNodes:       harResolved.RequestNodes,
		NoOpNodes:          harResolved.NoOpNodes,
		Edges:              harResolved.Edges,
		Domains:            domains,
		WorkspaceID:        req.WorkspaceID,
	}

	// Store all results atomically
	if err := s.importer.StoreImportResults(ctx, results); err != nil {
		s.logger.Error("Storage failed - unexpected internal error",
			"workspace_id", req.WorkspaceID,
			"flow_id", flow.ID,
			"http_requests_count", len(httpReqsPtr),
			"files_count", len(filesPtr),
			"domains_count", len(domains),
			"error", err)
		return nil, fmt.Errorf("storage operation failed: %w", err)
	}

	s.logger.Info("Import completed successfully",
		"workspace_id", req.WorkspaceID,
		"flow_id", flow.ID,
		"http_requests", len(harResolved.HTTPRequests),
		"files", len(harResolved.Files),
		"domains", len(domains))

	// Process domain data if provided
	if len(req.DomainData) > 0 {
		// Process provided domain data for future templating support
		if err := processDomainData(ctx, req.DomainData, req.WorkspaceID, s.logger); err != nil {
			return nil, fmt.Errorf("domain data processing failed: %w", err)
		}

		// Store domain variables (creates default environment if needed)
		createdEnvs, createdVars, updatedVars, err := s.importer.StoreDomainVariables(ctx, req.WorkspaceID, req.DomainData)
		if err != nil {
			return nil, fmt.Errorf("domain variable storage failed: %w", err)
		}

		// Store created/updated envs and vars in results for sync event publishing
		results.CreatedEnvs = createdEnvs
		results.CreatedVars = createdVars
		results.UpdatedVars = updatedVars

		// Apply domain templates to HTTP requests if domain data is provided
		httpReqsPtr, err = applyDomainTemplate(ctx, httpReqsPtr, req.DomainData, s.logger)
		if err != nil {
			return nil, fmt.Errorf("domain template application failed: %w", err)
		}

		// Update the results with templated requests (no need to re-store since already stored above)
		results.HTTPReqs = httpReqsPtr

		s.logger.Info("Applied domain templates",
			"workspace_id", req.WorkspaceID,
			"domain_data_count", len(req.DomainData))
	} else if len(domains) > 0 {
		// We have domains but no domain data was provided, indicate missing domain data
		results.MissingData = ImportMissingDataKind_DOMAIN
		s.logger.Info("Domain data missing for extracted domains",
			"workspace_id", req.WorkspaceID,
			"domain_count", len(domains),
			"domains", domains)
	}

	return results, nil
}

// ImportWithTextData processes HAR data from text format
func (s *Service) ImportWithTextData(ctx context.Context, req *ImportRequest) (*ImportResults, error) {
	s.logger.Debug("Import with text data called",
		"workspace_id", req.WorkspaceID,
		"has_text_data", len(req.TextData) > 0,
		"has_binary_data", len(req.Data) > 0)

	// Convert text data to bytes if provided
	if len(req.Data) == 0 && req.TextData != "" {
		req.Data = []byte(req.TextData)
		s.logger.Debug("Converted text data to binary",
			"workspace_id", req.WorkspaceID,
			"original_length", len(req.TextData),
			"converted_length", len(req.Data))
	}

	return s.Import(ctx, req)
}

// ImportUnified processes any supported format with automatic detection
func (s *Service) ImportUnified(ctx context.Context, req *ImportRequest) (*ImportResults, error) {
	s.logger.Debug("ImportUnified: Starting", "workspace_id", req.WorkspaceID)
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Set up context with timeout
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	s.logger.Info("Starting unified import",
		"workspace_id", req.WorkspaceID,
		"name", req.Name,
		"data_size", len(req.Data))

	// Validate the import request
	if err := s.validator.ValidateImportRequest(ctx, req); err != nil {
		return nil, NewValidationErrorWithCause("import_request", err)
	}

	// Validate workspace access
	if err := s.validator.ValidateWorkspaceAccess(ctx, req.WorkspaceID); err != nil {
		return nil, err // Return the original error for workspace access issues
	}

	s.logger.Debug("ImportUnified: Translating data")
	// Detect format and translate data
	translationResult, err := s.importer.ImportAndStoreUnified(ctx, req.Data, req.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("format detection and translation failed: %w", err)
	}

	s.logger.Info("Translation completed",
		"workspace_id", req.WorkspaceID,
		"detected_format", translationResult.DetectedFormat,
		"http_requests", len(translationResult.HTTPRequests),
		"files", len(translationResult.Files),
		"flows", len(translationResult.Flows))

	// Build results structure
	results := buildImportResults(translationResult, req.WorkspaceID)

	s.logger.Debug("ImportUnified: Checking for missing data",
		"domain_count", len(translationResult.Domains),
		"provided_domains", len(req.DomainData),
		"domain_data_was_provided", req.DomainDataWasProvided)

	// Two-step import flow for domain configuration:
	// 1. First call (DomainDataWasProvided=false): Detect domains, return them to user for configuration (no storage)
	// 2. Second call (DomainDataWasProvided=true):
	//    - If domainData has entries: Create env vars with the mappings, then store
	//    - If domainData is empty []: User chose to skip, just store without env vars
	if !req.DomainDataWasProvided && len(translationResult.Domains) > 0 {
		// First call: domains detected but user hasn't made a choice yet
		// Return early with MissingData set so the client can prompt the user
		results.MissingData = ImportMissingDataKind_DOMAIN
		results.Domains = translationResult.Domains
		s.logger.Info("Domain data missing for extracted domains - returning for user configuration",
			"workspace_id", req.WorkspaceID,
			"domain_count", len(translationResult.Domains),
			"domains", translationResult.Domains)
		return results, nil
	}
	// If DomainDataWasProvided=true, we proceed with import regardless of whether domainData is empty or has values

	// Apply domain-to-variable replacements in URLs before storage
	if len(req.DomainData) > 0 {
		translationResult.HTTPRequests = applyDomainReplacements(translationResult.HTTPRequests, req.DomainData)
		s.logger.Debug("ImportUnified: Applied domain replacements to URLs",
			"http_requests_count", len(translationResult.HTTPRequests),
			"domain_mappings", len(req.DomainData))
	}

	s.logger.Debug("ImportUnified: Storing results")
	// Store all results atomically
	if err := s.importer.StoreUnifiedResults(ctx, translationResult); err != nil {
		s.logger.Error("Storage failed - unexpected internal error",
			"workspace_id", req.WorkspaceID,
			"format", translationResult.DetectedFormat,
			"http_requests_count", len(translationResult.HTTPRequests),
			"files_count", len(translationResult.Files),
			"flows_count", len(translationResult.Flows),
			"domains_count", len(translationResult.Domains),
			"error", err)
		return nil, fmt.Errorf("storage operation failed: %w", err)
	}
	s.logger.Debug("ImportUnified: Storage complete")

	s.logger.Info("Unified import completed successfully",
		"workspace_id", req.WorkspaceID,
		"format", translationResult.DetectedFormat,
		"http_requests", len(translationResult.HTTPRequests),
		"files", len(translationResult.Files),
		"flows", len(translationResult.Flows),
		"domains", len(translationResult.Domains))

	// Process domain data if provided (and we have already stored the initial data)
	if len(req.DomainData) > 0 {
		// Add domain-to-variable mappings to all existing environments
		createdEnvs, createdVars, updatedVars, err := s.importer.StoreDomainVariables(ctx, req.WorkspaceID, req.DomainData)
		if err != nil {
			s.logger.Error("Failed to store domain variables",
				"workspace_id", req.WorkspaceID,
				"error", err)
			return nil, fmt.Errorf("domain variable storage failed: %w", err)
		}

		// Store created/updated envs and vars in results for sync event publishing
		results.CreatedEnvs = createdEnvs
		results.CreatedVars = createdVars
		results.UpdatedVars = updatedVars

		if len(createdVars) > 0 || len(updatedVars) > 0 {
			s.logger.Info("Added domain variables to environments",
				"workspace_id", req.WorkspaceID,
				"created_count", len(createdVars),
				"updated_count", len(updatedVars))
		}
	}

	// Clear domains from result if user made a decision (provided mappings OR skipped)
	// This signals that domain handling is complete
	if req.DomainDataWasProvided {
		results.Domains = nil
	}

	return results, nil
}

// buildImportResults converts TranslationResult to ImportResults
func buildImportResults(tr *TranslationResult, workspaceID idwrap.IDWrap) *ImportResults {
	// Helper to create slice pointers
	httpReqsPtr := make([]*mhttp.HTTP, len(tr.HTTPRequests))
	for i := range tr.HTTPRequests {
		httpReqsPtr[i] = &tr.HTTPRequests[i]
	}

	filesPtr := make([]*mfile.File, len(tr.Files))
	for i := range tr.Files {
		filesPtr[i] = &tr.Files[i]
	}

	headersPtr := make([]*mhttp.HTTPHeader, len(tr.Headers))
	for i := range tr.Headers {
		headersPtr[i] = &tr.Headers[i]
	}

	paramsPtr := make([]*mhttp.HTTPSearchParam, len(tr.SearchParams))
	for i := range tr.SearchParams {
		paramsPtr[i] = &tr.SearchParams[i]
	}

	bodyFormsPtr := make([]*mhttp.HTTPBodyForm, len(tr.BodyForms))
	for i := range tr.BodyForms {
		bodyFormsPtr[i] = &tr.BodyForms[i]
	}

	bodyUrlEncodedPtr := make([]*mhttp.HTTPBodyUrlencoded, len(tr.BodyUrlencoded))
	for i := range tr.BodyUrlencoded {
		bodyUrlEncodedPtr[i] = &tr.BodyUrlencoded[i]
	}

	bodyRawsPtr := make([]*mhttp.HTTPBodyRaw, len(tr.BodyRaw))
	for i := range tr.BodyRaw {
		bodyRawsPtr[i] = &tr.BodyRaw[i]
	}

	assertsPtr := make([]*mhttp.HTTPAssert, len(tr.Asserts))
	for i := range tr.Asserts {
		assertsPtr[i] = &tr.Asserts[i]
	}

	// Only support single flow for now in ImportResults
	var flow *mflow.Flow
	if len(tr.Flows) > 0 {
		flow = &tr.Flows[0]
	}

	return &ImportResults{
		Flow:               flow,
		HTTPReqs:           httpReqsPtr,
		Files:              filesPtr,
		HTTPHeaders:        headersPtr,
		HTTPSearchParams:   paramsPtr,
		HTTPBodyForms:      bodyFormsPtr,
		HTTPBodyUrlEncoded: bodyUrlEncodedPtr,
		HTTPBodyRaws:       bodyRawsPtr,
		HTTPAsserts:        assertsPtr,
		Nodes:              tr.Nodes,
		RequestNodes:       tr.RequestNodes,
		NoOpNodes:          tr.NoOpNodes,
		Edges:              tr.Edges,
		Domains:            tr.Domains,
		WorkspaceID:        workspaceID,
		MissingData:        ImportMissingDataKind_UNSPECIFIED,
	}
}

// ImportUnifiedWithTextData processes any supported format from text with automatic detection
func (s *Service) ImportUnifiedWithTextData(ctx context.Context, req *ImportRequest) (*ImportResults, error) {
	s.logger.Debug("Unified import with text data called",
		"workspace_id", req.WorkspaceID,
		"has_text_data", len(req.TextData) > 0,
		"has_binary_data", len(req.Data) > 0)

	// Convert text data to bytes if provided
	if len(req.Data) == 0 && req.TextData != "" {
		req.Data = []byte(req.TextData)
		s.logger.Debug("Converted text data to binary",
			"workspace_id", req.WorkspaceID,
			"original_length", len(req.TextData),
			"converted_length", len(req.Data))
	}

	return s.ImportUnified(ctx, req)
}

// DetectFormat performs format detection on the provided data
func (s *Service) DetectFormat(ctx context.Context, data []byte) (*DetectionResult, error) {
	if len(data) == 0 {
		return nil, NewValidationError("data", "empty data provided")
	}

	result, err := s.translatorRegistry.detector.DetectAndValidate(data)
	if err != nil {
		return nil, err
	}
	if result.Format == FormatUnknown {
		return result, fmt.Errorf("unable to detect format: %s", result.Reason)
	}

	return result, nil
}

// GetSupportedFormats returns all supported import formats
func (s *Service) GetSupportedFormats() []Format {
	return s.translatorRegistry.GetSupportedFormats()
}

// ValidateFormat validates data for a specific format
func (s *Service) ValidateFormat(ctx context.Context, data []byte, format Format) error {
	if len(data) == 0 {
		return NewValidationError("data", "empty data provided")
	}

	return s.translatorRegistry.ValidateFormat(data, format)
}

// ValidateImportRequestExtended performs comprehensive validation of import requests
func (s *Service) ValidateImportRequestExtended(ctx context.Context, req *ImportRequest, constraints *ImportConstraints) error {
	// Basic validation
	if err := s.validator.ValidateImportRequest(ctx, req); err != nil {
		return err
	}

	// Validate data size
	if err := s.validator.ValidateDataSize(ctx, req.Data); err != nil {
		return err
	}

	// Validate UTF-8 encoding for text data
	if req.TextData != "" && !IsUTF8([]byte(req.TextData)) {
		return NewValidationError("text_data", "text data must be valid UTF-8")
	}

	// Validate binary data encoding
	if len(req.Data) > 0 && !IsUTF8(req.Data) {
		// For binary data, we should validate it's expected binary format
		s.logger.Debug("Binary data detected, skipping UTF-8 validation",
			"workspace_id", req.WorkspaceID,
			"data_size", len(req.Data))
	}

	// Detect format early to validate support
	detection, err := s.DetectFormat(ctx, req.Data)
	if err != nil {
		// If format detection fails, we'll let the main import method handle it
		s.logger.Debug("Early format detection failed, will retry in main import",
			"workspace_id", req.WorkspaceID,
			"error", err)
	} else {
		// Validate format support
		if err := s.validator.ValidateFormatSupport(ctx, detection.Format); err != nil {
			return fmt.Errorf("format %s is not supported: %w", detection.Format, err)
		}
	}

	// Validate domain data
	if len(req.DomainData) > 0 {
		if err := s.validateDomainData(req.DomainData); err != nil {
			return NewValidationErrorWithCause("domain_data", err)
		}
	}

	return nil
}

// ValidateAndSanitizeRequest validates and sanitizes import request data
func (s *Service) ValidateAndSanitizeRequest(ctx context.Context, req *ImportRequest) (*ImportRequest, error) {
	// Create a copy to avoid modifying the original
	sanitized := &ImportRequest{
		WorkspaceID: req.WorkspaceID,
		Name:        strings.TrimSpace(req.Name),
		Data:        req.Data,
		TextData:    strings.TrimSpace(req.TextData),
		DomainData:  make([]ImportDomainData, len(req.DomainData)),
	}

	// Copy and sanitize domain data
	for i, dd := range req.DomainData {
		sanitized.DomainData[i] = ImportDomainData{
			Enabled:  dd.Enabled,
			Domain:   strings.ToLower(strings.TrimSpace(dd.Domain)),
			Variable: strings.TrimSpace(dd.Variable),
		}
	}

	// Validate the sanitized request
	if err := s.ValidateImportRequestExtended(ctx, sanitized, nil); err != nil {
		return nil, err
	}

	return sanitized, nil
}