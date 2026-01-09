package sflow

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mflow"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type FlowReader struct {
	queries *gen.Queries
}

func NewFlowReader(db *sql.DB) *FlowReader {
	return &FlowReader{queries: gen.New(db)}
}

func NewFlowReaderFromQueries(queries *gen.Queries) *FlowReader {
	return &FlowReader{queries: queries}
}

func (r *FlowReader) GetFlow(ctx context.Context, id idwrap.IDWrap) (mflow.Flow, error) {
	item, err := r.queries.GetFlow(ctx, id)
	if err != nil {
		return mflow.Flow{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return ConvertDBToFlow(item), nil
}

func (r *FlowReader) GetFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToFlow), nil
}

// GetAllFlowsByWorkspaceID returns all flows including versions for TanStack DB sync
func (r *FlowReader) GetAllFlowsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetAllFlowsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToFlow), nil
}

func (r *FlowReader) GetFlowsByVersionParentID(ctx context.Context, versionParentID idwrap.IDWrap) ([]mflow.Flow, error) {
	item, err := r.queries.GetFlowsByVersionParentID(ctx, &versionParentID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowFound, err)
	}
	return tgeneric.MassConvert(item, ConvertDBToFlow), nil
}

// GetLatestVersionByParentID returns the most recent version of a flow
func (r *FlowReader) GetLatestVersionByParentID(ctx context.Context, parentID idwrap.IDWrap) (*mflow.Flow, error) {
	item, err := r.queries.GetLatestVersionByParentID(ctx, &parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No version exists yet
		}
		return nil, err
	}
	flow := ConvertDBToFlow(item)
	return &flow, nil
}
