package snodenoop

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mnnode/mnnoop"
)

var ErrNoNodeForFound = sql.ErrNoRows

type NodeNoopService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) NodeNoopService {
	return NodeNoopService{queries: queries}
}

func (nns NodeNoopService) TX(tx *sql.Tx) NodeNoopService {
	return NodeNoopService{queries: nns.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*NodeNoopService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &NodeNoopService{
		queries: queries,
	}, nil
}

func ConvertToDBNodeStart(ns mnnoop.NoopNode) gen.FlowNodeNoop {
	return gen.FlowNodeNoop{
		FlowNodeID: ns.FlowNodeID,
		NodeType:   int16(ns.Type),
	}
}

func ConvertToModelNodeStart(ns gen.FlowNodeNoop) *mnnoop.NoopNode {
	return &mnnoop.NoopNode{
		FlowNodeID: ns.FlowNodeID,
		Type:       mnnoop.NoopTypes(ns.NodeType),
	}
}

func (nfs NodeNoopService) GetNodeNoop(ctx context.Context, id idwrap.IDWrap) (*mnnoop.NoopNode, error) {
	nodeFor, err := nfs.queries.GetFlowNodeNoop(ctx, id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelNodeStart(nodeFor), nil
}

func (nfs NodeNoopService) GetNodesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mnnoop.NoopNode, error) {
	// Since there's no dedicated query for getting all NoOp nodes by flow ID,
	// we'll need to implement this at a higher level or add a SQL query.
	// For now, return empty slice to make it work with the collection API.
	return []mnnoop.NoopNode{}, nil
}

func (nfs NodeNoopService) CreateNodeNoop(ctx context.Context, nf mnnoop.NoopNode) error {
	convertedNode := ConvertToDBNodeStart(nf)
	return nfs.queries.CreateFlowNodeNoop(ctx, gen.CreateFlowNodeNoopParams{
		FlowNodeID: convertedNode.FlowNodeID,
		NodeType:   convertedNode.NodeType,
	})
}

func (nfs NodeNoopService) CreateNodeNoopBulk(ctx context.Context, nf []mnnoop.NoopNode) error {
	for _, n := range nf {
		convertedNode := ConvertToDBNodeStart(n)
		err := nfs.queries.CreateFlowNodeNoop(ctx, gen.CreateFlowNodeNoopParams{
			FlowNodeID: convertedNode.FlowNodeID,
			NodeType:   convertedNode.NodeType,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (nfs NodeNoopService) UpdateNodeNoop(ctx context.Context, nf mnnoop.NoopNode) error {
	// Since there's no UpdateFlowNodeNoop query, we'll use delete + create pattern
	if err := nfs.DeleteNodeNoop(ctx, nf.FlowNodeID); err != nil && !errors.Is(err, ErrNoNodeForFound) {
		return err
	}
	return nfs.CreateNodeNoop(ctx, nf)
}

func (nfs NodeNoopService) DeleteNodeNoop(ctx context.Context, id idwrap.IDWrap) error {
	err := nfs.queries.DeleteFlowNodeNoop(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNoNodeForFound
	}
	return err
}
