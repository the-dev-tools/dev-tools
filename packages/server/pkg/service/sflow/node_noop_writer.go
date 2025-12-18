package sflow

import (
	"context"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type NodeNoopWriter struct {
	queries *gen.Queries
}

func NewNodeNoopWriter(tx gen.DBTX) *NodeNoopWriter {
	return &NodeNoopWriter{queries: gen.New(tx)}
}

func NewNodeNoopWriterFromQueries(queries *gen.Queries) *NodeNoopWriter {
	return &NodeNoopWriter{queries: queries}
}

func (w *NodeNoopWriter) CreateNodeNoop(ctx context.Context, nf mflow.NodeNoop) error {
	convertedNode := ConvertToDBNodeStart(nf)
	return w.queries.CreateFlowNodeNoop(ctx, gen.CreateFlowNodeNoopParams(convertedNode))
}

func (w *NodeNoopWriter) CreateNodeNoopBulk(ctx context.Context, nf []mflow.NodeNoop) error {
	for _, n := range nf {
		convertedNode := ConvertToDBNodeStart(n)
		err := w.queries.CreateFlowNodeNoop(ctx, gen.CreateFlowNodeNoopParams(convertedNode))
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *NodeNoopWriter) UpdateNodeNoop(ctx context.Context, nf mflow.NodeNoop) error {
	// Since there's no UpdateFlowNodeNoop query, we'll use delete + create pattern
	// Note: In strict segregation, using w.DeleteNodeNoop directly is fine as it's within the NodeNoopWriter struct
	if err := w.DeleteNodeNoop(ctx, nf.FlowNodeID); err != nil && !errors.Is(err, ErrNoNodeNoopFound) {
		return err
	}
	return w.CreateNodeNoop(ctx, nf)
}

func (w *NodeNoopWriter) DeleteNodeNoop(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlowNodeNoop(ctx, id)
	return err // In NodeNoopWriter context, we might return DB errors directly or handle NoRows if needed
}
