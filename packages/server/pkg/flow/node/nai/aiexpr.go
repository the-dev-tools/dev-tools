package nai

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// AIExprEnv provides AI-aware expression functions for use with expr-lang.
// These functions help generate detailed variable descriptions that AI can understand.
type AIExprEnv struct {
	VarMap    map[string]any
	AINodeName string // The AI node name (e.g., "ai_1") for context
}

// NewAIExprEnv creates a new AI expression environment
func NewAIExprEnv(varMap map[string]any, aiNodeName string) *AIExprEnv {
	return &AIExprEnv{
		VarMap:    varMap,
		AINodeName: aiNodeName,
	}
}

// GetEnvMap returns a map with all AI functions for expr-lang
func (e *AIExprEnv) GetEnvMap() map[string]any {
	env := make(map[string]any)

	// Copy existing varMap
	for k, v := range e.VarMap {
		env[k] = v
	}

	// Add AI functions
	env["ai"] = e.AI
	env["aivar"] = e.AIVar
	env["airef"] = e.AIRef
	env["aidesc"] = e.AIDesc
	env["aichain"] = e.AIChain

	return env
}

// AI creates a detailed AI parameter description.
// Usage: ai("userId", "The user ID to fetch", "number")
// Returns: [AI_PARAM: name=userId | type=number | desc="The user ID to fetch" | set_with=ai_1.userId]
func (e *AIExprEnv) AI(name, description, varType string) string {
	return fmt.Sprintf("[AI_PARAM: name=%s | type=%s | desc=\"%s\" | set_with=%s.%s]",
		name, varType, description, e.AINodeName, name)
}

// AIVar creates a detailed variable reference for AI.
// Usage: aivar("GetUser.response.body.id")
// Returns a detailed description of the variable including its current value if available.
func (e *AIExprEnv) AIVar(path string) string {
	// Try to get the current value
	value, valueType := e.resolvePathWithType(path)

	var parts []string
	parts = append(parts, fmt.Sprintf("path=%s", path))

	if valueType != "" {
		parts = append(parts, fmt.Sprintf("type=%s", valueType))
	}

	if value != nil {
		// Truncate long values
		valueStr := formatValueForAI(value)
		if len(valueStr) > 100 {
			valueStr = valueStr[:100] + "..."
		}
		parts = append(parts, fmt.Sprintf("value=%s", valueStr))
	}

	parts = append(parts, "access=get_variable")

	return fmt.Sprintf("[AI_VAR: %s]", strings.Join(parts, " | "))
}

// AIRef creates a reference hint for chaining - tells AI where to get a value.
// Usage: airef("GetUser.response.body.id", "userId", "number")
// Means: "Get GetUser.response.body.id and use it as userId (which should be a number)"
func (e *AIExprEnv) AIRef(sourcePath, targetParam, varType string) string {
	return fmt.Sprintf("[AI_CHAIN: get %s → set %s.%s (%s)]",
		sourcePath, e.AINodeName, targetParam, varType)
}

// AIDesc creates a rich description of a variable path with context.
// Usage: aidesc("GetUser.response.body", "User profile data from the API")
func (e *AIExprEnv) AIDesc(path, description string) string {
	value, valueType := e.resolvePathWithType(path)

	detail := AIVarDetail{
		Path:        path,
		Type:        valueType,
		Description: description,
	}

	if value != nil {
		detail.Example = formatValueForAI(value)
		if len(detail.Example) > 50 {
			detail.Example = detail.Example[:50] + "..."
		}
	}

	return detail.ToAIString()
}

// AIChain creates an explicit chain instruction for the AI.
// Usage: aichain("GetPosts.response.body[0].id", "postId", "GetComments")
// Tells AI: "After GetPosts, get response.body[0].id and set it as postId before calling GetComments"
func (e *AIExprEnv) AIChain(sourcePath, paramName, targetTool string) string {
	return fmt.Sprintf("[AI_FLOW: %s ──► %s.%s ──► %s]",
		sourcePath, e.AINodeName, paramName, targetTool)
}

// resolvePathWithType tries to resolve a dotted path and determine its type
func (e *AIExprEnv) resolvePathWithType(path string) (any, string) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, ""
	}

	var current any = e.VarMap

	for _, part := range parts {
		if current == nil {
			return nil, ""
		}

		switch v := current.(type) {
		case map[string]any:
			current = v[part]
		default:
			// Try reflection for struct access
			rv := reflect.ValueOf(current)
			if rv.Kind() == reflect.Map {
				key := reflect.ValueOf(part)
				val := rv.MapIndex(key)
				if val.IsValid() {
					current = val.Interface()
				} else {
					return nil, ""
				}
			} else {
				return nil, ""
			}
		}
	}

	return current, inferType(current)
}

// inferType determines the type string for a value
func inferType(v any) string {
	if v == nil {
		return "null"
	}

	switch v.(type) {
	case string:
		return "string"
	case int, int32, int64, float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []any, []map[string]any:
		return "array"
	case map[string]any:
		return "object"
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			return "array"
		case reflect.Map, reflect.Struct:
			return "object"
		case reflect.String:
			return "string"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			return "number"
		case reflect.Bool:
			return "boolean"
		default:
			return "unknown"
		}
	}
}

// formatValueForAI creates a readable string representation of a value
func formatValueForAI(v any) string {
	if v == nil {
		return "null"
	}

	switch val := v.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", val)
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%v", val)
	default:
		// For complex types, use JSON
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	}
}

// =============================================================================
// TOOL DESCRIPTION ENHANCER - Adds AI context to tool descriptions
// =============================================================================

// EnhanceToolDescriptionForAI takes a basic tool description and adds
// AI-friendly context including variable paths, types, and chain hints.
func EnhanceToolDescriptionForAI(
	toolName string,
	baseDescription string,
	params []AIParam,
	outputVars []string,
	prevToolOutputs map[string][]string, // Previous tool outputs that can be used
) string {
	var sb strings.Builder

	// Base description
	if baseDescription != "" {
		sb.WriteString(baseDescription)
	} else {
		sb.WriteString(fmt.Sprintf("Tool: %s", toolName))
	}
	sb.WriteString("\n")

	// Input parameters with chain hints
	if len(params) > 0 {
		sb.WriteString("\n═══ INPUTS (set before calling) ═══\n")
		for _, p := range params {
			sb.WriteString(fmt.Sprintf("• ai_1.%s (%s): %s\n", p.Name, p.Type, p.Description))

			if p.SourceHint != "" {
				sb.WriteString(fmt.Sprintf("  ↳ Chain from: %s\n", p.SourceHint))
				sb.WriteString(fmt.Sprintf("  ↳ Example: set_variable(key=\"ai_1.%s\", value=<value from %s>)\n",
					p.Name, p.SourceHint))
			} else {
				// Check if there's a matching output from previous tools
				for prevTool, outputs := range prevToolOutputs {
					for _, out := range outputs {
						if strings.Contains(strings.ToLower(out), strings.ToLower(p.Name)) {
							sb.WriteString(fmt.Sprintf("  ↳ Hint: Maybe use %s.%s?\n", prevTool, out))
						}
					}
				}
			}
		}
	}

	// Output variables
	if len(outputVars) > 0 {
		sb.WriteString("\n═══ OUTPUTS (available after calling) ═══\n")
		for _, v := range outputVars {
			sb.WriteString(fmt.Sprintf("• %s.%s\n", toolName, v))
		}
	}

	return sb.String()
}

// GenerateFlowOverview creates a high-level overview of tool chain for AI
func GenerateFlowOverview(tools []ToolOverview) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("═══════════════════════════════════════════\n")
	sb.WriteString("         AVAILABLE TOOLS OVERVIEW          \n")
	sb.WriteString("═══════════════════════════════════════════\n\n")

	for i, tool := range tools {
		sb.WriteString(fmt.Sprintf("[%d] %s\n", i+1, tool.Name))

		if len(tool.Inputs) > 0 {
			sb.WriteString("    Inputs:  ")
			var inputStrs []string
			for _, in := range tool.Inputs {
				inputStrs = append(inputStrs, fmt.Sprintf("%s(%s)", in.Name, in.Type))
			}
			sb.WriteString(strings.Join(inputStrs, ", "))
			sb.WriteString("\n")
		}

		if len(tool.Outputs) > 0 {
			sb.WriteString("    Outputs: ")
			sb.WriteString(strings.Join(tool.Outputs, ", "))
			sb.WriteString("\n")
		}

		// Show chain connections
		for _, in := range tool.Inputs {
			if in.SourceHint != "" {
				sb.WriteString(fmt.Sprintf("    Chain:   %s → ai_1.%s\n", in.SourceHint, in.Name))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// ToolOverview represents a simplified view of a tool for AI understanding
type ToolOverview struct {
	Name    string
	Inputs  []AIParam
	Outputs []string
}
