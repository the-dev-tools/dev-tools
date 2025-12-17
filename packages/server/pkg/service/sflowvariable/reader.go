package sflowvariable

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mflowvariable"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type Reader struct {
	queries *gen.Queries
}

func NewReader(db *sql.DB) *Reader {
	return &Reader{queries: gen.New(db)}
}

func NewReaderFromQueries(queries *gen.Queries) *Reader {
	return &Reader{queries: queries}
}

func (r *Reader) GetFlowVariable(ctx context.Context, id idwrap.IDWrap) (mflowvariable.FlowVariable, error) {
	item, err := r.queries.GetFlowVariable(ctx, id)
	if err != nil {
		return mflowvariable.FlowVariable{}, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}
	return ConvertDBToModel(item), nil
}

func (r *Reader) GetFlowVariablesByFlowID(ctx context.Context, flowID idwrap.IDWrap) ([]mflowvariable.FlowVariable, error) {
	items, err := r.queries.GetFlowVariablesByFlowID(ctx, flowID)
	if err != nil {
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}
	return tgeneric.MassConvert(items, ConvertDBToModel), nil
}

// GetFlowVariablesByFlowIDOrdered returns flow variables in the flow ordered by display_order
func (r *Reader) GetFlowVariablesByFlowIDOrdered(ctx context.Context, flowID idwrap.IDWrap) ([]mflowvariable.FlowVariable, error) {
	items, err := r.queries.GetFlowVariablesByFlowIDOrdered(ctx, flowID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mflowvariable.FlowVariable{}, nil
		}
		return nil, tgeneric.ReplaceRootWithSub(sql.ErrNoRows, ErrNoFlowVariableFound, err)
	}

	return tgeneric.MassConvert(items, ConvertDBToModel), nil
}
