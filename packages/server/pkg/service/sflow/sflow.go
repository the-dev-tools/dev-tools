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
	err := s.queries.CreateFlow(ctx, gen.CreateFlowParams(arg))
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

func (s *FlowService) CreateFlowBulk(ctx context.Context, flows []mflow.Flow) error {
	batchSize := 10
	for i := 0; i < len(flows); i += batchSize {
		end := i + batchSize
		if end > len(flows) {
			end = len(flows)
		}

		batch := flows[i:end]
		if len(batch) == batchSize {
			// Use bulk insert for full batches
			arg := gen.CreateFlowsBulkParams{
				ID:                 batch[0].ID,
				WorkspaceID:        batch[0].WorkspaceID,
				VersionParentID:    batch[0].VersionParentID,
				Name:               batch[0].Name,
				Duration:           batch[0].Duration,
				Running:            batch[0].Running,
				ID_2:               batch[1].ID,
				WorkspaceID_2:      batch[1].WorkspaceID,
				VersionParentID_2:  batch[1].VersionParentID,
				Name_2:             batch[1].Name,
				Duration_2:         batch[1].Duration,
				Running_2:          batch[1].Running,
				ID_3:               batch[2].ID,
				WorkspaceID_3:      batch[2].WorkspaceID,
				VersionParentID_3:  batch[2].VersionParentID,
				Name_3:             batch[2].Name,
				Duration_3:         batch[2].Duration,
				Running_3:          batch[2].Running,
				ID_4:               batch[3].ID,
				WorkspaceID_4:      batch[3].WorkspaceID,
				VersionParentID_4:  batch[3].VersionParentID,
				Name_4:             batch[3].Name,
				Duration_4:         batch[3].Duration,
				Running_4:          batch[3].Running,
				ID_5:               batch[4].ID,
				WorkspaceID_5:      batch[4].WorkspaceID,
				VersionParentID_5:  batch[4].VersionParentID,
				Name_5:             batch[4].Name,
				Duration_5:         batch[4].Duration,
				Running_5:          batch[4].Running,
				ID_6:               batch[5].ID,
				WorkspaceID_6:      batch[5].WorkspaceID,
				VersionParentID_6:  batch[5].VersionParentID,
				Name_6:             batch[5].Name,
				Duration_6:         batch[5].Duration,
				Running_6:          batch[5].Running,
				ID_7:               batch[6].ID,
				WorkspaceID_7:      batch[6].WorkspaceID,
				VersionParentID_7:  batch[6].VersionParentID,
				Name_7:             batch[6].Name,
				Duration_7:         batch[6].Duration,
				Running_7:          batch[6].Running,
				ID_8:               batch[7].ID,
				WorkspaceID_8:      batch[7].WorkspaceID,
				VersionParentID_8:  batch[7].VersionParentID,
				Name_8:             batch[7].Name,
				Duration_8:         batch[7].Duration,
				Running_8:          batch[7].Running,
				ID_9:               batch[8].ID,
				WorkspaceID_9:      batch[8].WorkspaceID,
				VersionParentID_9:  batch[8].VersionParentID,
				Name_9:             batch[8].Name,
				Duration_9:         batch[8].Duration,
				Running_9:          batch[8].Running,
				ID_10:              batch[9].ID,
				WorkspaceID_10:     batch[9].WorkspaceID,
				VersionParentID_10: batch[9].VersionParentID,
				Name_10:            batch[9].Name,
				Duration_10:        batch[9].Duration,
				Running_10:         batch[9].Running,
			}
			err := s.queries.CreateFlowsBulk(ctx, arg)
			if err != nil {
				return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
			}
		} else {
			// Fallback to single inserts for remainder
			for _, flow := range batch {
				err := s.CreateFlow(ctx, flow)
				if err != nil {
					return err
				}
			}
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
