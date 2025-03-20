package sflow

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type FlowService struct {
	queries *gen.Queries
}

var ErrNoFlowFound = sql.ErrNoRows

func New(queries *gen.Queries) FlowService {
	return FlowService{queries: queries}
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
	}
}

func ConvertDBToModel(item gen.Flow) mflow.Flow {
	return mflow.Flow{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		VersionParentID: item.VersionParentID,
		Name:            item.Name,
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
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

func (s *FlowService) UpdateFlow(ctx context.Context, flow mflow.Flow) error {
	arg := ConvertModelToDB(flow)
	err := s.queries.UpdateFlow(ctx, gen.UpdateFlowParams{
		ID:   arg.ID,
		Name: arg.Name,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

func (s *FlowService) DeleteFlow(ctx context.Context, id idwrap.IDWrap) error {
	err := s.queries.DeleteFlow(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}
