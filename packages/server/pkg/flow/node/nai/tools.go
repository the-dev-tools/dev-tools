package nai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
)

var toolNameRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitizeToolName(name string) string {
	return toolNameRegex.ReplaceAllString(name, "_")
}

// getVariableTool allows the agent to read a variable from the flow's context.
func getVariableTool(req *node.FlowNodeRequest) llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "get_variable",
			Description: "Get the value of a specific variable from the flow context. Only use this if you need specific data not provided in the initial prompt.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{
						"type":        "string",
						"description": "The exact name of the variable to retrieve (e.g., 'auth_token' or 'NodeName.data').",
					},
				},
				"required": []string{"key"},
			},
		},
	}
}

// setVariableTool allows the agent to write a variable to the flow's context.
func setVariableTool(req *node.FlowNodeRequest) llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "set_variable",
			Description: "Set a value in the flow context. Use this to pass data to subsequent tools or nodes in the flow.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"key": map[string]any{
						"type":        "string",
						"description": "The name of the variable to set.",
					},
					"value": map[string]any{
						"type":        "string",
						"description": "The value to store (JSON string or plain text).",
					},
				},
				"required": []string{"key", "value"},
			},
		},
	}
}

func handleGetVariable(ctx context.Context, req *node.FlowNodeRequest, args string) (string, error) {
	var input struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(args), &input); err != nil {
		return "", err
	}

	val, err := node.ReadVarRawWithTracking(req, input.Key, req.VariableTracker)
	if err != nil {
		return "", fmt.Errorf("variable '%s' not found: %w", input.Key, err)
	}

	res, err := json.Marshal(val)
	if err != nil {
		return "", fmt.Errorf("failed to marshal variable value: %w", err)
	}
	return string(res), nil
}

func handleSetVariable(ctx context.Context, req *node.FlowNodeRequest, args string) (string, error) {
	var input struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	if err := json.Unmarshal([]byte(args), &input); err != nil {
		return "", err
	}

	// If value is a string that looks like JSON, try to unmarshal it first
	// so it's stored as a proper object in the map if possible.
	// But node.WriteVar is generic.

	node.WriteVar(req, input.Key, input.Value)

	if req.VariableTracker != nil {
		req.VariableTracker.TrackWrite(input.Key, input.Value)
	}

	return fmt.Sprintf("Successfully set variable '%s'", input.Key), nil
}

// discoverToolsTool creates the discover_tools function for PoC #3
// This allows the AI to dynamically learn about available tools
func discoverToolsTool() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name: "discover_tools",
			Description: `List all available tools and their detailed descriptions.
Call this to understand what tools are available and how to use them.
You can optionally filter by tool name pattern.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"filter": map[string]any{
						"type":        "string",
						"description": "Optional filter to match tool names (case-insensitive substring match). Leave empty to list all tools.",
					},
				},
				"required": []string{},
			},
		},
	}
}

// ToolInfo represents information about a tool for discovery
type ToolInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	RequiredVars []string `json:"required_variables,omitempty"`
	OutputVars   []string `json:"output_variables,omitempty"`
}

// handleDiscoverTools handles the discover_tools function call
func handleDiscoverTools(ctx context.Context, toolMap map[string]*NodeTool, args string) (string, error) {
	var input struct {
		Filter string `json:"filter"`
	}
	// Parse args - it's okay if it's empty
	if args != "" && args != "{}" {
		if err := json.Unmarshal([]byte(args), &input); err != nil {
			// Ignore parse errors, just use empty filter
			input.Filter = ""
		}
	}

	var tools []ToolInfo
	filterLower := strings.ToLower(input.Filter)

	for name, nodeTool := range toolMap {
		// Apply filter if specified
		if filterLower != "" && !strings.Contains(strings.ToLower(name), filterLower) {
			continue
		}

		info := ToolInfo{
			Name: name,
		}

		// Get description from DescribableNode if available
		if describable, ok := nodeTool.TargetNode.(DescribableNode); ok {
			if desc := describable.GetDescription(); desc != "" {
				info.Description = desc
			}
		}

		// Get variable info from VariableIntrospector if available
		if introspector, ok := nodeTool.TargetNode.(node.VariableIntrospector); ok {
			info.RequiredVars = introspector.GetRequiredVariables()
			info.OutputVars = introspector.GetOutputVariables()
		}

		// If no description yet, build a basic one
		if info.Description == "" {
			info.Description = fmt.Sprintf("Executes the '%s' node.", nodeTool.TargetNode.GetName())
			if len(info.RequiredVars) > 0 {
				info.Description += fmt.Sprintf(" Requires variables: %s.", strings.Join(info.RequiredVars, ", "))
			}
			if len(info.OutputVars) > 0 {
				outputExamples := info.OutputVars
				if len(outputExamples) > 3 {
					outputExamples = outputExamples[:3]
				}
				info.Description += fmt.Sprintf(" Outputs available at: %s.%s", nodeTool.TargetNode.GetName(), strings.Join(outputExamples, ", "+nodeTool.TargetNode.GetName()+"."))
			}
		}

		tools = append(tools, info)
	}

	// Also include built-in tools
	builtInTools := []ToolInfo{
		{
			Name:        "get_variable",
			Description: "Get the value of a variable from the flow context. Use to read data from node outputs.",
		},
		{
			Name:        "set_variable",
			Description: "Set a variable in the flow context. Use to pass data to nodes that require input variables.",
		},
	}
	tools = append(tools, builtInTools...)

	// Format output
	result, err := json.MarshalIndent(map[string]any{
		"available_tools": tools,
		"total_count":     len(tools),
		"usage_hint":      "Use set_variable to set required inputs before calling a tool. Use get_variable to read outputs after calling a tool.",
	}, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format tool list: %w", err)
	}

	return string(result), nil
}

// strings helper for case-insensitive contains
func strings_Contains_CI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
