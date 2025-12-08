//nolint:revive // exported
package snoderequest

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnrequest"
)

type NodeRequestService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeRequestService {
	return NodeRequestService{queries: queries}
}

func (nrs NodeRequestService) TX(tx *sql.Tx) NodeRequestService {
	return NodeRequestService{queries: nrs.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeRequestService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeRequestService{
		queries: queries,
	}, nil
}

func ConvertToDBNodeHTTP(nr mnrequest.MNRequest) (gen.FlowNodeHttp, bool) {
	if nr.HttpID == nil || isZeroID(*nr.HttpID) {
		return gen.FlowNodeHttp{}, false
	}

	var deltaID []byte
	if nr.DeltaHttpID != nil && !isZeroID(*nr.DeltaHttpID) {
		deltaID = nr.DeltaHttpID.Bytes()
	}

	return gen.FlowNodeHttp{
		FlowNodeID:  nr.FlowNodeID,
		HttpID:      *nr.HttpID,
		DeltaHttpID: deltaID,
	}, true
}

func ConvertToModelNodeHTTP(nr gen.FlowNodeHttp) *mnrequest.MNRequest {
	var deltaID *idwrap.IDWrap
	if len(nr.DeltaHttpID) > 0 {
		id, err := idwrap.NewFromBytes(nr.DeltaHttpID)
		if err == nil {
			deltaID = &id
		}
	}
	httpID := nr.HttpID

	result := &mnrequest.MNRequest{
		FlowNodeID:  nr.FlowNodeID,
		HttpID:      &httpID,
		DeltaHttpID: deltaID,
	}
	result.HasRequestConfig = !isZeroID(nr.HttpID)
	return result
}

func isZeroID(id idwrap.IDWrap) bool {
	return id == idwrap.IDWrap{}
}

func (nrs NodeRequestService) GetNodeRequest(ctx context.Context, id idwrap.IDWrap) (*mnrequest.MNRequest, error) {
	nodeHTTP, err := nrs.queries.GetFlowNodeHTTP(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return ConvertToModelNodeHTTP(nodeHTTP), nil
}

func (nrs NodeRequestService) CreateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
	nodeHTTP, ok := ConvertToDBNodeHTTP(nr)
	if !ok {
		return nil
	}
	return nrs.queries.CreateFlowNodeHTTP(ctx, gen.CreateFlowNodeHTTPParams(nodeHTTP))
}

func (nrs NodeRequestService) CreateNodeRequestBulk(ctx context.Context, nodes []mnrequest.MNRequest) error {
	for _, node := range nodes {
		nodeHTTP, ok := ConvertToDBNodeHTTP(node)
		if !ok {
			continue
		}

		if err := nrs.queries.CreateFlowNodeHTTP(ctx, gen.CreateFlowNodeHTTPParams(nodeHTTP)); err != nil {
			return err
		}
	}
	return nil
}

func (nrs NodeRequestService) UpdateNodeRequest(ctx context.Context, nr mnrequest.MNRequest) error {
	nodeHTTP, ok := ConvertToDBNodeHTTP(nr)
	if !ok {
		// Treat removal of HttpID as request to delete any existing binding.
		if err := nrs.queries.DeleteFlowNodeHTTP(ctx, nr.FlowNodeID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		return nil
	}
	return nrs.queries.UpdateFlowNodeHTTP(ctx, gen.UpdateFlowNodeHTTPParams(nodeHTTP))
}

func (nrs NodeRequestService) DeleteNodeRequest(ctx context.Context, id idwrap.IDWrap) error {
	return nrs.queries.DeleteFlowNodeHTTP(ctx, id)
}
