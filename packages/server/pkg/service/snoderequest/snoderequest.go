//nolint:revive // exported
package snoderequest

import (
	"context"
	"database/sql"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

type NodeRequestService struct {
	reader  *Reader
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeRequestService {
	return NodeRequestService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (nrs NodeRequestService) TX(tx *sql.Tx) NodeRequestService {
	newQueries := nrs.queries.WithTx(tx)
	return NodeRequestService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeRequestService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeRequestService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nrs NodeRequestService) GetNodeRequest(ctx context.Context, id idwrap.IDWrap) (*mnrequest.MNRequest, error) {
	return nrs.reader.GetNodeRequest(ctx, id)
}

func (nrs NodeRequestService) CreateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
	return NewWriterFromQueries(nrs.queries).CreateNodeRequest(ctx, nr)
}

func (nrs NodeRequestService) CreateNodeRequestBulk(ctx context.Context, nodes []mnrequest.MNRequest) error {
	return NewWriterFromQueries(nrs.queries).CreateNodeRequestBulk(ctx, nodes)
}

func (nrs NodeRequestService) UpdateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
	return NewWriterFromQueries(nrs.queries).UpdateNodeRequest(ctx, nr)
}

func (nrs NodeRequestService) DeleteNodeRequest(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(nrs.queries).DeleteNodeRequest(ctx, id)
}