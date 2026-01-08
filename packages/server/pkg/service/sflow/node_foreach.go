//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

var ErrNoNodeForEachFound = errors.New("node foreach not found")

type NodeForEachService struct {
	reader  *NodeForEachReader
	queries *gen.Queries
}

func NewNodeForEachService(queries *gen.Queries) NodeForEachService {
	return NodeForEachService{
		reader:  NewNodeForEachReaderFromQueries(queries),
		queries: queries,
	}
}

func (nfs NodeForEachService) TX(tx *sql.Tx) NodeForEachService {
	newQueries := nfs.queries.WithTx(tx)
	return NodeForEachService{
		reader:  NewNodeForEachReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeForEachServiceTX(ctx context.Context, tx *sql.Tx) (*NodeForEachService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeForEachService{
		reader:  NewNodeForEachReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nfs NodeForEachService) GetNodeForEach(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeForEach, error) {
	return nfs.reader.GetNodeForEach(ctx, id)
}

func (nfs NodeForEachService) CreateNodeForEach(ctx context.Context, nf mflow.NodeForEach) error {
	return NewNodeForEachWriterFromQueries(nfs.queries).CreateNodeForEach(ctx, nf)
}

func (nfs NodeForEachService) CreateNodeForEachBulk(ctx context.Context, forEachNodes []mflow.NodeForEach) error {
	return NewNodeForEachWriterFromQueries(nfs.queries).CreateNodeForEachBulk(ctx, forEachNodes)
}

func (nfs NodeForEachService) UpdateNodeForEach(ctx context.Context, nf mflow.NodeForEach) error {
	return NewNodeForEachWriterFromQueries(nfs.queries).UpdateNodeForEach(ctx, nf)
}

func (nfs NodeForEachService) DeleteNodeForEach(ctx context.Context, id idwrap.IDWrap) error {
	err := NewNodeForEachWriterFromQueries(nfs.queries).DeleteNodeForEach(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeForEachFound
	}
	return err
}

func (s NodeForEachService) Reader() *NodeForEachReader { return s.reader }
