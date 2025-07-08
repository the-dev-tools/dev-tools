package workflowsimple

import (
	"gopkg.in/yaml.v3"
	"regexp"
	"strings"
	"the-dev-tools/server/pkg/ioworkspace"
)

// ExportOptions contains options for cleaning up exports
type ExportOptions struct {
	// ReplaceTokens replaces hardcoded JWT tokens with variables
	ReplaceTokens bool
	// FilterBrowserHeaders removes common browser headers
	FilterBrowserHeaders bool
	// TokenVariableName is the variable name to use for tokens (default: "token")
	TokenVariableName string
}

// Default export options
var DefaultExportOptions = ExportOptions{
	ReplaceTokens:        false,
	FilterBrowserHeaders: false,
	TokenVariableName:    "token",
}

// Browser headers that are typically not needed in API testing
var browserHeaders = map[string]bool{
	"sec-ch-ua":          true,
	"sec-ch-ua-mobile":   true,
	"sec-ch-ua-platform": true,
	"sec-fetch-dest":     true,
	"sec-fetch-mode":     true,
	"sec-fetch-site":     true,
	"user-agent":         true,
	"accept-encoding":    true,
	"accept-language":    true,
	"priority":           true,
	"referer":            true,
	"origin":             true,
	"if-none-match":      true,
}

// JWT token regex pattern
var jwtRegex = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)

// ExportWorkflowCleanWithOptions exports with cleanup options
func ExportWorkflowCleanWithOptions(workspaceData *ioworkspace.WorkspaceData, options ExportOptions) ([]byte, error) {
	// First, do the normal export
	exported, err := ExportWorkflowClean(workspaceData)
	if err != nil {
		return nil, err
	}

	if !options.ReplaceTokens && !options.FilterBrowserHeaders {
		// No cleanup needed
		return exported, nil
	}

	// Parse the YAML to clean it up
	var data map[string]any
	if err := yaml.Unmarshal(exported, &data); err != nil {
		return nil, err
	}

	// Clean up requests
	if requests, ok := data["requests"].([]any); ok {
		for _, req := range requests {
			if reqMap, ok := req.(map[string]any); ok {
				cleanRequest(reqMap, options)
			}
		}
	}

	// Re-marshal the cleaned data
	return yaml.Marshal(data)
}

// cleanRequest cleans up a single request based on options
func cleanRequest(req map[string]any, options ExportOptions) {
	// Clean headers
	if headers, ok := req["headers"].(map[string]any); ok {
		cleanedHeaders := make(map[string]any)
		
		for key, value := range headers {
			keyLower := strings.ToLower(key)
			
			// Skip browser headers if filtering is enabled
			if options.FilterBrowserHeaders && browserHeaders[keyLower] {
				continue
			}
			
			// Replace JWT tokens if enabled
			if options.ReplaceTokens && keyLower == "authorization" {
				if strValue, ok := value.(string); ok {
					if jwtRegex.MatchString(strValue) {
						value = jwtRegex.ReplaceAllString(strValue, "{{"+options.TokenVariableName+"}}")
					}
				}
			}
			
			cleanedHeaders[key] = value
		}
		
		// Only include headers if there are any left
		if len(cleanedHeaders) > 0 {
			req["headers"] = cleanedHeaders
		} else {
			delete(req, "headers")
		}
	}
}

