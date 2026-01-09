package shttp

import (
	"context"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

type HttpResponseWriter struct {
	queries *gen.Queries
}

func NewHttpResponseWriter(tx gen.DBTX) *HttpResponseWriter {
	return &HttpResponseWriter{
		queries: gen.New(tx),
	}
}

func NewHttpResponseWriterFromQueries(queries *gen.Queries) *HttpResponseWriter {
	return &HttpResponseWriter{
		queries: queries,
	}
}

func (w *HttpResponseWriter) Create(ctx context.Context, response mhttp.HTTPResponse) error {
	dbResponse := ConvertToDBHttpResponse(response)
	return w.queries.CreateHTTPResponse(ctx, gen.CreateHTTPResponseParams(dbResponse))
}

func (w *HttpResponseWriter) CreateHeader(ctx context.Context, header mhttp.HTTPResponseHeader) error {
	return w.queries.CreateHTTPResponseHeader(ctx, gen.CreateHTTPResponseHeaderParams{
		ID:         header.ID,
		ResponseID: header.ResponseID,
		Key:        header.HeaderKey,
		Value:      header.HeaderValue,
		CreatedAt:  header.CreatedAt,
	})
}

func (w *HttpResponseWriter) CreateAssert(ctx context.Context, assert mhttp.HTTPResponseAssert) error {
	return w.queries.CreateHTTPResponseAssert(ctx, gen.CreateHTTPResponseAssertParams{
		ID:         assert.ID,
		ResponseID: assert.ResponseID,
		Value:      assert.Value,
		Success:    assert.Success,
		CreatedAt:  assert.CreatedAt,
	})
}
