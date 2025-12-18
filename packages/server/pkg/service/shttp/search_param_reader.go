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

type SearchParamReader struct {
	queries *gen.Queries
}

func NewSearchParamReader(db *sql.DB) *SearchParamReader {
	return &SearchParamReader{queries: gen.New(db)}
}

func NewSearchParamReaderFromQueries(queries *gen.Queries) *SearchParamReader {
	return &SearchParamReader{queries: queries}
}

func (r *SearchParamReader) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	dbParams, err := r.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPSearchParam{}, nil
		}
		return nil, err
	}

	params := tgeneric.MassConvert(dbParams, DeserializeSearchParamGenToModel)
	return params, nil
}

func (r *SearchParamReader) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	dbParams, err := r.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPSearchParam{}, nil
		}
		return nil, err
	}

	// Sort by order field
	slices.SortFunc(dbParams, func(a, b gen.GetHTTPSearchParamsRow) int {
		if a.DisplayOrder < b.DisplayOrder {
			return -1
		}
		if a.DisplayOrder > b.DisplayOrder {
			return 1
		}
		return 0
	})

	params := tgeneric.MassConvert(dbParams, DeserializeSearchParamGenToModel)
	return params, nil
}

func (r *SearchParamReader) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPSearchParam{}, nil
	}

	rows, err := r.queries.GetHTTPSearchParamsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	params := tgeneric.MassConvert(rows, deserializeSearchParamByIDsRowToModel)
	return params, nil
}

func (r *SearchParamReader) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPSearchParam, error) {
	rows, err := r.queries.GetHTTPSearchParamsByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoHttpSearchParamFound
	}

	model := deserializeSearchParamByIDsRowToModel(rows[0])
	return &model, nil
}

func (r *SearchParamReader) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPSearchParam, error) {
	result := make(map[idwrap.IDWrap][]mhttp.HTTPSearchParam, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	for _, httpID := range httpIDs {
		rows, err := r.queries.GetHTTPSearchParams(ctx, httpID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				result[httpID] = []mhttp.HTTPSearchParam{}
				continue
			}
			return nil, err
		}

		var params []mhttp.HTTPSearchParam
		for _, row := range rows {
			model := DeserializeSearchParamGenToModel(row)
			params = append(params, model)
		}
		result[httpID] = params
	}

	return result, nil
}

func (r *SearchParamReader) GetStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]gen.GetHTTPSearchParamsStreamingRow, error) {
	return r.queries.GetHTTPSearchParamsStreaming(ctx, gen.GetHTTPSearchParamsStreamingParams{
		HttpIds:   httpIDs,
		UpdatedAt: updatedAt,
	})
}
