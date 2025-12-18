package shttp

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type SearchParamWriter struct {
	queries *gen.Queries
	reader  *SearchParamReader
}

func NewSearchParamWriter(tx gen.DBTX) *SearchParamWriter {
	queries := gen.New(tx)
	return &SearchParamWriter{
		queries: queries,
		reader:  NewSearchParamReaderFromQueries(queries),
	}
}

func NewSearchParamWriterFromQueries(queries *gen.Queries) *SearchParamWriter {
	return &SearchParamWriter{
		queries: queries,
		reader:  NewSearchParamReaderFromQueries(queries),
	}
}

func (w *SearchParamWriter) Create(ctx context.Context, param *mhttp.HTTPSearchParam) error {
	dbParam := SerializeSearchParamModelToGen(*param)
	return w.queries.CreateHTTPSearchParam(ctx, dbParam)
}

func (w *SearchParamWriter) CreateBulk(ctx context.Context, httpID idwrap.IDWrap, params []mhttp.HTTPSearchParam) error {
	if len(params) == 0 {
		return nil
	}

	for i := range params {
		params[i].HttpID = httpID
	}

	dbParams := tgeneric.MassConvert(params, SerializeSearchParamModelToGen)

	chunkSize := 100
	for i := 0; i < len(dbParams); i += chunkSize {
		end := i + chunkSize
		if end > len(dbParams) {
			end = len(dbParams)
		}

		chunk := dbParams[i:end]
		for _, param := range chunk {
			err := w.queries.CreateHTTPSearchParam(ctx, param)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *SearchParamWriter) Update(ctx context.Context, param *mhttp.HTTPSearchParam) error {
	err := w.queries.UpdateHTTPSearchParam(ctx, gen.UpdateHTTPSearchParamParams{
		Key:         param.Key,
		Value:       param.Value,
		Description: param.Description,
		Enabled:     param.Enabled,
		ID:          param.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoHttpSearchParamFound
		}
		return err
	}
	return nil
}

func (w *SearchParamWriter) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float64) error {
	return w.queries.UpdateHTTPSearchParamDelta(ctx, gen.UpdateHTTPSearchParamDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               id,
	})
}

func (w *SearchParamWriter) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float64) error {
	return w.queries.UpdateHTTPSearchParamOrder(ctx, gen.UpdateHTTPSearchParamOrderParams{
		DisplayOrder: order,
		ID:           id,
		HttpID:       httpID,
	})
}

func (w *SearchParamWriter) Delete(ctx context.Context, paramID idwrap.IDWrap) error {
	err := w.queries.DeleteHTTPSearchParam(ctx, paramID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoHttpSearchParamFound
		}
		return err
	}
	return nil
}

func (w *SearchParamWriter) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	params, err := w.reader.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, param := range params {
		if err := w.Delete(ctx, param.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w *SearchParamWriter) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	param, err := w.reader.GetByID(ctx, id)
	if err != nil {
		return err
	}

	err = w.Update(ctx, &mhttp.HTTPSearchParam{
		ID:                      param.ID,
		HttpID:                  param.HttpID,
		Key:                     param.Key,
		Value:                   param.Value,
		Enabled:                 param.Enabled,
		Description:             param.Description,
		DisplayOrder:            param.DisplayOrder,
		ParentHttpSearchParamID: nil,
		IsDelta:                 false,
		DeltaKey:                nil,
		DeltaValue:              nil,
		DeltaEnabled:            nil,
		DeltaDescription:        nil,
		DeltaDisplayOrder:       nil,
		CreatedAt:               param.CreatedAt,
		UpdatedAt:               param.UpdatedAt,
	})
	if err != nil {
		return err
	}

	return w.UpdateDelta(ctx, id, nil, nil, nil, nil, nil)
}
