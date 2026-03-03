package sflow

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type FlowWriter struct {
	queries *gen.Queries
}

func NewFlowWriter(tx gen.DBTX) *FlowWriter {
	return &FlowWriter{queries: gen.New(tx)}
}

func NewFlowWriterFromQueries(queries *gen.Queries) *FlowWriter {
	return &FlowWriter{queries: queries}
}

func (w *FlowWriter) CreateFlow(ctx context.Context, item mflow.Flow) error {
	arg := ConvertFlowToDB(item)
	err := w.queries.CreateFlow(ctx, gen.CreateFlowParams(arg))
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

func (w *FlowWriter) CreateFlowBulk(ctx context.Context, flows []mflow.Flow) error {
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
				Error:              nullStringFromPtr(batch[0].Error),
				NodeIDMapping:      batch[0].NodeIDMapping,
				ID_2:               batch[1].ID,
				WorkspaceID_2:      batch[1].WorkspaceID,
				VersionParentID_2:  batch[1].VersionParentID,
				Name_2:             batch[1].Name,
				Duration_2:         batch[1].Duration,
				Running_2:          batch[1].Running,
				Error_2:            nullStringFromPtr(batch[1].Error),
				NodeIDMapping_2:    batch[1].NodeIDMapping,
				ID_3:               batch[2].ID,
				WorkspaceID_3:      batch[2].WorkspaceID,
				VersionParentID_3:  batch[2].VersionParentID,
				Name_3:             batch[2].Name,
				Duration_3:         batch[2].Duration,
				Running_3:          batch[2].Running,
				Error_3:            nullStringFromPtr(batch[2].Error),
				NodeIDMapping_3:    batch[2].NodeIDMapping,
				ID_4:               batch[3].ID,
				WorkspaceID_4:      batch[3].WorkspaceID,
				VersionParentID_4:  batch[3].VersionParentID,
				Name_4:             batch[3].Name,
				Duration_4:         batch[3].Duration,
				Running_4:          batch[3].Running,
				Error_4:            nullStringFromPtr(batch[3].Error),
				NodeIDMapping_4:    batch[3].NodeIDMapping,
				ID_5:               batch[4].ID,
				WorkspaceID_5:      batch[4].WorkspaceID,
				VersionParentID_5:  batch[4].VersionParentID,
				Name_5:             batch[4].Name,
				Duration_5:         batch[4].Duration,
				Running_5:          batch[4].Running,
				Error_5:            nullStringFromPtr(batch[4].Error),
				NodeIDMapping_5:    batch[4].NodeIDMapping,
				ID_6:               batch[5].ID,
				WorkspaceID_6:      batch[5].WorkspaceID,
				VersionParentID_6:  batch[5].VersionParentID,
				Name_6:             batch[5].Name,
				Duration_6:         batch[5].Duration,
				Running_6:          batch[5].Running,
				Error_6:            nullStringFromPtr(batch[5].Error),
				NodeIDMapping_6:    batch[5].NodeIDMapping,
				ID_7:               batch[6].ID,
				WorkspaceID_7:      batch[6].WorkspaceID,
				VersionParentID_7:  batch[6].VersionParentID,
				Name_7:             batch[6].Name,
				Duration_7:         batch[6].Duration,
				Running_7:          batch[6].Running,
				Error_7:            nullStringFromPtr(batch[6].Error),
				NodeIDMapping_7:    batch[6].NodeIDMapping,
				ID_8:               batch[7].ID,
				WorkspaceID_8:      batch[7].WorkspaceID,
				VersionParentID_8:  batch[7].VersionParentID,
				Name_8:             batch[7].Name,
				Duration_8:         batch[7].Duration,
				Running_8:          batch[7].Running,
				Error_8:            nullStringFromPtr(batch[7].Error),
				NodeIDMapping_8:    batch[7].NodeIDMapping,
				ID_9:               batch[8].ID,
				WorkspaceID_9:      batch[8].WorkspaceID,
				VersionParentID_9:  batch[8].VersionParentID,
				Name_9:             batch[8].Name,
				Duration_9:         batch[8].Duration,
				Running_9:          batch[8].Running,
				Error_9:            nullStringFromPtr(batch[8].Error),
				NodeIDMapping_9:    batch[8].NodeIDMapping,
				ID_10:              batch[9].ID,
				WorkspaceID_10:     batch[9].WorkspaceID,
				VersionParentID_10: batch[9].VersionParentID,
				Name_10:            batch[9].Name,
				Duration_10:        batch[9].Duration,
				Running_10:         batch[9].Running,
				Error_10:           nullStringFromPtr(batch[9].Error),
				NodeIDMapping_10:   batch[9].NodeIDMapping,
			}
			err := w.queries.CreateFlowsBulk(ctx, arg)
			if err != nil {
				return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
			}
		} else {
			// Fallback to single inserts for remainder
			for _, flow := range batch {
				err := w.CreateFlow(ctx, flow)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (w *FlowWriter) UpdateFlow(ctx context.Context, flow mflow.Flow) error {
	arg := ConvertFlowToDB(flow)
	err := w.queries.UpdateFlow(ctx, gen.UpdateFlowParams{
		ID:       arg.ID,
		Name:     arg.Name,
		Duration: arg.Duration,
		Running:  arg.Running,
		Error:    arg.Error,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

func (w *FlowWriter) DeleteFlow(ctx context.Context, id idwrap.IDWrap) error {
	err := w.queries.DeleteFlow(ctx, id)
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}

// CreateFlowVersion creates a new flow version (a flow with VersionParentID set)
// This is used to snapshot a flow when it's run
func (w *FlowWriter) CreateFlowVersion(ctx context.Context, parentFlow mflow.Flow) (mflow.Flow, error) {
	versionID := idwrap.NewNow()
	version := mflow.Flow{
		ID:              versionID,
		WorkspaceID:     parentFlow.WorkspaceID,
		VersionParentID: &parentFlow.ID,
		Name:            parentFlow.Name,
		Duration:        parentFlow.Duration,
		Running:         false,
	}

	err := w.queries.CreateFlow(ctx, gen.CreateFlowParams{
		ID:              version.ID,
		WorkspaceID:     version.WorkspaceID,
		VersionParentID: version.VersionParentID,
		Name:            version.Name,
		Duration:        version.Duration,
		Running:         version.Running,
		NodeIDMapping:   nil, // Will be set later via UpdateFlowNodeIDMapping
	})
	if err != nil {
		return mflow.Flow{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}

	return version, nil
}

// UpdateFlowNodeIDMapping updates the node ID mapping for a flow version
func (w *FlowWriter) UpdateFlowNodeIDMapping(ctx context.Context, flowID idwrap.IDWrap, mapping []byte) error {
	err := w.queries.UpdateFlowNodeIDMapping(ctx, gen.UpdateFlowNodeIDMappingParams{
		ID:            flowID,
		NodeIDMapping: mapping,
	})
	return tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
}
