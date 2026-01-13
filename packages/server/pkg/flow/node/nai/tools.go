package nai

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

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
