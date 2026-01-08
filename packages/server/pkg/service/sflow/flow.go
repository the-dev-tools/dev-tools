//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
)

type FlowService struct {
	reader  *FlowReader
	queries *gen.Queries
}

var ErrNoFlowFound = sql.ErrNoRows

func NewFlowService(queries *gen.Queries) FlowService {
	return FlowService{
		reader:  NewFlowReaderFromQueries(queries),
		queries: queries,
	}
}

func (s FlowService) TX(tx *sql.Tx) FlowService {
	newQueries := s.queries.WithTx(tx)
	return FlowService{
		reader:  NewFlowReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewFlowServiceTX(ctx context.Context, tx *sql.Tx) (*FlowService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &FlowService{
		reader:  NewFlowReaderFromQueries(queries),
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
	return NewFlowWriterFromQueries(s.queries).CreateFlow(ctx, item)
}

func (s *FlowService) CreateFlowBulk(ctx context.Context, flows []mflow.Flow) error {
	return NewFlowWriterFromQueries(s.queries).CreateFlowBulk(ctx, flows)
}

func (s *FlowService) UpdateFlow(ctx context.Context, flow mflow.Flow) error {
	return NewFlowWriterFromQueries(s.queries).UpdateFlow(ctx, flow)
}

func (s *FlowService) DeleteFlow(ctx context.Context, id idwrap.IDWrap) error {
	return NewFlowWriterFromQueries(s.queries).DeleteFlow(ctx, id)
}

// CreateFlowVersion creates a new flow version (a flow with VersionParentID set)
// This is used to snapshot a flow when it's run
func (s *FlowService) CreateFlowVersion(ctx context.Context, parentFlow mflow.Flow) (mflow.Flow, error) {
	return NewFlowWriterFromQueries(s.queries).CreateFlowVersion(ctx, parentFlow)
}

// GetLatestVersionByParentID returns the most recent version of a flow
func (s *FlowService) GetLatestVersionByParentID(ctx context.Context, parentID idwrap.IDWrap) (*mflow.Flow, error) {
	return s.reader.GetLatestVersionByParentID(ctx, parentID)
}

// UpdateFlowNodeIDMapping updates the node ID mapping for a flow version
func (s *FlowService) UpdateFlowNodeIDMapping(ctx context.Context, flowID idwrap.IDWrap, mapping []byte) error {
	return NewFlowWriterFromQueries(s.queries).UpdateFlowNodeIDMapping(ctx, flowID, mapping)
}

func (s FlowService) Reader() *FlowReader { return s.reader }
