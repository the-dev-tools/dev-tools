package sedge

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/flow/edge"
	"the-dev-tools/server/pkg/idwrap"
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

func (w *Writer) CreateEdge(ctx context.Context, e edge.Edge) error {
	edge := ConvertToDBEdge(e)
	return w.queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams(edge))
}

func (w *Writer) CreateEdgeBulk(ctx context.Context, edges []edge.Edge) error {
	for _, e := range edges {
		err := w.CreateEdge(ctx, e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) UpdateEdge(ctx context.Context, e edge.Edge) error {
	edge := ConvertToDBEdge(e)
	return w.queries.UpdateFlowEdge(ctx, gen.UpdateFlowEdgeParams{
		ID:           edge.ID,
		SourceID:     edge.SourceID,
		TargetID:     edge.TargetID,
		SourceHandle: edge.SourceHandle,
		EdgeKind:     edge.EdgeKind,
	})
}

func (w *Writer) DeleteEdge(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlowEdge(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
