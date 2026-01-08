package rimportv2

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

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
		suffix.WriteString("?")
		suffix.WriteString(parsedURL.RawQuery)
	}

	// Add fragment
	if parsedURL.Fragment != "" {
		suffix.WriteString("#")
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
		return fmt.Sprintf("{%s}", variable)
	}
	if !strings.HasPrefix(suffix, "/") && !strings.HasPrefix(suffix, "?") && !strings.HasPrefix(suffix, "#") {
		suffix = "/" + suffix
	}
	return fmt.Sprintf("{%s}%s", variable, suffix)
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

// applyDomainReplacements replaces domain URLs with variable references in HTTP requests.
// For example: https://api.example.com/users -> {{API_HOST}}/users
// This also handles DeltaUrl if it's set (for depfinder-templated URLs).
func applyDomainReplacements(httpRequests []mhttp.HTTP, domainData []ImportDomainData) []mhttp.HTTP {
	// Build a map of domain -> variable for enabled domains with variables
	domainToVar := make(map[string]string)
	for _, dd := range domainData {
		if dd.Enabled && dd.Variable != "" {
			domainToVar[dd.Domain] = dd.Variable
		}
	}

	if len(domainToVar) == 0 {
		return httpRequests
	}

	// Replace domains in each HTTP request URL
	for i := range httpRequests {
		httpRequests[i].Url = replaceDomainInURL(httpRequests[i].Url, domainToVar)

		// Also replace in DeltaUrl if it's set (non-nil means there's an actual override)
		if httpRequests[i].DeltaUrl != nil {
			replacedDeltaUrl := replaceDomainInURL(*httpRequests[i].DeltaUrl, domainToVar)
			httpRequests[i].DeltaUrl = &replacedDeltaUrl
		}
	}
	return httpRequests
}

// replaceDomainInURL replaces the domain part of a URL with a variable reference.
// Example: https://api.example.com/users -> {{API_HOST}}/users
// Note: This uses string manipulation to preserve template variables like {{ var }}
// that may already exist in the URL path (from depfinder).
func replaceDomainInURL(urlStr string, domainToVar map[string]string) string {
	// Find the scheme (http:// or https://)
	schemeEnd := strings.Index(urlStr, "://")
	if schemeEnd == -1 {
		return urlStr // Not a valid URL with scheme
	}

	// Find where the host ends (first / after scheme, or end of string)
	hostStart := schemeEnd + 3
	pathStart := strings.Index(urlStr[hostStart:], "/")

	var host, pathAndMore string
	if pathStart == -1 {
		// No path, just host
		host = urlStr[hostStart:]
		pathAndMore = ""
	} else {
		host = urlStr[hostStart : hostStart+pathStart]
		pathAndMore = urlStr[hostStart+pathStart:]
	}

	// Remove port if present for domain matching
	hostWithoutPort := host
	if colonIdx := strings.LastIndex(host, ":"); colonIdx != -1 {
		// Check if this is actually a port (not IPv6)
		if !strings.Contains(host[colonIdx:], "]") {
			hostWithoutPort = host[:colonIdx]
		}
	}

	// Check if this domain has a variable mapping
	varName, exists := domainToVar[hostWithoutPort]
	if !exists {
		// Also try with the full host (including port)
		varName, exists = domainToVar[host]
		if !exists {
			return urlStr // No mapping found, return unchanged
		}
	}

	// Build the new URL with variable reference
	// {{VARIABLE}}/path?query#fragment
	varRef := "{{" + varName + "}}"

	if pathAndMore == "" || pathAndMore == "/" {
		return varRef
	}

	return varRef + pathAndMore
}
