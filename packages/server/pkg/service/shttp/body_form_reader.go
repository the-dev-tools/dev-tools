package shttp

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type BodyFormReader struct {
	queries *gen.Queries
}

func NewBodyFormReader(db *sql.DB) *BodyFormReader {
	return &BodyFormReader{queries: gen.New(db)}
}

func NewBodyFormReaderFromQueries(queries *gen.Queries) *BodyFormReader {
	return &BodyFormReader{queries: queries}
}

func (r *BodyFormReader) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyForm, error) {
	rows, err := r.queries.GetHTTPBodyFormsByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoHttpBodyFormFound
	}

	model := deserializeBodyFormByIDsRowToModel(rows[0])
	return &model, nil
}

func (r *BodyFormReader) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	rows, err := r.queries.GetHTTPBodyForms(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyForm{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPBodyForm, len(rows))
	for i, row := range rows {
		result[i] = DeserializeBodyFormGenToModel(row)
	}
	return result, nil
}

func (r *BodyFormReader) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	rows, err := r.queries.GetHTTPBodyForms(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyForm{}, nil
		}
		return nil, err
	}

	// Sort by order field
	slices.SortFunc(rows, func(a, b gen.GetHTTPBodyFormsRow) int {
		if a.DisplayOrder < b.DisplayOrder {
			return -1
		}
		if a.DisplayOrder > b.DisplayOrder {
			return 1
		}
		return 0
	})

	result := make([]mhttp.HTTPBodyForm, len(rows))
	for i, row := range rows {
		result[i] = DeserializeBodyFormGenToModel(row)
	}
	return result, nil
}

func (r *BodyFormReader) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPBodyForm{}, nil
	}

	rows, err := r.queries.GetHTTPBodyFormsByIDs(ctx, ids)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyForm{}, nil
		}
		return nil, err
	}

	return tgeneric.MassConvert(rows, deserializeBodyFormByIDsRowToModel), nil
}

func (r *BodyFormReader) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPBodyForm, error) {
	result := make(map[idwrap.IDWrap][]mhttp.HTTPBodyForm, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	rows, err := r.queries.GetHTTPBodyFormsByIDs(ctx, httpIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}

	for _, row := range rows {
		model := deserializeBodyFormByIDsRowToModel(row)
		httpID := model.HttpID
		result[httpID] = append(result[httpID], model)
	}

	return result, nil
}

func (r *BodyFormReader) GetStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]gen.GetHTTPBodyFormStreamingRow, error) {
	return r.queries.GetHTTPBodyFormStreaming(ctx, gen.GetHTTPBodyFormStreamingParams{
		HttpIds:   httpIDs,
		UpdatedAt: updatedAt,
	})
}
