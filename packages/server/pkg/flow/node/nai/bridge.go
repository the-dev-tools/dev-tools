package nai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tmc/langchaingo/llms"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/flow/node"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// ToolExecuteResult contains the result of a tool execution
type ToolExecuteResult struct {
	Output      string
	OutputData  any
	AuxiliaryID *idwrap.IDWrap
	Err         error
}

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
	name := sanitizeToolName(nt.TargetNode.GetName())
	nodeName := nt.TargetNode.GetName()
	// Description explains that this executes ONLY this node (not any downstream nodes)
	// and how to access the output via get_variable
	description := fmt.Sprintf(
		"Executes the flow node '%s'. This runs ONLY this specific node, not any nodes connected after it. "+
			"After execution, the node's output is available via get_variable using paths like '%s.response.body' (for HTTP nodes) "+
			"or '%s.<field>' for other node types. The tool returns the JSON output directly.",
		nodeName, nodeName, nodeName,
	)
	return llms.Tool{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        name,
			Description: description,
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// Execute runs the tool and returns a string result (for LLM consumption)
func (nt *NodeTool) Execute(ctx context.Context, args string) (string, error) {
	result := nt.ExecuteWithDetails(ctx, args)
	return result.Output, result.Err
}

// ExecuteWithDetails runs the tool and returns full execution details including AuxiliaryID
func (nt *NodeTool) ExecuteWithDetails(ctx context.Context, args string) ToolExecuteResult {
	result := nt.TargetNode.RunSync(ctx, nt.Req)
	if result.Err != nil {
		return ToolExecuteResult{
			Err: fmt.Errorf("node execution failed: %w", result.Err),
		}
	}

	// Debug: Log what we got from the node's RunSync
	if nt.Req.Logger != nil {
		if result.AuxiliaryID != nil {
			nt.Req.Logger.Debug("NodeTool.ExecuteWithDetails received AuxiliaryID",
				"tool_name", nt.TargetNode.GetName(),
				"auxiliary_id", result.AuxiliaryID.String(),
			)
		} else {
			nt.Req.Logger.Debug("NodeTool.ExecuteWithDetails received no AuxiliaryID",
				"tool_name", nt.TargetNode.GetName(),
			)
		}
	}

	// Extract node output from VarMap
	nodeName := nt.TargetNode.GetName()
	var output string
	var outputData any

	if data, ok := nt.Req.VarMap[nodeName]; ok {
		outputData = data
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			output = fmt.Sprintf("Node '%s' executed successfully. Output not serializable: %v", nodeName, err)
		} else {
			output = string(jsonBytes)
		}
	} else {
		output = fmt.Sprintf("Node '%s' executed successfully. No output captured.", nodeName)
	}

	return ToolExecuteResult{
		Output:      output,
		OutputData:  outputData,
		AuxiliaryID: result.AuxiliaryID,
		Err:         nil,
	}
}
