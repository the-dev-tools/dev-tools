package shttp

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

type BodyUrlEncodedReader struct {
	queries *gen.Queries
}

func NewBodyUrlEncodedReader(db *sql.DB) *BodyUrlEncodedReader {
	return &BodyUrlEncodedReader{queries: gen.New(db)}
}

func NewBodyUrlEncodedReaderFromQueries(queries *gen.Queries) *BodyUrlEncodedReader {
	return &BodyUrlEncodedReader{queries: queries}
}

func (r *BodyUrlEncodedReader) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyUrlencoded, error) {
	body, err := r.queries.GetHTTPBodyUrlEncoded(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHttpBodyUrlEncodedFound
		}
		return nil, err
	}

	model := DeserializeBodyUrlEncodedGenToModel(body)
	return &model, nil
}

func (r *BodyUrlEncodedReader) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	bodies, err := r.queries.GetHTTPBodyUrlEncodedByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyUrlencoded{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPBodyUrlencoded, len(bodies))
	for i, body := range bodies {
		result[i] = DeserializeBodyUrlEncodedGenToModel(body)
	}
	return result, nil
}

func (r *BodyUrlEncodedReader) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	rows, err := r.queries.GetHTTPBodyUrlEncodedByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyUrlencoded{}, nil
		}
		return nil, err
	}

	// Sort by order field
	slices.SortFunc(rows, func(a, b gen.HttpBodyUrlencoded) int {
		if a.DisplayOrder < b.DisplayOrder {
			return -1
		}
		if a.DisplayOrder > b.DisplayOrder {
			return 1
		}
		return 0
	})

	result := make([]mhttp.HTTPBodyUrlencoded, len(rows))
	for i, row := range rows {
		result[i] = DeserializeBodyUrlEncodedGenToModel(row)
	}
	return result, nil
}

func (r *BodyUrlEncodedReader) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPBodyUrlencoded{}, nil
	}

	bodies, err := r.queries.GetHTTPBodyUrlEncodedsByIDs(ctx, ids)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyUrlencoded{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPBodyUrlencoded, len(bodies))
	for i, body := range bodies {
		result[i] = DeserializeBodyUrlEncodedGenToModel(body)
	}
	return result, nil
}

func (r *BodyUrlEncodedReader) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded, error) {
	result := make(map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	bodies, err := r.queries.GetHTTPBodyUrlEncodedsByIDs(ctx, httpIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}

	for _, body := range bodies {
		model := DeserializeBodyUrlEncodedGenToModel(body)
		httpID := model.HttpID
		result[httpID] = append(result[httpID], model)
	}

	return result, nil
}
