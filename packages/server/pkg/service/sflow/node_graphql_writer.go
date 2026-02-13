package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeGraphQLWriter struct {
	queries *gen.Queries
}

func NewNodeGraphQLWriter(tx gen.DBTX) *NodeGraphQLWriter {
	return &NodeGraphQLWriter{queries: gen.New(tx)}
}

func NewNodeGraphQLWriterFromQueries(queries *gen.Queries) *NodeGraphQLWriter {
	return &NodeGraphQLWriter{queries: queries}
}

func (w *NodeGraphQLWriter) CreateNodeGraphQL(ctx context.Context, ng mflow.NodeGraphQL) error {
	dbModel, ok := ConvertToDBNodeGraphQL(ng)
	if !ok {
		return nil
	}
	return w.queries.CreateFlowNodeGraphQL(ctx, gen.CreateFlowNodeGraphQLParams(dbModel))
}

func (w *NodeGraphQLWriter) UpdateNodeGraphQL(ctx context.Context, ng mflow.NodeGraphQL) error {
	dbModel, ok := ConvertToDBNodeGraphQL(ng)
	if !ok {
		// Treat removal of GraphQLID as request to delete any existing binding.
		if err := w.queries.DeleteFlowNodeGraphQL(ctx, ng.FlowNodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return nil
	}
	return w.queries.UpdateFlowNodeGraphQL(ctx, gen.UpdateFlowNodeGraphQLParams(dbModel))
}

func (w *NodeGraphQLWriter) DeleteNodeGraphQL(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteFlowNodeGraphQL(ctx, id)
}
