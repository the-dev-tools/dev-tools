// Package rimportv2 provides a modern unified import service with TypeSpec compliance.
// It implements automatic format detection and supports multiple import formats.
//
//nolint:revive // exported
package rimportv2

import (
	"context"
	"fmt"
	"strings"
	"time"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/menv"
	"the-dev-tools/server/pkg/model/mfile"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/service/shttp"
	"the-dev-tools/server/pkg/translate/harv2"
	"the-dev-tools/server/pkg/translate/tcurlv2"
	"the-dev-tools/server/pkg/translate/tpostmanv2"
	yamlflowsimplev2 "the-dev-tools/server/pkg/translate/yamlflowsimplev2"
)

// TranslationResult represents the unified result from any translator
type TranslationResult struct {
	// Core entities
	HTTPRequests []mhttp.HTTP
	Files        []mfile.File // ALL files: HTTP, folders, AND flow files (ContentType=Flow)
	Flows        []mflow.Flow

	// Associated HTTP data (headers, params, bodies)
	Headers        []mhttp.HTTPHeader
	SearchParams   []mhttp.HTTPSearchParam
	BodyForms      []mhttp.HTTPBodyForm
	BodyUrlencoded []mhttp.HTTPBodyUrlencoded
	BodyRaw        []mhttp.HTTPBodyRaw
	Asserts        []mhttp.HTTPAssert

	// Flow-specific entities
	Nodes        []mflow.Node
	RequestNodes []mflow.NodeRequest
	Edges        []mflow.Edge

	// Variables (collection or environment level)
	Variables []menv.Variable

	// Metadata
	DetectedFormat Format
	Domains        []string
	ProcessedAt    int64
	WorkspaceID    idwrap.IDWrap
}

// Translator defines the unified interface for all format translators
type Translator interface {
	// Translate converts input data to the unified TranslationResult format
	Translate(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error)

	// GetFormat returns the format this translator handles
	GetFormat() Format

	// Validate checks if the data is valid for this format
	Validate(data []byte) error
}

// TranslatorRegistry manages all available translators
type TranslatorRegistry struct {
	translators map[Format]Translator
	detector    *FormatDetector
}

// NewTranslatorRegistry creates a new registry with all available translators
func NewTranslatorRegistry(httpService *shttp.HTTPService) *TranslatorRegistry {
	registry := &TranslatorRegistry{
		translators: make(map[Format]Translator),
		detector:    NewFormatDetector(),
	}

	// Register all available translators
	registry.RegisterTranslator(NewHARTranslator(httpService))
	registry.RegisterTranslator(NewYAMLTranslator())
	registry.RegisterTranslator(NewCURLTranslator())
	registry.RegisterTranslator(NewPostmanTranslator())
	registry.RegisterTranslator(NewJSONTranslator())

	return registry
}

// RegisterTranslator adds a translator to the registry
func (r *TranslatorRegistry) RegisterTranslator(translator Translator) {
	r.translators[translator.GetFormat()] = translator
}

// GetTranslator returns the translator for the given format
func (r *TranslatorRegistry) GetTranslator(format Format) (Translator, error) {
	translator, exists := r.translators[format]
	if !exists {
		return nil, fmt.Errorf("no translator available for format: %v", format)
	}
	return translator, nil
}

// DetectAndTranslate detects format and translates data in one step
func (r *TranslatorRegistry) DetectAndTranslate(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	// Detect format
	detection, err := r.detector.DetectAndValidate(data)
	if err != nil {
		return nil, fmt.Errorf("format detection failed: %w", err)
	}

	// Get appropriate translator
	translator, err := r.GetTranslator(detection.Format)
	if err != nil {
		return nil, fmt.Errorf("translator retrieval failed: %w", err)
	}

	// Translate data
	result, err := translator.Translate(ctx, data, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("translation failed for %s format: %w", detection.Format, err)
	}

	// Set detected format in result
	result.DetectedFormat = detection.Format
	result.ProcessedAt = time.Now().UnixMilli()
	result.WorkspaceID = workspaceID

	return result, nil
}

// GetSupportedFormats returns all supported formats
func (r *TranslatorRegistry) GetSupportedFormats() []Format {
	formats := make([]Format, 0, len(r.translators))
	for format := range r.translators {
		formats = append(formats, format)
	}
	return formats
}

// ValidateFormat validates data for a specific format
func (r *TranslatorRegistry) ValidateFormat(data []byte, format Format) error {
	translator, err := r.GetTranslator(format)
	if err != nil {
		return fmt.Errorf("no translator for format %v: %w", format, err)
	}
	return translator.Validate(data)
}

// HARTranslator implements Translator for HAR format
type HARTranslator struct {
	detector    *FormatDetector
	httpService *shttp.HTTPService
}

// NewHARTranslator creates a new HAR translator
func NewHARTranslator(httpService *shttp.HTTPService) *HARTranslator {
	return &HARTranslator{
		detector:    NewFormatDetector(),
		httpService: httpService,
	}
}

func (t *HARTranslator) GetFormat() Format {
	return FormatHAR
}

func (t *HARTranslator) Validate(data []byte) error {
	return t.detector.ValidateFormat(data, FormatHAR)
}

func (t *HARTranslator) Translate(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	// Parse HAR data
	har, err := harv2.ConvertRaw(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HAR data: %w", err)
	}

	// Convert to modern models without overwrite detection (always create new)
	resolved, err := harv2.ConvertHAR(har, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HAR: %w", err)
	}

	// Convert to unified result
	// Files contains ALL files: HTTP, folders, AND flow files (harv2 creates flow files)
	result := &TranslationResult{
		HTTPRequests:   resolved.HTTPRequests,
		Files:          resolved.Files, // All files including flow files
		Flows:          []mflow.Flow{resolved.Flow},
		Headers:        resolved.HTTPHeaders,
		SearchParams:   resolved.HTTPSearchParams,
		BodyForms:      resolved.HTTPBodyForms,
		BodyUrlencoded: resolved.HTTPBodyUrlEncoded,
		Asserts:        resolved.HTTPAsserts,
		Nodes:          resolved.Nodes,
		RequestNodes:   resolved.RequestNodes,
		Edges:          resolved.Edges,
		ProcessedAt:    time.Now().UnixMilli(),
	}

	// Copy body raw data
	if len(resolved.HTTPBodyRaws) > 0 {
		result.BodyRaw = resolved.HTTPBodyRaws
	}

	// Extract domains from HTTP requests
	result.Domains = extractDomainsFromHTTP(result.HTTPRequests)

	return result, nil
}

// YAMLTranslator implements Translator for YAML flow format
type YAMLTranslator struct {
	detector *FormatDetector
}

// NewYAMLTranslator creates a new YAML translator
func NewYAMLTranslator() *YAMLTranslator {
	return &YAMLTranslator{
		detector: NewFormatDetector(),
	}
}

func (t *YAMLTranslator) GetFormat() Format {
	return FormatYAML
}

func (t *YAMLTranslator) Validate(data []byte) error {
	return t.detector.ValidateFormat(data, FormatYAML)
}

func (t *YAMLTranslator) Translate(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	// Convert YAML options
	opts := yamlflowsimplev2.ConvertOptionsV2{
		WorkspaceID:       workspaceID,
		GenerateFiles:     true,
		FileOrder:         0,
		EnableCompression: false,
		CompressionType:   0,
	}

	// Convert YAML to modern models
	resolved, err := yamlflowsimplev2.ConvertSimplifiedYAML(data, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML: %w", err)
	}

	// Convert to unified result
	result := &TranslationResult{
		HTTPRequests:   resolved.HTTPRequests,
		Files:          resolved.Files,
		Flows:          resolved.Flows,
		Headers:        resolved.HTTPHeaders,
		SearchParams:   resolved.HTTPSearchParams,
		BodyForms:      resolved.HTTPBodyForms,
		BodyUrlencoded: resolved.HTTPBodyUrlencoded,
		BodyRaw:        resolved.HTTPBodyRaw,
		Nodes:          resolved.FlowNodes,
		RequestNodes:   resolved.FlowRequestNodes,
		Edges:          resolved.FlowEdges,
		ProcessedAt:    time.Now().UnixMilli(),
	}

	// YAML imports don't need domain extraction - they typically already use
	// template variables like {{baseUrl}}. Domain extraction is only for HAR
	// imports where real URLs need to be converted to variables.
	// result.Domains intentionally left empty

	return result, nil
}

// CURLTranslator implements Translator for CURL command format
type CURLTranslator struct {
	detector *FormatDetector
}

// NewCURLTranslator creates a new CURL translator
func NewCURLTranslator() *CURLTranslator {
	return &CURLTranslator{
		detector: NewFormatDetector(),
	}
}

func (t *CURLTranslator) GetFormat() Format {
	return FormatCURL
}

func (t *CURLTranslator) Validate(data []byte) error {
	return t.detector.ValidateFormat(data, FormatCURL)
}

func (t *CURLTranslator) Translate(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	// Convert curl options
	opts := tcurlv2.ConvertCurlOptions{
		WorkspaceID: workspaceID,
	}

	// Convert curl to modern models
	resolved, err := tcurlv2.ConvertCurl(string(data), opts)
	if err != nil {
		return nil, fmt.Errorf("failed to convert curl: %w", err)
	}

	// Convert to unified result
	result := &TranslationResult{
		HTTPRequests:   []mhttp.HTTP{resolved.HTTP},
		Files:          []mfile.File{resolved.File},
		Headers:        resolved.Headers,
		SearchParams:   resolved.SearchParams,
		BodyForms:      resolved.BodyForms,
		BodyUrlencoded: resolved.BodyUrlencoded,
		ProcessedAt:    time.Now().UnixMilli(),
	}

	if resolved.BodyRaw != nil {
		result.BodyRaw = []mhttp.HTTPBodyRaw{*resolved.BodyRaw}
	}

	// Extract domains from HTTP requests
	result.Domains = extractDomainsFromHTTP(result.HTTPRequests)

	return result, nil
}

// PostmanTranslator implements Translator for Postman collection format
type PostmanTranslator struct {
	detector *FormatDetector
}

// NewPostmanTranslator creates a new Postman translator
func NewPostmanTranslator() *PostmanTranslator {
	return &PostmanTranslator{
		detector: NewFormatDetector(),
	}
}

func (t *PostmanTranslator) GetFormat() Format {
	return FormatPostman
}

func (t *PostmanTranslator) Validate(data []byte) error {
	return t.detector.ValidateFormat(data, FormatPostman)
}

func (t *PostmanTranslator) Translate(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	// Convert Postman options
	opts := tpostmanv2.ConvertOptions{
		WorkspaceID: workspaceID,
	}

	// Convert Postman to modern models
	resolved, err := tpostmanv2.ConvertPostmanCollection(data, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to convert Postman collection: %w", err)
	}

	// Convert to unified result
	result := &TranslationResult{
		HTTPRequests:   resolved.HTTPRequests,
		Files:          resolved.Files,
		Headers:        resolved.Headers,
		SearchParams:   resolved.SearchParams,
		BodyForms:      resolved.BodyForms,
		BodyUrlencoded: resolved.BodyUrlencoded,
		BodyRaw:        resolved.BodyRaw,
		Asserts:        resolved.Asserts,
		Flows:          []mflow.Flow{resolved.Flow},
		Nodes:          resolved.Nodes,
		RequestNodes:   resolved.RequestNodes,
		Edges:          resolved.Edges,
		ProcessedAt:    time.Now().UnixMilli(),
	}

	// Map collection variables
	if len(resolved.Variables) > 0 {
		result.Variables = make([]menv.Variable, 0, len(resolved.Variables))
		for i, v := range resolved.Variables {
			result.Variables = append(result.Variables, menv.Variable{
				ID:      idwrap.NewNow(),
				VarKey:  v.Key,
				Value:   v.Value,
				Enabled: true,
				Order:   float64(i + 1),
			})
		}
	}

	// Extract domains from HTTP requests
	result.Domains = extractDomainsFromHTTP(result.HTTPRequests)

	return result, nil
}

// JSONTranslator implements Translator for generic JSON format
type JSONTranslator struct {
	detector *FormatDetector
}

// NewJSONTranslator creates a new JSON translator
func NewJSONTranslator() *JSONTranslator {
	return &JSONTranslator{
		detector: NewFormatDetector(),
	}
}

func (t *JSONTranslator) GetFormat() Format {
	return FormatJSON
}

func (t *JSONTranslator) Validate(data []byte) error {
	return t.detector.ValidateFormat(data, FormatJSON)
}

func (t *JSONTranslator) Translate(ctx context.Context, data []byte, workspaceID idwrap.IDWrap) (*TranslationResult, error) {
	// For generic JSON, we try to interpret it as a single HTTP request
	// This is a best-effort translation since JSON format is not standardized

	// For now, return an error since generic JSON translation requires
	// more specific format knowledge
	return nil, fmt.Errorf("generic JSON translation is not yet implemented. Please use a more specific format (HAR, Postman, etc.)")
}

// extractDomainsFromHTTP extracts unique domains from HTTP requests
func extractDomainsFromHTTP(requests []mhttp.HTTP) []string {
	domainSet := make(map[string]struct{})

	for _, req := range requests {
		if req.Url != "" {
			// Simple domain extraction
			if strings.HasPrefix(req.Url, "http://") {
				domain := strings.TrimPrefix(req.Url, "http://")
				if slashIndex := strings.Index(domain, "/"); slashIndex != -1 {
					domain = domain[:slashIndex]
				}
				domainSet[strings.ToLower(domain)] = struct{}{}
			} else if strings.HasPrefix(req.Url, "https://") {
				domain := strings.TrimPrefix(req.Url, "https://")
				if slashIndex := strings.Index(domain, "/"); slashIndex != -1 {
					domain = domain[:slashIndex]
				}
				domainSet[strings.ToLower(domain)] = struct{}{}
			}
		}
	}

	// Convert to slice
	domains := make([]string, 0, len(domainSet))
	for domain := range domainSet {
		domains = append(domains, domain)
	}

	return domains
}

// TranslationOptions provides options for the translation process
type TranslationOptions struct {
	WorkspaceID       idwrap.IDWrap
	GenerateFiles     bool
	FileOrder         int
	EnableCompression bool
	CompressionType   int8
}

// DefaultTranslationOptions returns sensible default options
func DefaultTranslationOptions(workspaceID idwrap.IDWrap) *TranslationOptions {
	return &TranslationOptions{
		WorkspaceID:       workspaceID,
		GenerateFiles:     true,
		FileOrder:         0,
		EnableCompression: false,
		CompressionType:   0,
	}
}

// WithFiles configures the translation to generate files
func (opts *TranslationOptions) WithFiles(generate bool) *TranslationOptions {
	opts.GenerateFiles = generate
	return opts
}

// WithCompression configures compression options
func (opts *TranslationOptions) WithCompression(enable bool, compressionType int8) *TranslationOptions {
	opts.EnableCompression = enable
	opts.CompressionType = compressionType
	return opts
}

// MergeWithDefaults creates a complete TranslationOptions by merging with defaults
func (opts *TranslationOptions) MergeWithDefaults(workspaceID idwrap.IDWrap) *TranslationOptions {
	if opts == nil {
		return DefaultTranslationOptions(workspaceID)
	}

	result := *opts
	if result.WorkspaceID.Compare(idwrap.IDWrap{}) == 0 {
		result.WorkspaceID = workspaceID
	}

	return &result
}
