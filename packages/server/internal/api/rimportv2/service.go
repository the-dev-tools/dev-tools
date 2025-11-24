// Package rimportv2 provides a modern HAR import service with TypeSpec compliance.
// It implements a simple, maintainable architecture with dependency injection for core services,
// functional options pattern for configuration, and comprehensive error handling for local development tool workflows.
package rimportv2

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/harv2"
)

// Error types moved from errors.go

// Common errors for the rimportv2 service
var (
	ErrInvalidHARFormat     = errors.New("invalid HAR format")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrStorageFailed        = errors.New("storage operation failed")
	ErrWorkspaceNotFound    = errors.New("workspace not found")
	ErrFormatDetection      = errors.New("format detection failed")
	ErrUnsupportedFormat    = errors.New("unsupported format")
	ErrInvalidData          = errors.New("invalid data provided")
	ErrTranslationFailed    = errors.New("translation failed")
	ErrValidationFailed     = errors.New("validation failed")
	ErrEmptyData           = errors.New("empty data provided")
	ErrDataTooLarge        = errors.New("data exceeds size limit")
	ErrTimeout             = errors.New("operation timed out")
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
	MaxDataSizeBytes int64      // Maximum size of import data
	SupportedFormats []Format   // List of supported formats
	AllowedMimeTypes []string   // Allowed MIME types for file uploads
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
	Flow        *mflow.Flow
	HTTPReqs    []*mhttp.HTTP
	Files       []*mfile.File
	
	HTTPHeaders        []*mhttp.HTTPHeader
	HTTPSearchParams   []*mhttp.HTTPSearchParam
	HTTPBodyForms      []*mhttp.HTTPBodyForm
	HTTPBodyUrlEncoded []*mhttp.HTTPBodyUrlencoded
	HTTPBodyRaws       []*mhttp.HTTPBodyRaw

	Domains     []string
	WorkspaceID idwrap.IDWrap
	MissingData ImportMissingDataKind
}

// ImportRequest represents the incoming import request with domain data
type ImportRequest struct {
	WorkspaceID idwrap.IDWrap
	Name        string
	Data        []byte
	TextData    string
	DomainData  []ImportDomainData
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

// Service implements the main business logic for unified import
type Service struct {
	importer         Importer
	validator        Validator
	translatorRegistry *TranslatorRegistry
	logger           *slog.Logger
	timeout          time.Duration
}

// NewService creates a new Service with dependency injection and optional configuration
// Required dependencies: importer and validator
// Optional dependencies can be configured using ServiceOption functions
func NewService(importer Importer, validator Validator, opts ...ServiceOption) *Service {
	// Set sensible defaults
	service := &Service{
		importer:           importer,
		validator:          validator,
		translatorRegistry: NewTranslatorRegistry(), // Auto-initialize translator registry
		timeout:            30 * time.Minute, // Default timeout for import processing
		logger:             slog.Default(),   // Default logger
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

// extractDomains extracts unique domains from HTTP requests, filtering for XHR-like requests
func extractDomains(ctx context.Context, httpReqs []*mhttp.HTTP, logger *slog.Logger) ([]string, error) {
	domains := make(map[string]struct{}, len(httpReqs))

	for _, req := range httpReqs {
		if req == nil {
			continue
		}

		// Skip non-XHR-like requests - replicate logic from thar.IsXHRRequest
		if !isXHRRequest(req) {
			continue
		}

		domain, err := extractDomainFromURL(req.Url)
		if err != nil {
			continue // Skip URLs that can't be parsed - expected condition
		}

		if domain != "" {
			domains[strings.ToLower(domain)] = struct{}{}
		}
	}

	// Convert to sorted slice
	result := make([]string, 0, len(domains))
	for domain := range domains {
		result = append(result, domain)
	}
	sort.Strings(result)

	logger.Debug("Extracted domains from HTTP requests",
		"total_requests", len(httpReqs),
		"xhr_requests", countXHRRequests(httpReqs),
		"unique_domains", len(result))

	return result, nil
}

// processDomainData processes domain variable configurations for future templating support
func processDomainData(ctx context.Context, domainData []ImportDomainData, workspaceID idwrap.IDWrap, logger *slog.Logger) error {
	// For now, this is a placeholder for future domain variable processing
	// This method will be used to set up domain-to-variable mappings for templating
	if len(domainData) == 0 {
		return nil
	}

	logger.Debug("Processing domain data",
		"workspace_id", workspaceID,
		"domain_count", len(domainData))

	// Validate domain data
	for _, dd := range domainData {
		if dd.Domain == "" {
			return fmt.Errorf("domain data entry missing domain")
		}
		if dd.Variable == "" {
			return fmt.Errorf("domain data entry for domain '%s' missing variable name", dd.Domain)
		}
	}

	return nil
}

// applyDomainTemplate applies domain variable substitution to HTTP requests
func applyDomainTemplate(ctx context.Context, httpReqs []*mhttp.HTTP, domainData []ImportDomainData, logger *slog.Logger) ([]*mhttp.HTTP, error) {
	if len(domainData) == 0 {
		return httpReqs, nil
	}

	// Create domain-to-variable mapping
	domainMap := make(map[string]string, len(domainData))
	for _, dd := range domainData {
		if dd.Enabled {
			domainMap[strings.ToLower(dd.Domain)] = sanitizeVariableName(dd.Variable)
		}
	}

	if len(domainMap) == 0 {
		return httpReqs, nil
	}

	// Create a copy of requests to avoid modifying originals
	result := make([]*mhttp.HTTP, len(httpReqs))
	copy(result, httpReqs)

	// Apply domain variable substitution
	for i, req := range result {
		if req == nil {
			continue
		}

		parsedURL, err := url.Parse(req.Url)
		if err != nil {
			continue // Skip URLs that can't be parsed - expected condition
		}

		variable, exists := domainMap[strings.ToLower(parsedURL.Host)]
		if !exists || variable == "" {
			continue
		}

		suffix := buildURLSuffix(parsedURL)
		templatedURL := buildTemplatedURL(variable, suffix)

		// Create a copy of the request with the templated URL
		updatedReq := *req
		updatedReq.Url = templatedURL
		result[i] = &updatedReq

		logger.Debug("Applied domain template",
			"original_url", req.Url,
			"templated_url", templatedURL,
			"variable", variable)
	}

	logger.Debug("Applied domain templates to HTTP requests",
		"total_requests", len(httpReqs),
		"templated_requests", countTemplatedRequests(result, httpReqs))

	return result, nil
}

// Helper functions for domain processing

// isXHRRequest determines if a request should be treated as an XHR request
// This replicates the logic from thar.IsXHRRequest for the modern HTTP model
func isXHRRequest(req *mhttp.HTTP) bool {
	if req == nil {
		return false
	}

	// For modern HTTP model, we need to check if this would be an XHR request
	// Since we don't have the original request headers, we'll use URL patterns
	// that are commonly associated with XHR requests

	parsedURL, err := url.Parse(req.Url)
	if err != nil {
		return false
	}

	// Common API path patterns
	path := strings.ToLower(parsedURL.Path)

	// Check for common API indicators
	apiIndicators := []string{
		"/api/", "/v1/", "/v2/", "/v3/",
		".json", ".xml", "/graphql", "/rest",
		"/ajax/", "/xhr/",
	}

	for _, indicator := range apiIndicators {
		if strings.Contains(path, indicator) {
			return true
		}
	}

	// Check hostname for API patterns
	host := strings.ToLower(parsedURL.Hostname())
	hostnameAPIIndicators := []string{
		"api.", "api-", ".api", // API subdomain patterns
		"rest.", "rest-", ".rest", // REST API patterns
		"graph.", "graph-", ".graph", // GraphQL patterns
	}

	for _, indicator := range hostnameAPIIndicators {
		if strings.Contains(host, indicator) {
			return true
		}
	}

	// Check for HTTP methods commonly used in XHR
	xhrMethods := map[string]bool{
		"POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	}

	if xhrMethods[strings.ToUpper(req.Method)] {
		return true
	}

	// Check for query parameters common in XHR requests
	if strings.Contains(strings.ToLower(parsedURL.RawQuery), "callback=") ||
		strings.Contains(strings.ToLower(parsedURL.RawQuery), "jsonp=") {
		return true
	}

	return false
}

// extractDomainFromURL extracts the domain from a URL string
func extractDomainFromURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL '%s': %w", rawURL, err)
	}

	return parsedURL.Host, nil
}

// buildURLSuffix builds the path, query, and fragment part of a URL
func buildURLSuffix(parsedURL *url.URL) string {
	if parsedURL == nil {
		return ""
	}

	var suffix strings.Builder

	// Add path
	if parsedURL.Path == "" {
		if parsedURL.Opaque != "" {
			suffix.WriteString(parsedURL.Opaque)
		}
	} else {
		if parsedURL.Path != "/" {
			suffix.WriteString(parsedURL.Path)
		}
	}

	// Add query
	if parsedURL.RawQuery != "" {
		if suffix.Len() == 0 {
			suffix.WriteString("?")
		} else {
			suffix.WriteString("?")
		}
		suffix.WriteString(parsedURL.RawQuery)
	}

	// Add fragment
	if parsedURL.Fragment != "" {
		if suffix.Len() == 0 {
			suffix.WriteString("#")
		} else {
			suffix.WriteString("#")
		}
		suffix.WriteString(parsedURL.Fragment)
	}

	return suffix.String()
}

// countXHRRequests counts XHR-like requests for logging
func countXHRRequests(httpReqs []*mhttp.HTTP) int {
	count := 0
	for _, req := range httpReqs {
		if req != nil && isXHRRequest(req) {
			count++
		}
	}
	return count
}

// countTemplatedRequests counts how many requests were modified with templates
func countTemplatedRequests(templated, original []*mhttp.HTTP) int {
	count := 0
	for i := range templated {
		if i >= len(original) {
			break
		}
		if templated[i] != nil && original[i] != nil &&
			templated[i].Url != original[i].Url {
			count++
		}
	}
	return count
}

// sanitizeVariableName cleans up variable names for safe use in templates
func sanitizeVariableName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.Trim(trimmed, "{}\t \n")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	return trimmed
}

// buildTemplatedURL creates a templated URL using the variable and suffix
func buildTemplatedURL(variable, suffix string) string {
	if variable == "" {
		return suffix
	}
	if suffix == "" {
		return fmt.Sprintf("{{%s}}", variable)
	}
	if !strings.HasPrefix(suffix, "/") && !strings.HasPrefix(suffix, "?") && !strings.HasPrefix(suffix, "#") {
		suffix = "/" + suffix
	}
	return fmt.Sprintf("{{%s}}%s", variable, suffix)
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

	// Helper to create slice pointers
	httpReqsPtr := make([]*mhttp.HTTP, len(translationResult.HTTPRequests))
	for i := range translationResult.HTTPRequests {
		httpReqsPtr[i] = &translationResult.HTTPRequests[i]
	}

	filesPtr := make([]*mfile.File, len(translationResult.Files))
	for i := range translationResult.Files {
		filesPtr[i] = &translationResult.Files[i]
	}

	headersPtr := make([]*mhttp.HTTPHeader, len(translationResult.Headers))
	for i := range translationResult.Headers {
		headersPtr[i] = &translationResult.Headers[i]
	}

	paramsPtr := make([]*mhttp.HTTPSearchParam, len(translationResult.SearchParams))
	for i := range translationResult.SearchParams {
		paramsPtr[i] = &translationResult.SearchParams[i]
	}

	bodyFormsPtr := make([]*mhttp.HTTPBodyForm, len(translationResult.BodyForms))
	for i := range translationResult.BodyForms {
		bodyFormsPtr[i] = &translationResult.BodyForms[i]
	}

	bodyUrlEncodedPtr := make([]*mhttp.HTTPBodyUrlencoded, len(translationResult.BodyUrlencoded))
	for i := range translationResult.BodyUrlencoded {
		bodyUrlEncodedPtr[i] = &translationResult.BodyUrlencoded[i]
	}

	// BodyRaw is already []*mhttp.HTTPBodyRaw in TranslationResult
	bodyRawsPtr := translationResult.BodyRaw

	// Only support single flow for now in ImportResults
	var flow *mflow.Flow
	if len(translationResult.Flows) > 0 {
		flow = &translationResult.Flows[0]
	}

	// Build results structure early to check for missing data
	results := &ImportResults{
		Flow:               flow,
		HTTPReqs:           httpReqsPtr,
		Files:              filesPtr,
		HTTPHeaders:        headersPtr,
		HTTPSearchParams:   paramsPtr,
		HTTPBodyForms:      bodyFormsPtr,
		HTTPBodyUrlEncoded: bodyUrlEncodedPtr,
		HTTPBodyRaws:       bodyRawsPtr,
		Domains:            translationResult.Domains,
		WorkspaceID:        req.WorkspaceID,
		MissingData:        ImportMissingDataKind_UNSPECIFIED,
	}

	s.logger.Debug("ImportUnified: Checking for missing data", "domain_count", len(translationResult.Domains), "provided_domains", len(req.DomainData))
	// Check for missing domain data BEFORE storage
	if len(req.DomainData) == 0 && len(translationResult.Domains) > 0 {
		// We have domains but no domain data was provided
		// Return early with MissingData set so the client can prompt the user
		// Do NOT store data yet
		results.MissingData = ImportMissingDataKind_DOMAIN
		s.logger.Info("Domain data missing for extracted domains - returning early without storage",
			"workspace_id", req.WorkspaceID,
			"domain_count", len(translationResult.Domains),
			"domains", translationResult.Domains)
		return results, nil
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
		// Process provided domain data for future templating support
		if err := processDomainData(ctx, req.DomainData, req.WorkspaceID, s.logger); err != nil {
			return nil, fmt.Errorf("domain data processing failed: %w", err)
		}

		s.logger.Info("Applied domain templates",
			"workspace_id", req.WorkspaceID,
			"domain_data_count", len(req.DomainData))

		// Clear domains from result as they are now handled and we don't want to return them on success
		results.Domains = nil
	}

	return results, nil
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
	// Apply default constraints if none provided
	if constraints == nil {
		constraints = DefaultConstraints()
	}

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

// validateDomainData validates domain variable configuration
func (s *Service) validateDomainData(domainData []ImportDomainData) error {
	domainMap := make(map[string]string)

	for _, dd := range domainData {
		// Validate domain format
		if dd.Domain == "" {
			return fmt.Errorf("domain cannot be empty")
		}

		// Basic domain validation
		if !s.isValidDomain(dd.Domain) {
			return fmt.Errorf("invalid domain format: %s", dd.Domain)
		}

		// Validate variable name
		if dd.Variable == "" {
			return fmt.Errorf("variable name cannot be empty for domain: %s", dd.Domain)
		}

		// Check for duplicate domains
		if existingVar, exists := domainMap[dd.Domain]; exists {
			return fmt.Errorf("duplicate domain configuration: %s (variables: %s, %s)",
				dd.Domain, existingVar, dd.Variable)
		}

		domainMap[dd.Domain] = dd.Variable
	}

	return nil
}

// isValidDomain performs basic domain validation
func (s *Service) isValidDomain(domain string) bool {
	if domain == "" {
		return false
	}

	// Basic checks - no spaces, reasonable length
	if len(domain) > 253 || strings.ContainsAny(domain, " \t\n\r") {
		return false
	}

	// Could add more sophisticated domain validation here if needed
	return true
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

