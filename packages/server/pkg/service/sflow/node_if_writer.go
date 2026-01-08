package sflow

import (
	"context"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeIfWriter struct {
	queries *gen.Queries
}

func NewNodeIfWriter(tx gen.DBTX) *NodeIfWriter {
	return &NodeIfWriter{queries: gen.New(tx)}
}

func NewNodeIfWriterFromQueries(queries *gen.Queries) *NodeIfWriter {
	return &NodeIfWriter{queries: queries}
}

func (w *NodeIfWriter) CreateNodeIf(ctx context.Context, ni mflow.NodeIf) error {
	nodeIf := ConvertToDBNodeIf(ni)
	return w.queries.CreateFlowNodeCondition(ctx, gen.CreateFlowNodeConditionParams{
		FlowNodeID: nodeIf.FlowNodeID,
		Expression: ni.Condition.Comparisons.Expression,
	})
}

func (w *NodeIfWriter) CreateNodeIfBulk(ctx context.Context, conditionNodes []mflow.NodeIf) error {
	var err error
	for _, n := range conditionNodes {
		err = w.CreateNodeIf(ctx, n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *NodeIfWriter) UpdateNodeIf(ctx context.Context, ni mflow.NodeIf) error {
	nodeIf := ConvertToDBNodeIf(ni)
	return w.queries.UpdateFlowNodeCondition(ctx, gen.UpdateFlowNodeConditionParams{
		FlowNodeID: nodeIf.FlowNodeID,
		Expression: nodeIf.Expression,
	})
}

func (w *NodeIfWriter) DeleteNodeIf(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeCondition(ctx, id)
}
