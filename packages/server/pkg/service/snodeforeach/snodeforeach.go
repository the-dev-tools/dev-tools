//nolint:revive // exported
package snodeforeach

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnforeach"
)

var ErrNoNodeForEachFound = errors.New("node foreach not found")

type NodeForEachService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeForEachService {
	return NodeForEachService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (nfs NodeForEachService) TX(tx *sql.Tx) NodeForEachService {
	newQueries := nfs.queries.WithTx(tx)
	return NodeForEachService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeForEachService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeForEachService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nfs NodeForEachService) GetNodeForEach(ctx context.Context, id idwrap.IDWrap) (*mnforeach.MNForEach, error) {
	return nfs.reader.GetNodeForEach(ctx, id)
}

func (nfs NodeForEachService) CreateNodeForEach(ctx context.Context, nf mnforeach.MNForEach) error {
	return NewWriterFromQueries(nfs.queries).CreateNodeForEach(ctx, nf)
}

func (nfs NodeForEachService) CreateNodeForEachBulk(ctx context.Context, forEachNodes []mnforeach.MNForEach) error {
	return NewWriterFromQueries(nfs.queries).CreateNodeForEachBulk(ctx, forEachNodes)
}

func (nfs NodeForEachService) UpdateNodeForEach(ctx context.Context, nf mnforeach.MNForEach) error {
	return NewWriterFromQueries(nfs.queries).UpdateNodeForEach(ctx, nf)
}

func (nfs NodeForEachService) DeleteNodeForEach(ctx context.Context, id idwrap.IDWrap) error {
	err := NewWriterFromQueries(nfs.queries).DeleteNodeForEach(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeForEachFound
	}
	return err
}