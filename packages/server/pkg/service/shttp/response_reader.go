package shttp

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

type HttpResponseReader struct {
	queries *gen.Queries
}

func NewHttpResponseReader(db *sql.DB) *HttpResponseReader {
	return &HttpResponseReader{queries: gen.New(db)}
}

func NewHttpResponseReaderFromQueries(queries *gen.Queries) *HttpResponseReader {
	return &HttpResponseReader{queries: queries}
}

func (r *HttpResponseReader) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponse, error) {
	responses, err := r.queries.GetHTTPResponsesByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPResponse{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPResponse, len(responses))
	for i, response := range responses {
		result[i] = ConvertToModelHttpResponse(response)
	}
	return result, nil
}

func (r *HttpResponseReader) GetHeadersByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponseHeader, error) {
	headers, err := r.queries.GetHTTPResponseHeadersByHttpID(ctx, httpID)
	if err != nil {
		return nil, err
	}

	result := make([]mhttp.HTTPResponseHeader, len(headers))
	for i, header := range headers {
		result[i] = mhttp.HTTPResponseHeader{
			ID:          header.ID,
			ResponseID:  header.ResponseID,
			HeaderKey:   header.Key,
			HeaderValue: header.Value,
			CreatedAt:   header.CreatedAt,
		}
	}
	return result, nil
}

func (r *HttpResponseReader) GetAssertsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponseAssert, error) {
	asserts, err := r.queries.GetHTTPResponseAssertsByHttpID(ctx, httpID)
	if err != nil {
		return nil, err
	}

	result := make([]mhttp.HTTPResponseAssert, len(asserts))
	for i, assert := range asserts {
		result[i] = mhttp.HTTPResponseAssert{
			ID:         assert.ID,
			ResponseID: assert.ResponseID,
			Value:      assert.Value,
			Success:    assert.Success,
			CreatedAt:  assert.CreatedAt,
		}
	}
	return result, nil
}
