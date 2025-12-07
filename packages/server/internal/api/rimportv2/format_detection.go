//nolint:revive // exported
// Package rimportv2 provides a modern unified import service with TypeSpec compliance.
// It implements automatic format detection and supports multiple import formats.
package rimportv2

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// Format represents the supported import formats
type Format int

const (
	FormatUnknown Format = iota
	FormatHAR
	FormatYAML
	FormatJSON
	FormatCURL
	FormatPostman
)

// String returns the string representation of the format
func (f Format) String() string {
	switch f {
	case FormatHAR:
		return "HAR"
	case FormatYAML:
		return "YAML"
	case FormatJSON:
		return "JSON"
	case FormatCURL:
		return "CURL"
	case FormatPostman:
		return "Postman"
	default:
		return "Unknown"
	}
}

// DetectionResult represents the result of format detection with confidence
type DetectionResult struct {
	Format     Format
	Confidence float64 // 0.0 to 1.0
	Reason     string  // Human-readable explanation
}

// FormatDetector implements automatic format detection with confidence scoring
type FormatDetector struct {
	// Pre-compiled regular expressions for performance
	harPattern     *regexp.Regexp
	curlPattern    *regexp.Regexp
	postmanPattern *regexp.Regexp
	yamlPattern    *regexp.Regexp
}

// NewFormatDetector creates a new format detector with compiled patterns
func NewFormatDetector() *FormatDetector {
	return &FormatDetector{
		harPattern:     regexp.MustCompile(`^\s*\{?\s*"?log"?[\s\S]*"?entries"?[\s\S]*\}?\s*$`),
		curlPattern:    regexp.MustCompile(`(?i)^\s*curl\s+`),
		postmanPattern: regexp.MustCompile(`(?i)"?info"?\s*:\s*\{[\s\S]*"?name"?[\s\S]*"?schema"?\s*:\s*"https://schema\.getpostman\.com/json/collection/v2\.1\.0/collection\.json"`),
		yamlPattern:    regexp.MustCompile(`(?i)^\s*flows?\s*:`),
	}
}

// DetectFormat automatically detects the format of input data with confidence scoring
func (fd *FormatDetector) DetectFormat(data []byte) *DetectionResult {
	if len(data) == 0 {
		return &DetectionResult{
			Format:     FormatUnknown,
			Confidence: 1.0,
			Reason:     "Empty data",
		}
	}

	// Convert to string for pattern matching
	content := string(data)
	trimmed := strings.TrimSpace(content)

	// Check each format with confidence scoring
	results := []*DetectionResult{
		fd.detectHAR(trimmed),
		fd.detectPostman(trimmed),
		fd.detectCURL(trimmed),
		fd.detectYAML(trimmed),
		fd.detectJSON(trimmed),
	}

	// Find the result with highest confidence
	best := &DetectionResult{Format: FormatUnknown, Confidence: 0.0}
	for _, result := range results {
		if result.Confidence > best.Confidence {
			best = result
		}
	}

	// If confidence is too low, return unknown
	if best.Confidence < 0.3 {
		return &DetectionResult{
			Format:     FormatUnknown,
			Confidence: 1.0 - best.Confidence,
			Reason:     fmt.Sprintf("Low confidence detection (%.2f), best guess: %s", best.Confidence, best.Format),
		}
	}

	return best
}

// detectHAR detects HAR format with confidence scoring
func (fd *FormatDetector) detectHAR(content string) *DetectionResult {
	confidence := 0.0
	reason := ""

	// Check for HAR JSON structure
	if fd.harPattern.MatchString(content) {
		confidence += 0.7
		reason += "HAR JSON structure detected; "
	}

	// Look for specific HAR fields
	if strings.Contains(content, `"log"`) {
		confidence += 0.2
		reason += "HAR log field found; "
	}

	if strings.Contains(content, `"entries"`) {
		confidence += 0.2
		reason += "HAR entries field found; "
	}

	// Check for HAR-specific fields
	harFields := []string{
		`"startedDateTime"`,
		`"request"`,
		`"response"`,
		`"time"`,
		`"_resourceType"`,
	}

	for _, field := range harFields {
		if strings.Contains(content, field) {
			confidence += 0.05
		}
	}

	// Check URL patterns common in HAR
	if strings.Contains(content, `"http://`) || strings.Contains(content, `"https://`) {
		confidence += 0.1
		reason += "HTTP URLs found; "
	}

	// Validate it's actually valid JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(content), &jsonData); err == nil {
		confidence += 0.2
		reason += "Valid JSON; "

		// Deep check for HAR structure
		if log, ok := jsonData["log"].(map[string]interface{}); ok {
			if _, hasEntries := log["entries"]; hasEntries {
				confidence += 0.3
				reason += "HAR log.entries structure validated; "
			}
		}
	} else {
		confidence -= 0.3
		reason += fmt.Sprintf("Invalid JSON: %v; ", err)
	}

	if confidence < 0 {
		confidence = 0
	}

	return &DetectionResult{
		Format:     FormatHAR,
		Confidence: confidence,
		Reason:     strings.TrimSpace(reason),
	}
}

// detectPostman detects Postman collection format with confidence scoring
func (fd *FormatDetector) detectPostman(content string) *DetectionResult {
	confidence := 0.0
	reason := ""

	// Check for Postman schema URL
	if fd.postmanPattern.MatchString(content) {
		confidence += 0.8
		reason += "Postman v2.1.0 schema detected; "
	}

	// Look for Postman-specific fields
	postmanFields := []string{
		`"info"`,
		`"item"`,
		`"request"`,
		`"header"`,
		`"body"`,
		`"url"`,
	}

	for _, field := range postmanFields {
		if strings.Contains(content, field) {
			confidence += 0.1
		}
	}

	// Check for Postman-specific structure
	if strings.Contains(content, `"method"`) && strings.Contains(content, `"GET"`) {
		confidence += 0.2
		reason += "HTTP methods found; "
	}

	// Validate it's valid JSON
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(content), &jsonData); err == nil {
		confidence += 0.2
		reason += "Valid JSON; "

		// Deep check for Postman structure
		if info, ok := jsonData["info"].(map[string]interface{}); ok {
			if _, hasName := info["name"]; hasName {
				confidence += 0.2
				reason += "Postman info.name structure validated; "
			}
		}

		if _, hasItems := jsonData["item"]; hasItems {
			confidence += 0.2
			reason += "Postman item array found; "
		}
	} else {
		confidence -= 0.3
		reason += fmt.Sprintf("Invalid JSON: %v; ", err)
	}

	if confidence < 0 {
		confidence = 0
	}

	return &DetectionResult{
		Format:     FormatPostman,
		Confidence: confidence,
		Reason:     strings.TrimSpace(reason),
	}
}

// detectCURL detects curl command format with confidence scoring
func (fd *FormatDetector) detectCURL(content string) *DetectionResult {
	confidence := 0.0
	reason := ""

	// Check for curl command pattern
	if fd.curlPattern.MatchString(content) {
		confidence += 0.8
		reason += "Curl command pattern detected; "
	}

	// Look for curl-specific flags
	curlFlags := []string{
		"-X", "--request",
		"-H", "--header",
		"-d", "--data",
		"-b", "--cookie",
		"-u", "--user",
		"-L", "--location",
		"-k", "--insecure",
		"-v", "--verbose",
	}

	for _, flag := range curlFlags {
		if strings.Contains(content, flag) {
			confidence += 0.1
		}
	}

	// Check for URLs in the content
	if strings.Contains(content, "http://") || strings.Contains(content, "https://") {
		confidence += 0.3
		reason += "HTTP URLs found; "
	}

	// Check for HTTP methods
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	for _, method := range methods {
		if strings.Contains(strings.ToUpper(content), method) {
			confidence += 0.1
			reason += fmt.Sprintf("%s method found; ", method)
		}
	}

	// Check for JSON data in curl
	if strings.Contains(content, `'{`) || strings.Contains(content, `"{`) {
		confidence += 0.1
		reason += "JSON data detected; "
	}

	// Should not be valid JSON (curl commands are not JSON)
	var jsonData map[string]interface{}
	if json.Unmarshal([]byte(content), &jsonData) != nil {
		confidence += 0.2
		reason += "Not JSON (expected for curl); "
	} else {
		confidence -= 0.5
		reason += "Appears to be JSON (unlikely for curl); "
	}

	if confidence < 0 {
		confidence = 0
	}

	return &DetectionResult{
		Format:     FormatCURL,
		Confidence: confidence,
		Reason:     strings.TrimSpace(reason),
	}
}

// detectYAML detects YAML flow format with confidence scoring
func (fd *FormatDetector) detectYAML(content string) *DetectionResult {
	confidence := 0.0
	reason := ""

	// Check for YAML flow pattern
	if fd.yamlPattern.MatchString(content) {
		confidence += 0.6
		reason += "YAML flows pattern detected; "
	}

	// Look for YAML-specific fields
	yamlFields := []string{
		"flows:",
		"requests:",
		"variables:",
		"steps:",
		"method:",
		"url:",
		"headers:",
		"body:",
	}

	for _, field := range yamlFields {
		if strings.Contains(content, field) {
			confidence += 0.15
		}
	}

	// Check for YAML structure (indentation, colons)
	if strings.Contains(content, ": ") && strings.Contains(content, "\n  ") {
		confidence += 0.2
		reason += "YAML indentation and structure detected; "
	}

	// Validate it's valid YAML
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &yamlData); err == nil {
		confidence += 0.3
		reason += "Valid YAML; "

		// Deep check for flow structure
		if _, hasFlows := yamlData["flows"]; hasFlows {
			confidence += 0.3
			reason += "YAML flows field found; "
		}

		if _, hasRequests := yamlData["requests"]; hasRequests {
			confidence += 0.2
			reason += "YAML requests field found; "
		}
	} else {
		confidence -= 0.3
		reason += fmt.Sprintf("Invalid YAML: %v; ", err)
	}

	// Should not be valid JSON
	var jsonData map[string]interface{}
	if json.Unmarshal([]byte(content), &jsonData) != nil {
		confidence += 0.1
		reason += "Not JSON (expected for YAML); "
	} else {
		confidence -= 0.2
		reason += "Also valid JSON (might be JSON, not YAML); "
	}

	if confidence < 0 {
		confidence = 0
	}

	return &DetectionResult{
		Format:     FormatYAML,
		Confidence: confidence,
		Reason:     strings.TrimSpace(reason),
	}
}

// detectJSON detects generic JSON format with confidence scoring
func (fd *FormatDetector) detectJSON(content string) *DetectionResult {
	confidence := 0.0
	reason := ""

	// Check if it starts/ends with braces
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		confidence += 0.6
		reason += "JSON object structure; "
	} else if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		confidence += 0.4
		reason += "JSON array structure; "
	}

	// Validate it's valid JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(content), &jsonData); err == nil {
		confidence += 0.5
		reason += "Valid JSON; "

		// Check for HTTP-related JSON that's not HAR or Postman
		if fd.containsHTTPFields(jsonData) {
			confidence += 0.3
			reason += "HTTP-related JSON fields; "
		}
	} else {
		confidence -= 0.8
		reason += fmt.Sprintf("Invalid JSON: %v; ", err)
	}

	if confidence < 0 {
		confidence = 0
	}

	return &DetectionResult{
		Format:     FormatJSON,
		Confidence: confidence,
		Reason:     strings.TrimSpace(reason),
	}
}

// containsHTTPFields checks if JSON data contains HTTP-related fields
func (fd *FormatDetector) containsHTTPFields(data interface{}) bool {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			lowerKey := strings.ToLower(key)
			if lowerKey == "method" || lowerKey == "url" || lowerKey == "headers" ||
				lowerKey == "body" || lowerKey == "query" || lowerKey == "params" {
				return true
			}
			if fd.containsHTTPFields(value) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if fd.containsHTTPFields(item) {
				return true
			}
		}
	}
	return false
}

// ValidateFormat performs additional validation on detected format
func (fd *FormatDetector) ValidateFormat(data []byte, format Format) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	switch format {
	case FormatHAR:
		return fd.validateHAR(data)
	case FormatPostman:
		return fd.validatePostman(data)
	case FormatCURL:
		return fd.validateCURL(data)
	case FormatYAML:
		return fd.validateYAML(data)
	case FormatJSON:
		return fd.validateJSON(data)
	default:
		return fmt.Errorf("unknown format: %v", format)
	}
}

// validateHAR validates HAR format specifically
func (fd *FormatDetector) validateHAR(data []byte) error {
	var har struct {
		Log struct {
			Entries []interface{} `json:"entries"`
		} `json:"log"`
	}

	if err := json.Unmarshal(data, &har); err != nil {
		return fmt.Errorf("invalid HAR JSON: %w", err)
	}

	if len(har.Log.Entries) == 0 {
		return fmt.Errorf("HAR file contains no entries")
	}

	return nil
}

// validatePostman validates Postman format specifically
func (fd *FormatDetector) validatePostman(data []byte) error {
	var postman struct {
		Info struct {
			Name   string `json:"name"`
			Schema string `json:"schema"`
		} `json:"info"`
		Item []interface{} `json:"item"`
	}

	if err := json.Unmarshal(data, &postman); err != nil {
		return fmt.Errorf("invalid Postman JSON: %w", err)
	}

	if postman.Info.Name == "" {
		return fmt.Errorf("Postman collection missing name")
	}

	if !strings.Contains(postman.Info.Schema, "postman.com") {
		return fmt.Errorf("invalid Postman schema URL")
	}

	return nil
}

// validateCURL validates curl command format
func (fd *FormatDetector) validateCURL(data []byte) error {
	content := string(data)
	if !fd.curlPattern.MatchString(content) {
		return fmt.Errorf("does not appear to be a curl command")
	}

	// Try to extract URL
	urlPattern := regexp.MustCompile(`(?:https?://|www\.)[^\s'"]+`)
	urls := urlPattern.FindAllString(content, -1)
	if len(urls) == 0 {
		return fmt.Errorf("no URL found in curl command")
	}

	// Validate URL format
	for _, urlStr := range urls {
		if _, err := url.Parse(urlStr); err != nil {
			return fmt.Errorf("invalid URL in curl command: %s", urlStr)
		}
	}

	return nil
}

// validateYAML validates YAML flow format
func (fd *FormatDetector) validateYAML(data []byte) error {
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}

	// Check for required flow structure
	if _, hasFlows := yamlData["flows"]; !hasFlows {
		if _, hasRequests := yamlData["requests"]; !hasRequests {
			return fmt.Errorf("YAML missing required 'flows' or 'requests' field")
		}
	}

	return nil
}

// validateJSON validates generic JSON format
func (fd *FormatDetector) validateJSON(data []byte) error {
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// IsUTF8 checks if data is valid UTF-8
func IsUTF8(data []byte) bool {
	return utf8.Valid(data)
}

// DetectAndValidate performs format detection and validation in one step
func (fd *FormatDetector) DetectAndValidate(data []byte) (*DetectionResult, error) {
	result := fd.DetectFormat(data)

	if result.Format == FormatUnknown {
		return result, fmt.Errorf("unable to detect format: %s", result.Reason)
	}

	if err := fd.ValidateFormat(data, result.Format); err != nil {
		return result, fmt.Errorf("format validation failed for %s: %w", result.Format, err)
	}

	return result, nil
}
