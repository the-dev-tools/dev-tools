//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type NodeRequestService struct {
	reader  *NodeRequestReader
	queries *gen.Queries
}

func NewNodeRequestService(queries *gen.Queries) NodeRequestService {
	return NodeRequestService{
		reader:  NewNodeRequestReaderFromQueries(queries),
		queries: queries,
	}
}

func (nrs NodeRequestService) TX(tx *sql.Tx) NodeRequestService {
	newQueries := nrs.queries.WithTx(tx)
	return NodeRequestService{
		reader:  NewNodeRequestReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewNodeRequestServiceTX(ctx context.Context, tx *sql.Tx) (*NodeRequestService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeRequestService{
		reader:  NewNodeRequestReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (nrs NodeRequestService) GetNodeRequest(ctx context.Context, id idwrap.IDWrap) (*mflow.NodeRequest, error) {
	return nrs.reader.GetNodeRequest(ctx, id)
}

func (nrs NodeRequestService) CreateNodeRequest(ctx context.Context, nr mflow.NodeRequest) error {
	return NewNodeRequestWriterFromQueries(nrs.queries).CreateNodeRequest(ctx, nr)
}

func (nrs NodeRequestService) CreateNodeRequestBulk(ctx context.Context, nodes []mflow.NodeRequest) error {
	return NewNodeRequestWriterFromQueries(nrs.queries).CreateNodeRequestBulk(ctx, nodes)
}

func (nrs NodeRequestService) UpdateNodeRequest(ctx context.Context, nr mflow.NodeRequest) error {
	return NewNodeRequestWriterFromQueries(nrs.queries).UpdateNodeRequest(ctx, nr)
}

func (nrs NodeRequestService) DeleteNodeRequest(ctx context.Context, id idwrap.IDWrap) error {
	return NewNodeRequestWriterFromQueries(nrs.queries).DeleteNodeRequest(ctx, id)
}

func (s NodeRequestService) Reader() *NodeRequestReader { return s.reader }
