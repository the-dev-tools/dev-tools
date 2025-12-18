package sflow

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type NodeReader struct {
	queries *gen.Queries
}

func NewNodeReader(db *sql.DB) *NodeReader {
	return &NodeReader{queries: gen.New(db)}
}

func NewNodeReaderFromQueries(queries *gen.Queries) *NodeReader {
	return &NodeReader{queries: queries}
}

func (r *NodeReader) GetNode(ctx context.Context, id idwrap.IDWrap) (*mflow.Node, error) {
	node, err := r.queries.GetFlowNode(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertNodeToModel(node), nil
}

func (r *NodeReader) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.Node, error) {
	nodes, err := r.queries.GetFlowNodesByFlowID(ctx, flowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mflow.Node{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvertPtr(nodes, ConvertNodeToModel), nil
}
