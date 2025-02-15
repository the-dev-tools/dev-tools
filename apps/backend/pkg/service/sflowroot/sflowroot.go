package sflowroot

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/mflow"
	"the-dev-tools/backend/pkg/model/mflowroot"
	"the-dev-tools/backend/pkg/service/sflow"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type FlowRootService struct {
	queries *gen.Queries
}

var ErrNoFlowRootFound = sql.ErrNoRows

func New(queries *gen.Queries) FlowRootService {
	return FlowRootService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*FlowRootService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &FlowRootService{
		queries: queries,
	}, nil
}

func ConvertModelToDB(item mflowroot.FlowRoot) gen.FlowRoot {
	return gen.FlowRoot{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		Name:            item.Name,
		LatestVersionID: item.LatestVersionID,
	}
}

func ConvertDBToModel(item gen.FlowRoot) mflowroot.FlowRoot {
	return mflowroot.FlowRoot{
		ID:              item.ID,
		WorkspaceID:     item.WorkspaceID,
		Name:            item.Name,
		LatestVersionID: item.LatestVersionID,
	}
}

func (s *FlowRootService) GetFlowRoot(ctx context.Context, id idwrap.IDWrap) (mflowroot.FlowRoot, error) {
	item, err := s.queries.GetFlowRoot(ctx, id)
	if err != nil {
		return mflowroot.FlowRoot{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowRootFound, err)
	}
	return ConvertDBToModel(item), nil
}

func (s *FlowRootService) GetLatestFlow(ctx context.Context, id idwrap.IDWrap, flowService sflow.FlowService) (mflow.Flow, error) {
	item, err := s.queries.GetFlowRoot(ctx, id)
	if err != nil {
		return mflow.Flow{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowRootFound, err)
	}

	if item.LatestVersionID == nil {
		flowData := mflow.Flow{
			ID:         idwrap.NewNow(),
			FlowRootID: id,
			Name:       "Initial Flow",
		}
		err = flowService.CreateFlow(ctx, flowData)
		if err != nil {
			return mflow.Flow{}, err
		}
		return flowData, nil
	}

	flow, err := flowService.GetFlow(ctx, *item.LatestVersionID)
	if err != nil {
		return mflow.Flow{}, err
	}

	return flow, nil
}

func (s *FlowRootService) GetFlowRootsByWorkspace(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflowroot.FlowRoot, error) {
	items, err := s.queries.GetFlowRootsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	var results []mflowroot.FlowRoot
	for _, item := range items {
		results = append(results, ConvertDBToModel(item))
	}
	return results, nil
}

func (s *FlowRootService) CreateFlowRoot(ctx context.Context, item mflowroot.FlowRoot) error {
	arg := ConvertModelToDB(item)
	err := s.queries.CreateFlowRoot(ctx, gen.CreateFlowRootParams{
		ID:              arg.ID,
		WorkspaceID:     arg.WorkspaceID,
		Name:            arg.Name,
		LatestVersionID: arg.LatestVersionID,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowRootFound, err)
}

func (s *FlowRootService) UpdateFlowRoot(ctx context.Context, flow mflowroot.FlowRoot) error {
	arg := ConvertModelToDB(flow)
	err := s.queries.UpdateFlowRoot(ctx, gen.UpdateFlowRootParams{
		ID:              arg.ID,
		Name:            arg.Name,
		LatestVersionID: arg.LatestVersionID,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowRootFound, err)
}

func (s *FlowRootService) DeleteFlowRoot(ctx context.Context, id idwrap.IDWrap) error {
	err := s.queries.DeleteFlow(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowRootFound, err)
}
