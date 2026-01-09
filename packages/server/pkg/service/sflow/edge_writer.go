package sflow

import (
	"context"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
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
	return w.queries.CreateFlowEdge(ctx, gen.CreateFlowEdgeParams{
		ID:           edge.ID,
		FlowID:       edge.FlowID,
		SourceID:     edge.SourceID,
		TargetID:     edge.TargetID,
		SourceHandle: edge.SourceHandle,
	})
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
	})
}
func (w *EdgeWriter) DeleteEdge(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlowEdge(ctx, id)
	if err != nil {
		return err
	}
	return nil
}

func (w *EdgeWriter) UpdateEdgeState(ctx context.Context, id idwrap.IDWrap, state mflow.NodeState) error {
	// Validate state is within valid range [0,4]
	if state < mflow.NODE_STATE_UNSPECIFIED || state > mflow.NODE_STATE_CANCELED {
		return fmt.Errorf("invalid edge state: %d", state)
	}
	return w.queries.UpdateFlowEdgeState(ctx, gen.UpdateFlowEdgeStateParams{
		ID:    id,
		State: state,
	})
}
