package sflow

import (
	"context"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type EdgeWriter struct {
	queries *gen.Queries
}

func NewEdgeWriter(tx gen.DBTX) *EdgeWriter {
	return &EdgeWriter{queries: gen.New(tx)}
}

func NewEdgeWriterFromQueries(queries *gen.Queries) *EdgeWriter {
	return &EdgeWriter{queries: queries}
}

func (w *EdgeWriter) CreateEdge(ctx context.Context, e mflow.Edge) error {
	edge := ConvertToDBEdge(e)
	return w.queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams(edge))
}

func (w *EdgeWriter) CreateEdgeBulk(ctx context.Context, edges []mflow.Edge) error {
	for _, e := range edges {
		err := w.CreateEdge(ctx, e)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *EdgeWriter) UpdateEdge(ctx context.Context, e mflow.Edge) error {
	edge := ConvertToDBEdge(e)
	return w.queries.UpdateFlowEdge(ctx, gen.UpdateFlowEdgeParams{
		ID:           edge.ID,
		SourceID:     edge.SourceID,
		TargetID:     edge.TargetID,
		SourceHandle: edge.SourceHandle,
		EdgeKind:     edge.EdgeKind,
	})
}

func (w *EdgeWriter) DeleteEdge(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlowEdge(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
