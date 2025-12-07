//nolint:revive // exported
package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type FlowService struct {
	queries *gen.Queries
}

var ErrNoFlowFound = sql.ErrNoRows

func New(queries *gen.Queries) FlowService {
	return FlowService{queries: queries}
}

func (s FlowService) TX(tx *sql.Tx) FlowService {
	return FlowService{queries: s.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*FlowService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &FlowService{
		queries: queries,
	}, nil
}

func ConvertModelToDB(item mflow.Flow) gen.Flow {
	return gen.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
	}
}

func ConvertDBToModel(item gen.Flow) mflow.Flow {
	return mflow.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
		Duration:        item.Duration,
		Running:         item.Running,
	}
}

func (s *FlowService) GetFlow(ctx context.Context, id idwrap.IDWrap) (mflow.Flow, error) {
	item, err := s.queries.GetFlow(ctx, id)
	if err != nil {
		return mflow.Flow{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return ConvertDBToModel(item), nil
}

func (s *FlowService) GetFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := s.queries.GetFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToModel), nil
}

// GetAllFlowsByWorkspaceID returns all flows including versions for TanStack DB sync
func (s *FlowService) GetAllFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := s.queries.GetAllFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToModel), nil
}

func (s *FlowService) GetFlowsByVersionParentID(ctx context.Context, versionParentID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := s.queries.GetFlowsByVersionParentID(ctx, &versionParentID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToModel), nil
}

func (s *FlowService) CreateFlow(ctx context.Context, item mflow.Flow) error {
	arg := ConvertModelToDB(item)
	err := s.queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:              arg.ID,
		WorkspaceID:     arg.WorkspaceID,
		VersionParentID: arg.VersionParentID,
		Name:            arg.Name,
		Duration:        arg.Duration,
		Running:         arg.Running,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

func (s *FlowService) CreateFlowBulk(ctx context.Context, flows []mflow.Flow) error {
	var err error
	for _, flow := range flows {
		err = s.CreateFlow(ctx, flow)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *FlowService) UpdateFlow(ctx context.Context, flow mflow.Flow) error {
	arg := ConvertModelToDB(flow)
	err := s.queries.UpdateFlow(ctx, gen.UpdateFlowParams{
		ID:       arg.ID,
		Name:     arg.Name,
		Duration: arg.Duration,
		Running:  arg.Running,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

func (s *FlowService) DeleteFlow(ctx context.Context, id idwrap.IDWrap) error {
	err := s.queries.DeleteFlow(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

// CreateFlowVersion creates a new flow version (a flow with VersionParentID set)
// This is used to snapshot a flow when it's run
func (s *FlowService) CreateFlowVersion(ctx context.Context, parentFlow mflow.Flow) (mflow.Flow, error) {
	versionID := idwrap.NewNow()
	version := mflow.Flow{
		ID:              versionID,
		WorkspaceID:     parentFlow.WorkspaceID,
		VersionParentID: &parentFlow.ID,
		Name:            parentFlow.Name,
		Duration:        parentFlow.Duration,
		Running:         false,
	}

	err := s.queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:              version.ID,
		WorkspaceID:     version.WorkspaceID,
		VersionParentID: version.VersionParentID,
		Name:            version.Name,
		Duration:        version.Duration,
		Running:         version.Running,
	})
	if err != nil {
		return mflow.Flow{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}

	return version, nil
}
