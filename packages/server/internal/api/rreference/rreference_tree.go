package rreference

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	"github.com/the-dev-tools/dev-tools/packages/server/internal/api/middleware/mwauth"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/menv"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/permcheck"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/reference"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/sort/sortenabled"
	referencev1 "github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go/api/reference/v1"
)

// resolveTree builds the ReferenceTreeItem list for the Tree endpoint.
func (c *ReferenceServiceRPC) resolveTree(ctx context.Context, p resolveParams) ([]*referencev1.ReferenceTreeItem, error) {
	var items []*referencev1.ReferenceTreeItem

	if p.workspaceID != nil {
		envItems, err := c.buildEnvTreeItems(ctx, *p.workspaceID)
		if err != nil {
			return nil, err
		}
		items = append(items, envItems...)
	}

	if p.httpID != nil {
		httpItems, err := c.buildHTTPTreeItems(ctx, *p.httpID)
		if err != nil {
			return nil, err
		}
		items = append(items, httpItems...)
	}

	if p.flowNodeID != nil {
		nodeItems, err := c.buildFlowNodeTreeItems(ctx, *p.flowNodeID)
		if err != nil {
			return nil, err
		}
		items = append(items, nodeItems...)
	}

	return items, nil
}

// buildEnvTreeItems builds the "env" group tree item with environment name metadata.
func (c *ReferenceServiceRPC) buildEnvTreeItems(ctx context.Context, wsID idwrap.IDWrap) ([]*referencev1.ReferenceTreeItem, error) {
	rpcErr := permcheck.CheckPerm(true, mwauth.CheckOwnerWorkspaceWithReader(ctx, c.userReader, wsID))
	if rpcErr != nil {
		return nil, rpcErr
	}

	envs, err := c.envReader.ListEnvironments(ctx, wsID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, ErrWorkspaceNotFound)
	}

	// Track which environments define each variable
	present := make(map[string][]menv.Env)
	var allVars []menv.Variable

	for _, env := range envs {
		vars, err := c.varReader.GetVariableByEnvID(ctx, env.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, ErrEnvNotFound)
		}
		sortenabled.GetAllWithState(&vars, true)
		for _, v := range vars {
			present[v.VarKey] = append(present[v.VarKey], env)
		}
		allVars = append(allVars, vars...)
	}

	envMap := make([]*referencev1.ReferenceTreeItem, 0, len(allVars))
	for _, v := range allVars {
		var envNames []string
		for _, env := range present[v.VarKey] {
			envNames = append(envNames, env.Name)
		}
		envMap = append(envMap, &referencev1.ReferenceTreeItem{
			Key: &referencev1.ReferenceKey{
				Kind: referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_KEY,
				Key:  &v.VarKey,
			},
			Kind:     referencev1.ReferenceKind_REFERENCE_KIND_VARIABLE,
			Variable: envNames,
		})
	}

	groupStr := "env"
	return []*referencev1.ReferenceTreeItem{{
		Key: &referencev1.ReferenceKey{
			Kind:  referencev1.ReferenceKeyKind_REFERENCE_KEY_KIND_GROUP,
			Group: &groupStr,
		},
		Kind: referencev1.ReferenceKind_REFERENCE_KIND_MAP,
		Map:  envMap,
	}}, nil
}

// buildHTTPTreeItems builds tree items for an HTTP response.
func (c *ReferenceServiceRPC) buildHTTPTreeItems(ctx context.Context, httpID idwrap.IDWrap) ([]*referencev1.ReferenceTreeItem, error) {
	resp, err := c.getLatestResponse(ctx, httpID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if resp == nil {
		return nil, nil
	}

	respRef := reference.NewReferenceFromInterfaceWithKey(resp, "response")
	converted, err := reference.ConvertPkgToRpcTree(respRef)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return []*referencev1.ReferenceTreeItem{converted}, nil
}

// buildFlowNodeTreeItems builds tree items for flow node context (upstream nodes + flow vars).
func (c *ReferenceServiceRPC) buildFlowNodeTreeItems(ctx context.Context, nodeID idwrap.IDWrap) ([]*referencev1.ReferenceTreeItem, error) {
	fc, err := c.fetchFlowNodeContext(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	var items []*referencev1.ReferenceTreeItem

	appendRef := func(data any, name string) error {
		ref := reference.NewReferenceFromInterfaceWithKey(data, name)
		converted, err := reference.ConvertPkgToRpcTree(ref)
		if err != nil {
			return fmt.Errorf("convert %q: %w", name, err)
		}
		items = append(items, converted)
		return nil
	}

	// Flow variables
	for _, fv := range fc.flowVars {
		if err := appendRef(fv.Value, fv.Name); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	// Upstream nodes
	for _, node := range fc.upstream {
		nodeData, ok := c.getNodeExecutionOutput(ctx, node)
		if ok {
			// Extract node-specific data from execution output
			if nodeMap, ok := nodeData.(map[string]any); ok {
				if nodeSpecific, hasKey := nodeMap[node.Name]; hasKey {
					if err := appendRef(nodeSpecific, node.Name); err != nil {
						return nil, connect.NewError(connect.CodeInternal, err)
					}
					continue
				}
			}
			if err := appendRef(nodeData, node.Name); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			continue
		}

		// Schema fallback
		if schema, ok := nodeDefaultSchema(node.NodeKind); ok {
			if err := appendRef(schema, node.Name); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		} else if node.NodeKind == mflow.NODE_KIND_SUB_FLOW_TRIGGER {
			paramMap := c.subFlowTriggerParamMap(ctx, node.ID)
			if err := appendRef(paramMap, node.Name); err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
		// SUB_FLOW_RETURN and unknown kinds: no output.
	}

	return items, nil
}
