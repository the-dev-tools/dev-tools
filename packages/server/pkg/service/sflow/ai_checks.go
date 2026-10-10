package sflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

// AIChecksService keeps flows' AI check settings and nodes' expect: blocks, stored as JSON.
type AIChecksService struct {
	queries *gen.Queries
}

// NewAIChecksService returns the service over queries.
func NewAIChecksService(queries *gen.Queries) AIChecksService {
	return AIChecksService{queries: queries}
}

// TX binds the service to a transaction.
func (s AIChecksService) TX(tx *sql.Tx) AIChecksService {
	return AIChecksService{queries: s.queries.WithTx(tx)}
}

// GetNodeExpect returns a node's expect: block, nil when it has none.
func (s AIChecksService) GetNodeExpect(ctx context.Context, nodeID idwrap.IDWrap) (*mflow.NodeExpect, error) {
	r, err := s.queries.GetFlowNodeExpect(ctx, nodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := mflow.NodeExpect{FlowNodeID: r.FlowNodeID}
	if err := json.Unmarshal([]byte(r.Expect), &out.Expect); err != nil {
		return nil, fmt.Errorf("decode expect of node %s: %w", nodeID, err)
	}
	return &out, nil
}

// GetNodeExpectsByFlowID returns the expect: blocks of a flow's nodes.
func (s AIChecksService) GetNodeExpectsByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.NodeExpect, error) {
	rows, err := s.queries.GetFlowNodeExpectsByFlowID(ctx, flowID)
	if err != nil {
		return nil, err
	}
	out := make([]mflow.NodeExpect, 0, len(rows))
	for _, r := range rows {
		e := mflow.NodeExpect{FlowNodeID: r.FlowNodeID}
		if err := json.Unmarshal([]byte(r.Expect), &e.Expect); err != nil {
			return nil, fmt.Errorf("decode expect of node %s: %w", r.FlowNodeID, err)
		}
		out = append(out, e)
	}
	return out, nil
}

// UpsertNodeExpect stores a node's expect: block.
func (s AIChecksService) UpsertNodeExpect(ctx context.Context, e mflow.NodeExpect) error {
	b, err := json.Marshal(e.Expect)
	if err != nil {
		return err
	}
	return s.queries.UpsertFlowNodeExpect(ctx, gen.UpsertFlowNodeExpectParams{FlowNodeID: e.FlowNodeID, Expect: string(b)})
}

// DeleteNodeExpect removes a node's expect: block.
func (s AIChecksService) DeleteNodeExpect(ctx context.Context, nodeID idwrap.IDWrap) error {
	return s.queries.DeleteFlowNodeExpect(ctx, nodeID)
}

// GetFlowAIChecks returns a flow's AI check settings, nil when it has none.
func (s AIChecksService) GetFlowAIChecks(ctx context.Context, flowID idwrap.IDWrap) (*mflow.FlowAIChecks, error) {
	r, err := s.queries.GetFlowAIChecks(ctx, flowID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := mflow.FlowAIChecks{FlowID: r.FlowID}
	if err := json.Unmarshal([]byte(r.Settings), &out.Settings); err != nil {
		return nil, fmt.Errorf("decode AI check settings of flow %s: %w", flowID, err)
	}
	return &out, nil
}

// UpsertFlowAIChecks stores a flow's AI check settings.
func (s AIChecksService) UpsertFlowAIChecks(ctx context.Context, c mflow.FlowAIChecks) error {
	b, err := json.Marshal(c.Settings)
	if err != nil {
		return err
	}
	return s.queries.UpsertFlowAIChecks(ctx, gen.UpsertFlowAIChecksParams{FlowID: c.FlowID, Settings: string(b)})
}

// DeleteFlowAIChecks removes a flow's AI check settings.
func (s AIChecksService) DeleteFlowAIChecks(ctx context.Context, flowID idwrap.IDWrap) error {
	return s.queries.DeleteFlowAIChecks(ctx, flowID)
}
