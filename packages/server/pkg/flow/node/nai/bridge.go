package nai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
)

// NodeTool wraps any FlowNode to be used by LangChain agents.
type NodeTool struct {
	TargetNode node.FlowNode
	Req        *node.FlowNodeRequest
}

func NewNodeTool(target node.FlowNode, req *node.FlowNodeRequest) *NodeTool {
	return &NodeTool{
		TargetNode: target,
		Req:        req,
	}
}

func (nt *NodeTool) AsLangChainTool() llms.Tool {
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        nt.TargetNode.GetName(),
			Description: fmt.Sprintf("Executes the flow node: %s", nt.TargetNode.GetName()),
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

func (nt *NodeTool) Execute(ctx context.Context, args string) (string, error) {
	result := nt.TargetNode.RunSync(ctx, nt.Req)
	if result.Err != nil {
		return "", fmt.Errorf("node execution failed: %w", result.Err)
	}

	// Extract node output from VarMap
	nodeName := nt.TargetNode.GetName()
	if output, ok := nt.Req.VarMap[nodeName]; ok {
		jsonBytes, err := json.Marshal(output)
		if err != nil {
			return fmt.Sprintf("Node '%s' executed successfully. Output not serializable: %v", nodeName, err), nil
		}
		return string(jsonBytes), nil
	}

	return fmt.Sprintf("Node '%s' executed successfully. No output captured.", nodeName), nil
}
