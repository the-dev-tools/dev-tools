package sflow

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeForEachWriter struct {
	queries *gen.Queries
}

func NewNodeForEachWriter(tx gen.DBTX) *NodeForEachWriter {
	return &NodeForEachWriter{queries: gen.New(tx)}
}

func NewNodeForEachWriterFromQueries(queries *gen.Queries) *NodeForEachWriter {
	return &NodeForEachWriter{queries: queries}
}

func (w *NodeForEachWriter) CreateNodeForEach(ctx context.Context, nf mflow.NodeForEach) error {
	nodeForEach := ConvertNodeForEachToDB(nf)
	return w.queries.CreateFlowNodeForEach(ctx, gen.CreateFlowNodeForEachParams(nodeForEach))
}

func (w *NodeForEachWriter) CreateNodeForEachBulk(ctx context.Context, forEachNodes []mflow.NodeForEach) error {
	var err error
	for _, forEachNode := range forEachNodes {
		err = w.CreateNodeForEach(ctx, forEachNode)
		if err != nil {
			break
		}
	}
	return err
}

func (w *NodeForEachWriter) UpdateNodeForEach(ctx context.Context, nf mflow.NodeForEach) error {
	nodeForEach := ConvertNodeForEachToDB(nf)
	return w.queries.UpdateFlowNodeForEach(ctx, gen.UpdateFlowNodeForEachParams{
		FlowNodeID:     nodeForEach.FlowNodeID,
		IterExpression: nodeForEach.IterExpression,
		ErrorHandling:  nodeForEach.ErrorHandling,
		Expression:     nodeForEach.Expression,
	})
}

func (w *NodeForEachWriter) DeleteNodeForEach(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeForEach(ctx, id)
}
