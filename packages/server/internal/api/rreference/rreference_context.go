package rreference

import (
	"context"

	"connectrpc.com/connect"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/permcheck"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/sort/sortenabled"
)

// referenceContextMsg holds the proto-agnostic ID bytes shared by all three request types.
type referenceContextMsg struct {
	WorkspaceID []byte
	HttpID      []byte
	GraphqlID   []byte
	FlowNodeID  []byte
}

// resolveParams holds validated IDs parsed from a request.
type resolveParams struct {
	workspaceID *idwrap.IDWrap
	httpID      *idwrap.IDWrap
	graphqlID   *idwrap.IDWrap
	flowNodeID  *idwrap.IDWrap
}

// parseReferenceContext converts raw ID bytes into validated IDWraps.
func parseReferenceContext(msg referenceContextMsg) (resolveParams, error) {
	var p resolveParams
	for _, entry := range []struct {
		src  []byte
		dest **idwrap.IDWrap
	}{
		{msg.WorkspaceID, &p.workspaceID},
		{msg.HttpID, &p.httpID},
		{msg.GraphqlID, &p.graphqlID},
		{msg.FlowNodeID, &p.flowNodeID},
	} {
		if entry.src == nil {
			continue
		}
		id, err := idwrap.NewFromBytes(entry.src)
		if err != nil {
			return resolveParams{}, connect.NewError(connect.CodeInvalidArgument, err)
		}
		*entry.dest = &id
	}
	return p, nil
}

// --------------------------------------------------------------------------
// Flat variable map (used by Completion and Value endpoints)
// --------------------------------------------------------------------------

// resolveVarMap builds the flat variable map consumed by Completion and Value.
func (c *ReferenceServiceRPC) resolveVarMap(ctx context.Context, p resolveParams) (map[string]any, error) {
	varMap := make(map[string]any)

	if p.workspaceID != nil {
		envVars, err := c.fetchEnvVars(ctx, *p.workspaceID)
		if err != nil {
			return nil, err
		}
		for k, v := range envVars {
			varMap[k] = v
		}
	}

	if p.httpID != nil {
		if err := c.addHTTPVars(ctx, *p.httpID, varMap); err != nil {
			return nil, err
		}
	}

	if p.graphqlID != nil {
		if err := c.addGraphQLVars(ctx, *p.graphqlID, varMap); err != nil {
			return nil, err
		}
	}

	if p.flowNodeID != nil {
		if err := c.addFlowNodeVars(ctx, *p.flowNodeID, varMap); err != nil {
			return nil, err
		}
	}

	return varMap, nil
}

// fetchEnvVars returns workspace environment variables as a flat map.
func (c *ReferenceServiceRPC) fetchEnvVars(ctx context.Context, wsID idwrap.IDWrap) (map[string]any, error) {
	rpcErr := permcheck.CheckPerm(true, mwauth.CheckOwnerWorkspaceWithReader(ctx, c.userReader, wsID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	envs, err := c.envReader.ListEnvironments(ctx, wsID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, ErrWorkspaceNotFound)
	}

	result := make(map[string]any)
	for _, env := range envs {
		vars, err := c.varReader.GetVariableByEnvID(ctx, env.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
		}
		sortenabled.GetAllWithState(&vars, true)
		for _, v := range vars {
			result[v.VarKey] = v.Value
		}
	}
	return result, nil
}

// addHTTPVars adds HTTP response/request variables to the var map.
func (c *ReferenceServiceRPC) addHTTPVars(ctx context.Context, httpID idwrap.IDWrap, varMap map[string]any) error {
	resp, err := c.getLatestResponse(ctx, httpID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if resp != nil {
		varMap["response"] = resp
	} else {
		varMap["response"] = defaultHTTPResponseSchema()
	}
	varMap["request"] = defaultHTTPRequestSchema()
	return nil
}

// addGraphQLVars adds GraphQL response + convenience variables to the var map.
func (c *ReferenceServiceRPC) addGraphQLVars(ctx context.Context, graphqlID idwrap.IDWrap, varMap map[string]any) error {
	resp, err := c.getLatestGraphQLResponse(ctx, graphqlID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	if resp != nil {
		varMap["response"] = resp
		addGraphQLConvenienceVars(resp, varMap)
	} else {
		varMap["response"] = defaultGraphQLResponseSchema()
		varMap["data"] = map[string]any{}
		varMap["status"] = 200
		varMap["success"] = true
		varMap["has_data"] = false
		varMap["has_errors"] = false
	}
	return nil
}

// addFlowNodeVars resolves upstream node outputs, flow variables, and self-reference.
func (c *ReferenceServiceRPC) addFlowNodeVars(ctx context.Context, nodeID idwrap.IDWrap, varMap map[string]any) error {
	fc, err := c.fetchFlowNodeContext(ctx, nodeID)
	if err != nil {
		return err
	}

	// Flow variables
	for _, fv := range fc.flowVars {
		varMap[fv.Name] = fv.Value
	}

	// Process upstream nodes
	for _, node := range fc.upstream {
		nodeData, ok := c.getNodeExecutionOutput(ctx, node)
		if ok {
			addExecutionDataToVarMap(nodeData, node.Name, varMap)
			continue
		}
		if schema, ok := nodeDefaultSchema(node.NodeKind); ok {
			varMap[node.Name] = schema
		} else if node.NodeKind == mflow.NODE_KIND_SUB_FLOW_TRIGGER {
			varMap[node.Name] = c.subFlowTriggerParamMap(ctx, node.ID)
		}
	}

	// Self-reference for current node
	c.addSelfReference(ctx, *fc.currentNode, varMap)

	return nil
}

// addSelfReference adds self-reference variables for the current node.
// FOR/FOREACH can reference their own index/item/key.
// REQUEST/GRAPHQL can reference their own request/response at root level.
func (c *ReferenceServiceRPC) addSelfReference(ctx context.Context, node mflow.Node, varMap map[string]any) {
	switch node.NodeKind {
	case mflow.NODE_KIND_FOR, mflow.NODE_KIND_FOR_EACH:
		nodeData, ok := c.getNodeExecutionOutput(ctx, node)
		if ok {
			varMap[node.Name] = nodeData
		} else if schema, ok := nodeDefaultSchema(node.NodeKind); ok {
			varMap[node.Name] = schema
		}

	case mflow.NODE_KIND_REQUEST, mflow.NODE_KIND_GRAPHQL:
		nodeData, ok := c.getNodeExecutionOutput(ctx, node)
		if ok {
			addExecutionDataToVarMapFlat(nodeData, node.Name, varMap)
		} else {
			schema, ok := nodeDefaultSchema(node.NodeKind)
			if !ok {
				return
			}
			for k, v := range schema {
				varMap[k] = v
			}
		}

	default:
		// Other node types don't have self-reference schemas.
	}
}

// findUpstreamNodes returns all nodes that are upstream of the given target node.
func (c *ReferenceServiceRPC) findUpstreamNodes(ctx context.Context, flowID, targetNodeID idwrap.IDWrap, allNodes []mflow.Node) ([]mflow.Node, error) {
	edges, err := c.flowEdgeReader.GetEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	edgesMap := mflow.NewEdgesMap(edges)

	result := make([]mflow.Node, 0, len(allNodes))
	for _, node := range allNodes {
		if mflow.IsNodeCheckTarget(edgesMap, node.ID, targetNodeID) == mflow.NodeBefore {
			result = append(result, node)
		}
	}
	return result, nil
}
