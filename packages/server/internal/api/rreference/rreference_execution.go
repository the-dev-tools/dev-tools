package rreference

import (
	"context"
	"encoding/json"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// getNodeExecutionOutput retrieves the latest execution output for a node.
// Returns the parsed JSON output and true, or nil and false if unavailable.
func (c *ReferenceServiceRPC) getNodeExecutionOutput(ctx context.Context, node mflow.Node) (any, bool) {
	executions, err := c.nodeExecutionReader.GetNodeExecutionsByNodeID(ctx, node.ID)
	if err != nil || len(executions) == 0 {
		return nil, false
	}

	latest := executions[0]
	outputJSON, err := latest.GetOutputJSON()
	if err != nil || len(outputJSON) == 0 {
		return nil, false
	}

	var parsed any
	if err := json.Unmarshal(outputJSON, &parsed); err != nil {
		return nil, false
	}
	return parsed, true
}

// addExecutionDataToVarMap adds execution data to the variable map, extracting
// node-specific data from the tree structure when available.
// Execution data may look like {"NodeName": {"key": "value"}} — this extracts
// just the subtree for the given node name.
func addExecutionDataToVarMap(data any, nodeName string, varMap map[string]any) {
	if nodeMap, ok := data.(map[string]any); ok {
		if nodeSpecific, hasKey := nodeMap[nodeName]; hasKey {
			varMap[nodeName] = nodeSpecific
			return
		}
	}
	varMap[nodeName] = data
}

// addExecutionDataToVarMapFlat extracts a node's execution data and adds its
// sub-keys directly at root level. Used for self-referencing nodes
// (REQUEST, GRAPHQL) where the user writes `response.status` not `myNode.response.status`.
func addExecutionDataToVarMapFlat(data any, nodeName string, varMap map[string]any) {
	if nodeMap, ok := data.(map[string]any); ok {
		if nodeSpecific, hasKey := nodeMap[nodeName]; hasKey {
			if nodeVars, ok := nodeSpecific.(map[string]any); ok {
				for k, v := range nodeVars {
					varMap[k] = v
				}
				return
			}
		}
	}
}
