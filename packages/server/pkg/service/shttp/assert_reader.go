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

type AssertReader struct {
	queries *gen.Queries
}

func NewAssertReader(db *sql.DB) *AssertReader {
	return &AssertReader{queries: gen.New(db)}
}

func NewAssertReaderFromQueries(queries *gen.Queries) *AssertReader {
	return &AssertReader{queries: queries}
}

func (r *AssertReader) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPAssert, error) {
	assert, err := r.queries.GetHTTPAssert(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHttpAssertFound
		}
		return nil, err
	}

	model := DeserializeAssertGenToModel(assert)
	return &model, nil
}

func (r *AssertReader) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	rows, err := r.queries.GetHTTPAssertsByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPAssert{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPAssert, len(rows))
	for i, row := range rows {
		result[i] = DeserializeAssertGenToModel(row)
	}
	return result, nil
}

func (r *AssertReader) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	rows, err := r.queries.GetHTTPAssertsByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPAssert{}, nil
		}
		return nil, err
	}

	// Sort by order field
	slices.SortFunc(rows, func(a, b gen.HttpAssert) int {
		if a.DisplayOrder < b.DisplayOrder {
			return -1
		}
		if a.DisplayOrder > b.DisplayOrder {
			return 1
		}
		return 0
	})

	result := make([]mhttp.HTTPAssert, len(rows))
	for i, row := range rows {
		result[i] = DeserializeAssertGenToModel(row)
	}
	return result, nil
}

func (r *AssertReader) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPAssert{}, nil
	}

	rows, err := r.queries.GetHTTPAssertsByIDs(ctx, ids)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPAssert{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPAssert, len(rows))
	for i, row := range rows {
		result[i] = DeserializeAssertGenToModel(row)
	}
	return result, nil
}

func (r *AssertReader) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPAssert, error) {
	result := make(map[idwrap.IDWrap][]mhttp.HTTPAssert, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	for _, httpID := range httpIDs {
		asserts, err := r.GetByHttpID(ctx, httpID)
		if err != nil {
			return nil, err
		}
		if len(asserts) > 0 {
			result[httpID] = asserts
		}
	}

	return result, nil
}
