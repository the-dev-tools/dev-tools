//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeForService struct {
	reader  *NodeForReader
	queries *gen.Queries
}

func NewNodeForService(queries *gen.Queries) NodeForService {
	return NodeForService{
		reader:  NewNodeForReaderFromQueries(queries),
		queries: queries,
	}
}

func (nfs NodeForService) TX(tx *sql.Tx) NodeForService {
	newQueries := nfs.queries.WithTx(tx)
	return NodeForService{
		reader:  NewNodeForReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeForServiceTX(ctx context.Context, tx *sql.Tx) (*NodeForService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeForService{
		reader:  NewNodeForReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nfs NodeForService) GetNodeFor(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeFor, error) {
	return nfs.reader.GetNodeFor(ctx, id)
}

func (nfs NodeForService) CreateNodeFor(ctx context.Context, nf mflow.NodeFor) error {
	return NewNodeForWriterFromQueries(nfs.queries).CreateNodeFor(ctx, nf)
}

func (nfs NodeForService) CreateNodeForBulk(ctx context.Context, nf []mflow.NodeFor) error {
	return NewNodeForWriterFromQueries(nfs.queries).CreateNodeForBulk(ctx, nf)
}

func (nfs NodeForService) UpdateNodeFor(ctx context.Context, nf mflow.NodeFor) error {
	return NewNodeForWriterFromQueries(nfs.queries).UpdateNodeFor(ctx, nf)
}

func (nfs NodeForService) DeleteNodeFor(ctx context.Context, id idwrap.IDWrap) error {
	err := NewNodeForWriterFromQueries(nfs.queries).DeleteNodeFor(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeForFound
	}
	return err
}

func (s NodeForService) Reader() *NodeForReader { return s.reader }
