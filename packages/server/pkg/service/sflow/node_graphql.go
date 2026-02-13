//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeGraphQLService struct {
	reader  *NodeGraphQLReader
	queries *gen.Queries
}

func NewNodeGraphQLService(queries *gen.Queries) NodeGraphQLService {
	return NodeGraphQLService{
		reader:  NewNodeGraphQLReaderFromQueries(queries),
		queries: queries,
	}
}

func (ngs NodeGraphQLService) TX(tx *sql.Tx) NodeGraphQLService {
	newQueries := ngs.queries.WithTx(tx)
	return NodeGraphQLService{
		reader:  NewNodeGraphQLReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeGraphQLServiceTX(ctx context.Context, tx *sql.Tx) (*NodeGraphQLService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeGraphQLService{
		reader:  NewNodeGraphQLReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (ngs NodeGraphQLService) GetNodeGraphQL(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeGraphQL, error) {
	return ngs.reader.GetNodeGraphQL(ctx, id)
}

func (ngs NodeGraphQLService) CreateNodeGraphQL(ctx context.Context, ng mflow.NodeGraphQL) error {
	return NewNodeGraphQLWriterFromQueries(ngs.queries).CreateNodeGraphQL(ctx, ng)
}

func (ngs NodeGraphQLService) UpdateNodeGraphQL(ctx context.Context, ng mflow.NodeGraphQL) error {
	return NewNodeGraphQLWriterFromQueries(ngs.queries).UpdateNodeGraphQL(ctx, ng)
}

func (ngs NodeGraphQLService) DeleteNodeGraphQL(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeGraphQLWriterFromQueries(ngs.queries).DeleteNodeGraphQL(ctx, id)
}

func (ngs NodeGraphQLService) Reader() *NodeGraphQLReader { return ngs.reader }
