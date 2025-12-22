//nolint:revive // exported
package ioworkspace

import (
	"context"
	"fmt"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/service/sflow"
)

// importFlows imports flows from the bundle.
func (s *IOWorkspaceService) importFlows(ctx context.Context, flowService sflow.FlowService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, flow := range bundle.Flows {
		oldID := flow.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			flow.ID = idwrap.NewNow()
		}

		// Update workspace ID
		flow.WorkspaceID = opts.WorkspaceID

		// Update version parent ID if it exists in the mapping
		if flow.VersionParentID != nil {
			if newParentID, ok := result.FlowIDMap[*flow.VersionParentID]; ok {
				flow.VersionParentID = &newParentID
			}
		}

		// Create flow
		if err := flowService.CreateFlow(ctx, flow); err != nil {
			return fmt.Errorf("failed to create flow %s: %w", flow.Name, err)
		}

		// Track ID mapping
		result.FlowIDMap[oldID] = flow.ID
		result.FlowsCreated++
	}
	return nil
}

// importFlowVariables imports flow variables from the bundle.
func (s *IOWorkspaceService) importFlowVariables(ctx context.Context, flowVariableService sflow.FlowVariableService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, flowVar := range bundle.FlowVariables {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			flowVar.ID = idwrap.NewNow()
		}

		// Remap flow ID
		if newFlowID, ok := result.FlowIDMap[flowVar.FlowID]; ok {
			flowVar.FlowID = newFlowID
		}

		// Create flow variable
		if err := flowVariableService.CreateFlowVariable(ctx, flowVar); err != nil {
			return fmt.Errorf("failed to create flow variable %s: %w", flowVar.Name, err)
		}

		result.FlowVariablesCreated++
	}
	return nil
}

// importFlowNodes imports flow nodes from the bundle.
func (s *IOWorkspaceService) importFlowNodes(ctx context.Context, nodeService sflow.NodeService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, node := range bundle.FlowNodes {
		oldID := node.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			node.ID = idwrap.NewNow()
		}

		// Remap flow ID
		if newFlowID, ok := result.FlowIDMap[node.FlowID]; ok {
			node.FlowID = newFlowID
		}

		// Create node
		if err := nodeService.CreateNode(ctx, node); err != nil {
			return fmt.Errorf("failed to create node %s: %w", node.Name, err)
		}

		// Track ID mapping
		result.NodeIDMap[oldID] = node.ID
		result.FlowNodesCreated++
	}
	return nil
}

// importFlowEdges imports flow edges from the bundle.
func (s *IOWorkspaceService) importFlowEdges(ctx context.Context, edgeService sflow.EdgeService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, edge := range bundle.FlowEdges {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			edge.ID = idwrap.NewNow()
		}

		// Remap flow ID
		if newFlowID, ok := result.FlowIDMap[edge.FlowID]; ok {
			edge.FlowID = newFlowID
		}

		// Remap source and target node IDs
		if newSourceID, ok := result.NodeIDMap[edge.SourceID]; ok {
			edge.SourceID = newSourceID
		}
		if newTargetID, ok := result.NodeIDMap[edge.TargetID]; ok {
			edge.TargetID = newTargetID
		}

		// Create edge
		if err := edgeService.CreateEdge(ctx, edge); err != nil {
			return fmt.Errorf("failed to create flow edge: %w", err)
		}

		result.FlowEdgesCreated++
	}
	return nil
}

// importFlowRequestNodes imports flow request nodes from the bundle.
func (s *IOWorkspaceService) importFlowRequestNodes(ctx context.Context, nodeRequestService sflow.NodeRequestService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, requestNode := range bundle.FlowRequestNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[requestNode.FlowNodeID]; ok {
			requestNode.FlowNodeID = newNodeID
		}

		// Remap HTTP ID
		if requestNode.HttpID != nil {
			if newHTTPID, ok := result.HTTPIDMap[*requestNode.HttpID]; ok {
				requestNode.HttpID = &newHTTPID
			}
		}

		// Remap delta HTTP ID
		if requestNode.DeltaHttpID != nil {
			if newDeltaHTTPID, ok := result.HTTPIDMap[*requestNode.DeltaHttpID]; ok {
				requestNode.DeltaHttpID = &newDeltaHTTPID
			}
		}

		// Create request node
		if err := nodeRequestService.CreateNodeRequest(ctx, requestNode); err != nil {
			return fmt.Errorf("failed to create flow request node: %w", err)
		}

		result.FlowRequestNodesCreated++
	}
	return nil
}

// importFlowConditionNodes imports flow condition nodes from the bundle.
func (s *IOWorkspaceService) importFlowConditionNodes(ctx context.Context, nodeIfService *sflow.NodeIfService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, conditionNode := range bundle.FlowConditionNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[conditionNode.FlowNodeID]; ok {
			conditionNode.FlowNodeID = newNodeID
		}

		// Create condition node
		if err := nodeIfService.CreateNodeIf(ctx, conditionNode); err != nil {
			return fmt.Errorf("failed to create flow condition node: %w", err)
		}

		result.FlowConditionNodesCreated++
	}
	return nil
}

// importFlowForNodes imports flow for nodes from the bundle.
func (s *IOWorkspaceService) importFlowForNodes(ctx context.Context, nodeForService sflow.NodeForService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, forNode := range bundle.FlowForNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[forNode.FlowNodeID]; ok {
			forNode.FlowNodeID = newNodeID
		}

		// Create for node
		if err := nodeForService.CreateNodeFor(ctx, forNode); err != nil {
			return fmt.Errorf("failed to create flow for node: %w", err)
		}

		result.FlowForNodesCreated++
	}
	return nil
}

// importFlowForEachNodes imports flow foreach nodes from the bundle.
func (s *IOWorkspaceService) importFlowForEachNodes(ctx context.Context, nodeForEachService sflow.NodeForEachService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, forEachNode := range bundle.FlowForEachNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[forEachNode.FlowNodeID]; ok {
			forEachNode.FlowNodeID = newNodeID
		}

		// Create foreach node
		if err := nodeForEachService.CreateNodeForEach(ctx, forEachNode); err != nil {
			return fmt.Errorf("failed to create flow foreach node: %w", err)
		}

		result.FlowForEachNodesCreated++
	}
	return nil
}

// importFlowJSNodes imports flow JS nodes from the bundle.
func (s *IOWorkspaceService) importFlowJSNodes(ctx context.Context, nodeJSService sflow.NodeJsService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, jsNode := range bundle.FlowJSNodes {
		// Remap flow node ID
		if newNodeID, ok := result.NodeIDMap[jsNode.FlowNodeID]; ok {
			jsNode.FlowNodeID = newNodeID
		}

		// Create JS node
		if err := nodeJSService.CreateNodeJS(ctx, jsNode); err != nil {
			return fmt.Errorf("failed to create flow JS node: %w", err)
		}

		result.FlowJSNodesCreated++
	}
	return nil
}
