package sflow

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflow"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type FlowVariableReader struct {
	queries *gen.Queries
}

func NewFlowVariableReader(db *sql.DB) *FlowVariableReader {
	return &FlowVariableReader{queries: gen.New(db)}
}

func NewFlowVariableReaderFromQueries(queries *gen.Queries) *FlowVariableReader {
	return &FlowVariableReader{queries: queries}
}

func (r *FlowVariableReader) GetFlowVariable(ctx context.Context, id idwrap.IDWrap) (mflow.FlowVariable, error) {
	item, err := r.queries.GetFlowVariable(ctx, id)
	if err != nil {
		return mflow.FlowVariable{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}
	return ConvertDBToFlowVariable(item), nil
}

func (r *FlowVariableReader) GetFlowVariablesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.FlowVariable, error) {
	items, err := r.queries.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}
	return tgeneric.MassConvert(items, ConvertDBToFlowVariable), nil
}

// GetFlowVariablesByFlowIDOrdered returns flow variables in the flow ordered by display_order
func (r *FlowVariableReader) GetFlowVariablesByFlowIDOrdered(ctx context.Context, flowID idwrap.IDWrap) ([]mflow.FlowVariable, error) {
	items, err := r.queries.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mflow.FlowVariable{}, nil
		}
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}

	return tgeneric.MassConvert(items, ConvertDBToFlowVariable), nil
}
