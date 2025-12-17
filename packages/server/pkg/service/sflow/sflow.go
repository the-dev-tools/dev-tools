//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
)

type FlowService struct {
	reader  *Reader
	queries *gen.Queries
}

var ErrNoFlowFound = sql.ErrNoRows

func New(queries *gen.Queries) FlowService {
	return FlowService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}
}

func (s FlowService) TX(tx *sql.Tx) FlowService {
	newQueries := s.queries.WithTx(tx)
	return FlowService{
		reader:  NewReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*FlowService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &FlowService{
		reader:  NewReaderFromQueries(queries),
		queries: queries,
	}, nil
}

func (s *FlowService) GetFlow(ctx context.Context, id idwrap.IDWrap) (mflow.Flow, error) {
	return s.reader.GetFlow(ctx, id)
}

func (s *FlowService) GetFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	return s.reader.GetFlowsByWorkspaceID(ctx, workspaceID)
}

// GetAllFlowsByWorkspaceID returns all flows including versions for TanStack DB sync
func (s *FlowService) GetAllFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	return s.reader.GetAllFlowsByWorkspaceID(ctx, workspaceID)
}

func (s *FlowService) GetFlowsByVersionParentID(ctx context.Context, versionParentID idwrap.IDWrap) ([]mflow.Flow, error) {
	return s.reader.GetFlowsByVersionParentID(ctx, versionParentID)
}

func (s *FlowService) CreateFlow(ctx context.Context, item mflow.Flow) error {
	return NewWriterFromQueries(s.queries).CreateFlow(ctx, item)
}

func (s *FlowService) CreateFlowBulk(ctx context.Context, flows []mflow.Flow) error {
	return NewWriterFromQueries(s.queries).CreateFlowBulk(ctx, flows)
}

func (s *FlowService) UpdateFlow(ctx context.Context, flow mflow.Flow) error {
	return NewWriterFromQueries(s.queries).UpdateFlow(ctx, flow)
}

func (s *FlowService) DeleteFlow(ctx context.Context, id idwrap.IDWrap) error {
	return NewWriterFromQueries(s.queries).DeleteFlow(ctx, id)
}

// CreateFlowVersion creates a new flow version (a flow with VersionParentID set)
// This is used to snapshot a flow when it's run
func (s *FlowService) CreateFlowVersion(ctx context.Context, parentFlow mflow.Flow) (mflow.Flow, error) {
	return NewWriterFromQueries(s.queries).CreateFlowVersion(ctx, parentFlow)
}