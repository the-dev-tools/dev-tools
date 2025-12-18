package snodeif

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriter(tx gen.DBTX) *Writer {
	return &Writer{queries: gen.New(tx)}
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{queries: queries}
}

func (w *Writer) CreateNodeIf(ctx context.Context, ni mflow.NodeIf) error {
	nodeIf := ConvertToDBNodeIf(ni)
	return w.queries.CreateFlowNodeCondition(ctx, gen.CreateFlowNodeConditionParams{
		FlowNodeID: nodeIf.FlowNodeID,
		Expression: ni.Condition.Comparisons.Expression,
	})
}

func (w *Writer) CreateNodeIfBulk(ctx context.Context, conditionNodes []mflow.NodeIf) error {
	var err error
	for _, n := range conditionNodes {
		err = w.CreateNodeIf(ctx, n)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) UpdateNodeIf(ctx context.Context, ni mflow.NodeIf) error {
	nodeIf := ConvertToDBNodeIf(ni)
	return w.queries.UpdateFlowNodeCondition(ctx, gen.UpdateFlowNodeConditionParams{
		FlowNodeID: nodeIf.FlowNodeID,
		Expression: nodeIf.Expression,
	})
}

func (w *Writer) DeleteNodeIf(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeCondition(ctx, id)
}
