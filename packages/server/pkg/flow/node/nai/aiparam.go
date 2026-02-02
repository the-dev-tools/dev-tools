package nai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// AIParam represents a typed parameter that the AI needs to provide.
// Used with the {{ ai('name', 'description', 'type') }} or
// {{ ai('name', 'description', 'type', 'source') }} syntax for chaining.
type AIParam struct {
	Name        string // Variable name (e.g., "userId")
	Description string // Human-readable description (e.g., "The user ID to fetch")
	Type        string // Data type: "string", "number", "boolean", "array", "object"
	Required    bool   // Whether this parameter is required (default: true)
	SourceHint  string // Optional: Where to get this value (e.g., "GetUser.response.body.id")
}

// AIParamProvider is an interface that nodes can implement to declare
// typed AI parameters using the {{ ai('name', 'desc', 'type') }} syntax.
type AIParamProvider interface {
	// GetAIParams returns all AI parameters this node requires.
	GetAIParams() []AIParam
}

// Supported types for AI parameters
const (
	AIParamTypeString  = "string"
	AIParamTypeNumber  = "number"
	AIParamTypeBoolean = "boolean"
	AIParamTypeArray   = "array"
	AIParamTypeObject  = "object"
)

// AIVarDetail represents a detailed description of a variable for AI understanding
type AIVarDetail struct {
	Path        string `json:"path"`                  // Full variable path (e.g., "GetUser.response.body.id")
	Type        string `json:"type,omitempty"`        // Data type if known
	Description string `json:"description,omitempty"` // Human description
	Source      string `json:"source,omitempty"`      // Where this value comes from
	Example     string `json:"example,omitempty"`     // Example value
}

// aiParamRegex matches {{ ai('name', 'description', 'type') }} patterns
// Supports both single and double quotes
var aiParamRegex = regexp.MustCompile(`\{\{\s*ai\(\s*['"]([^'"]+)['"]\s*,\s*['"]([^'"]+)['"]\s*,\s*['"]([^'"]+)['"]\s*\)\s*\}\}`)

// aiParamWithSourceRegex matches {{ ai('name', 'desc', 'type', 'source') }} for chaining
// The 4th parameter hints where to get the value from (e.g., "GetUser.response.body.id")
var aiParamWithSourceRegex = regexp.MustCompile(`\{\{\s*ai\(\s*['"]([^'"]+)['"]\s*,\s*['"]([^'"]+)['"]\s*,\s*['"]([^'"]+)['"]\s*,\s*['"]([^'"]+)['"]\s*\)\s*\}\}`)

// aiVarRegex matches {{ aivar('path') }} or {{ aivar('path', 'description') }} for detailed var descriptions
var aiVarRegex = regexp.MustCompile(`\{\{\s*aivar\(\s*['"]([^'"]+)['"]\s*(?:,\s*['"]([^'"]+)['"]\s*)?\)\s*\}\}`)

// ParseAIParams extracts all AI parameters from a string containing
// {{ ai('name', 'description', 'type') }} or {{ ai('name', 'desc', 'type', 'source') }} expressions.
func ParseAIParams(input string) []AIParam {
	if input == "" {
		return nil
	}

	params := make([]AIParam, 0)
	seen := make(map[string]bool)

	// First, try to match 4-parameter version (with source hint for chaining)
	matchesWithSource := aiParamWithSourceRegex.FindAllStringSubmatch(input, -1)
	for _, match := range matchesWithSource {
		if len(match) < 5 {
			continue
		}

		name := strings.TrimSpace(match[1])
		desc := strings.TrimSpace(match[2])
		paramType := normalizeParamType(match[3])
		sourceHint := strings.TrimSpace(match[4])

		if seen[name] {
			continue
		}
		seen[name] = true

		params = append(params, AIParam{
			Name:        name,
			Description: desc,
			Type:        paramType,
			Required:    true,
			SourceHint:  sourceHint,
		})
	}

	// Then match 3-parameter version (without source)
	matches := aiParamRegex.FindAllStringSubmatch(input, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		name := strings.TrimSpace(match[1])
		desc := strings.TrimSpace(match[2])
		paramType := normalizeParamType(match[3])

		// Skip if already found in 4-param version
		if seen[name] {
			continue
		}
		seen[name] = true

		params = append(params, AIParam{
			Name:        name,
			Description: desc,
			Type:        paramType,
			Required:    true,
		})
	}

	if len(params) == 0 {
		return nil
	}
	return params
}

// normalizeParamType validates and normalizes parameter types
func normalizeParamType(t string) string {
	paramType := strings.ToLower(strings.TrimSpace(t))
	switch paramType {
	case AIParamTypeString, AIParamTypeNumber, AIParamTypeBoolean, AIParamTypeArray, AIParamTypeObject:
		return paramType
	default:
		return AIParamTypeString
	}
}

// ParseAIParamsFromMultiple extracts AI parameters from multiple strings
// and returns a deduplicated list.
func ParseAIParamsFromMultiple(inputs ...string) []AIParam {
	seen := make(map[string]bool)
	var result []AIParam

	for _, input := range inputs {
		params := ParseAIParams(input)
		for _, p := range params {
			if !seen[p.Name] {
				seen[p.Name] = true
				result = append(result, p)
			}
		}
	}

	return result
}

// ResolveAIParamPlaceholder converts an AI param expression to the actual variable reference.
// {{ ai('userId', 'User ID', 'number') }} -> {{ aiNodeName.userId }}
func ResolveAIParamPlaceholder(input string, aiNodeName string) string {
	return aiParamRegex.ReplaceAllStringFunc(input, func(match string) string {
		submatch := aiParamRegex.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		paramName := submatch[1]
		return fmt.Sprintf("{{%s.%s}}", aiNodeName, paramName)
	})
}

// GenerateAIParamDescription creates a rich description string from AI parameters.
// This is used to tell the AI exactly what inputs are needed and their types.
// aiNodeName is the name of the AI agent node (e.g., "ai_1") - used for setting input variables.
func GenerateAIParamDescription(aiNodeName string, params []AIParam) string {
	if len(params) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("BEFORE CALLING: Set these variables using set_variable:\n")

	for _, p := range params {
		// Format with source hint for chaining
		if p.SourceHint != "" {
			// Auto-chain hint: tell AI exactly where to get the value
			sb.WriteString(fmt.Sprintf("  - %s.%s (%s): %s\n", aiNodeName, p.Name, p.Type, p.Description))
			sb.WriteString(fmt.Sprintf("    └─► GET VALUE FROM: %s (use get_variable first)\n", p.SourceHint))
		} else {
			sb.WriteString(fmt.Sprintf("  - %s.%s (%s): %s\n", aiNodeName, p.Name, p.Type, p.Description))
		}
	}

	return sb.String()
}

// GenerateAIParamToolDescription creates a complete tool description including AI params.
// toolNodeName is the name of the tool being described (e.g., "GetUser")
// aiNodeName is typically "ai_1" - where input variables should be set
func GenerateAIParamToolDescription(toolNodeName string, baseDescription string, params []AIParam, outputVars []string) string {
	var sb strings.Builder

	// Base description - keep it simple
	if baseDescription != "" {
		sb.WriteString(baseDescription)
	} else {
		sb.WriteString(fmt.Sprintf("Executes '%s'.", toolNodeName))
	}

	// AI Parameters section - use "ai_1" as the standard AI node name for inputs
	if len(params) > 0 {
		sb.WriteString("\n\n")
		sb.WriteString(GenerateAIParamDescription("ai_1", params))
	}

	// Output section with detailed info
	if len(outputVars) > 0 {
		sb.WriteString("\nAFTER CALLING: Results available at:\n")
		for _, v := range outputVars {
			sb.WriteString(fmt.Sprintf("  - %s.%s\n", toolNodeName, v))
		}
	}

	return sb.String()
}

// =============================================================================
// AI EXPRESSION FUNCTIONS - For detailed variable descriptions in expr-lang
// =============================================================================

// AIVar creates a detailed description string for a variable that AI can understand.
// Usage in expressions: aivar("GetUser.response.body.id", "The user's unique ID")
// Returns a detailed string like: "[AI_REF: GetUser.response.body.id | User's ID | Use with get_variable]"
func AIVar(path string, description string) string {
	detail := AIVarDetail{
		Path:        path,
		Description: description,
	}
	return detail.ToAIString()
}

// AIVarTyped creates a detailed description with type information.
// Usage: aivar_typed("GetUser.response.body.id", "User ID", "number")
func AIVarTyped(path, description, varType string) string {
	detail := AIVarDetail{
		Path:        path,
		Description: description,
		Type:        varType,
	}
	return detail.ToAIString()
}

// AIVarFull creates the most detailed description with source and example.
// Usage: aivar_full("ai_1.userId", "User ID to fetch", "number", "from user input", "123")
func AIVarFull(path, description, varType, source, example string) string {
	detail := AIVarDetail{
		Path:        path,
		Description: description,
		Type:        varType,
		Source:      source,
		Example:     example,
	}
	return detail.ToAIString()
}

// ToAIString converts AIVarDetail to a detailed string for AI understanding
func (d AIVarDetail) ToAIString() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("path=%s", d.Path))

	if d.Type != "" {
		parts = append(parts, fmt.Sprintf("type=%s", d.Type))
	}
	if d.Description != "" {
		parts = append(parts, fmt.Sprintf("desc=\"%s\"", d.Description))
	}
	if d.Source != "" {
		parts = append(parts, fmt.Sprintf("from=%s", d.Source))
	}
	if d.Example != "" {
		parts = append(parts, fmt.Sprintf("example=%s", d.Example))
	}

	return fmt.Sprintf("[AI_VAR: %s]", strings.Join(parts, " | "))
}

// ToJSON converts AIVarDetail to JSON for structured AI consumption
func (d AIVarDetail) ToJSON() string {
	data, _ := json.Marshal(d)
	return string(data)
}

// ParseAIVarExpressions parses {{ aivar('path', 'desc') }} expressions and returns detailed strings
func ParseAIVarExpressions(input string) string {
	if input == "" {
		return input
	}

	return aiVarRegex.ReplaceAllStringFunc(input, func(match string) string {
		submatch := aiVarRegex.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}

		path := strings.TrimSpace(submatch[1])
		desc := ""
		if len(submatch) >= 3 && submatch[2] != "" {
			desc = strings.TrimSpace(submatch[2])
		}

		return AIVar(path, desc)
	})
}

// =============================================================================
// AUTO-CHAINING HELPERS
// =============================================================================

// ChainInfo represents how outputs from one tool connect to inputs of another
type ChainInfo struct {
	FromTool   string // Source tool name (e.g., "GetUser")
	FromPath   string // Output path (e.g., "response.body.id")
	ToParam    string // Target parameter name (e.g., "userId")
	ToTool     string // Target tool name (e.g., "GetPosts")
	ParamType  string // Expected type
}

// GenerateChainDescription creates a description showing how tools connect
func GenerateChainDescription(chains []ChainInfo) string {
	if len(chains) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("DATA FLOW (how outputs connect to inputs):\n")

	for _, c := range chains {
		sb.WriteString(fmt.Sprintf("  %s.%s ──► ai_1.%s ──► %s\n",
			c.FromTool, c.FromPath, c.ToParam, c.ToTool))
	}

	return sb.String()
}

// BuildChainFromParams analyzes AI params and builds chain info
func BuildChainFromParams(toolName string, params []AIParam) []ChainInfo {
	var chains []ChainInfo

	for _, p := range params {
		if p.SourceHint == "" {
			continue
		}

		// Parse source hint (e.g., "GetUser.response.body.id")
		parts := strings.SplitN(p.SourceHint, ".", 2)
		if len(parts) < 2 {
			continue
		}

		chains = append(chains, ChainInfo{
			FromTool:  parts[0],
			FromPath:  parts[1],
			ToParam:   p.Name,
			ToTool:    toolName,
			ParamType: p.Type,
		})
	}

	return chains
}

// ExtractAIParamNames returns just the parameter names from a list of AIParams.
// Useful for getting required variable names.
func ExtractAIParamNames(aiNodeName string, params []AIParam) []string {
	if len(params) == 0 {
		return nil
	}

	names := make([]string, len(params))
	for i, p := range params {
		names[i] = fmt.Sprintf("%s.%s", aiNodeName, p.Name)
	}
	return names
}
