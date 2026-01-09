package shttp

import (
	"context"
	"database/sql"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

type HeaderReader struct {
	queries *gen.Queries
}

func NewHeaderReader(db *sql.DB) *HeaderReader {
	return &HeaderReader{queries: gen.New(db)}
}

func NewHeaderReaderFromQueries(queries *gen.Queries) *HeaderReader {
	return &HeaderReader{queries: queries}
}

func (r *HeaderReader) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	dbHeaders, err := r.queries.GetHTTPHeaders(ctx, httpID)
	if err != nil {
		return nil, err
	}

	var headers []mhttp.HTTPHeader
	for _, dbHeader := range dbHeaders {
		header := DeserializeHeaderGenToModel(dbHeader)
		headers = append(headers, header)
	}

	return headers, nil
}

func (r *HeaderReader) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	return r.GetByHttpID(ctx, httpID)
}

func (r *HeaderReader) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPHeader{}, nil
	}

	dbHeaders, err := r.queries.GetHTTPHeadersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	var headers []mhttp.HTTPHeader
	for _, dbHeader := range dbHeaders {
		header := DeserializeHeaderGenToModel(dbHeader)
		headers = append(headers, header)
	}

	return headers, nil
}

func (r *HeaderReader) GetByID(ctx context.Context, headerID idwrap.IDWrap) (mhttp.HTTPHeader, error) {
	headers, err := r.GetByIDs(ctx, []idwrap.IDWrap{headerID})
	if err != nil {
		return mhttp.HTTPHeader{}, err
	}

	if len(headers) == 0 {
		return mhttp.HTTPHeader{}, ErrNoHttpHeaderFound
	}

	return headers[0], nil
}
