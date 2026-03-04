package sflow

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type EdgeReader struct {
	queries *gen.Queries
}

func NewEdgeReader(db *sql.DB) *EdgeReader {
	return &EdgeReader{queries: gen.New(db)}
}

func NewEdgeReaderFromQueries(queries *gen.Queries) *EdgeReader {
	return &EdgeReader{queries: queries}
}

func (r *EdgeReader) GetEdge(ctx context.Context, id idwrap.IDWrap) (*mflow.Edge, error) {
	edge, err := r.queries.GetFlowEdge(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelEdge(edge), nil
}

func (r *EdgeReader) GetEdgesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.Edge, error) {
	edge, err := r.queries.GetFlowEdgesByFlowID(ctx, flowID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvertPtr(edge, ConvertToModelEdge), nil
}

func (r *EdgeReader) GetEdgesByNodeIDs(ctx context.Context, nodeIDs []idwrap.IDWrap) ([]mflow.Edge, error) {
	sourceEdges, err := r.queries.GetFlowEdgesBySourceNodeIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}
	targetEdges, err := r.queries.GetFlowEdgesByTargetNodeIDs(ctx, nodeIDs)
	if err != nil {
		return nil, err
	}

	// Deduplicate: an edge may appear in both results if both source and target are in nodeIDs
	seen := make(map[idwrap.IDWrap]struct{}, len(sourceEdges))
	result := make([]mflow.Edge, 0, len(sourceEdges)+len(targetEdges))
	for i := range sourceEdges {
		edge := ConvertToModelEdge(sourceEdges[i])
		seen[edge.ID] = struct{}{}
		result = append(result, *edge)
	}
	for i := range targetEdges {
		edge := ConvertToModelEdge(targetEdges[i])
		if _, ok := seen[edge.ID]; !ok {
			result = append(result, *edge)
		}
	}
	return result, nil
}
