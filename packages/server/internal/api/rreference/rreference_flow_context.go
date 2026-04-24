package rreference

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/sort/sortenabled"
)

// flowNodeContext holds pre-fetched data shared by both the varMap and tree pipelines.
type flowNodeContext struct {
	currentNode *mflow.Node
	allNodes    []mflow.Node
	flowVars    []mflow.FlowVariable
	upstream    []mflow.Node
}

// fetchFlowNodeContext loads the current node, all sibling nodes, enabled flow variables,
// and the upstream subset. Both addFlowNodeVars and buildFlowNodeTreeItems call this
// instead of duplicating the same fetch-and-filter sequence.
func (c *ReferenceServiceRPC) fetchFlowNodeContext(ctx context.Context, nodeID idwrap.IDWrap) (*flowNodeContext, error) {
	currentNode, err := c.nodeReader.GetNode(ctx, nodeID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	flowID := currentNode.FlowID

	allNodes, err := c.nodeReader.GetNodesByFlowID(ctx, flowID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	flowVars, err := c.flowVariableReader.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil {
		if !errors.Is(err, sflow.ErrNoFlowVariableFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		flowVars = []mflow.FlowVariable{}
	}
	sortenabled.GetAllWithState(&flowVars, true)

	upstream, err := c.findUpstreamNodes(ctx, flowID, nodeID, allNodes)
	if err != nil {
		return nil, err
	}

	return &flowNodeContext{
		currentNode: currentNode,
		allNodes:    allNodes,
		flowVars:    flowVars,
		upstream:    upstream,
	}, nil
}
