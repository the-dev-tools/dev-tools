//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"
	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"time"
)

var ErrNoHttpResponseFound = errors.New("no HTTP response found")

type HttpResponseService struct {
	reader  *HttpResponseReader
	queries *gen.Queries
}

func ConvertToDBHttpResponse(response mhttp.HTTPResponse) gen.HttpResponse {
	return gen.HttpResponse{
		ID:        response.ID,
		HttpID:    response.HttpID,
		Status:    response.Status,
		Body:      response.Body,
		Time:      time.Unix(response.Time, 0),
		Duration:  response.Duration,
		Size:      response.Size,
		CreatedAt: response.CreatedAt,
	}
}

func ConvertToModelHttpResponse(response gen.HttpResponse) mhttp.HTTPResponse {
	// Type assertions for interface{} fields
	var status int32
	if s, ok := response.Status.(int32); ok {
		status = s
	} else if s, ok := response.Status.(int64); ok {
		status = int32(s) // nolint:gosec // G115
	}

	var duration int32
	if d, ok := response.Duration.(int32); ok {
		duration = d
	} else if d, ok := response.Duration.(int64); ok {
		duration = int32(d) // nolint:gosec // G115
	}

	var size int32
	if s, ok := response.Size.(int32); ok {
		size = s
	} else if s, ok := response.Size.(int64); ok {
		size = int32(s) // nolint:gosec // G115
	}

	return mhttp.HTTPResponse{
		ID:        response.ID,
		HttpID:    response.HttpID,
		Status:    status,
		Body:      response.Body,
		Time:      response.Time.Unix(),
		Duration:  duration,
		Size:      size,
		CreatedAt: response.CreatedAt,
	}
}

func NewHttpResponseService(queries *gen.Queries) HttpResponseService {
	return HttpResponseService{
		reader:  NewHttpResponseReaderFromQueries(queries),
		queries: queries,
	}
}

func (hrs HttpResponseService) TX(tx *sql.Tx) HttpResponseService {
	newQueries := hrs.queries.WithTx(tx)
	return HttpResponseService{
		reader:  NewHttpResponseReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (hrs HttpResponseService) Create(ctx context.Context, response mhttp.HTTPResponse) error {
	return NewHttpResponseWriterFromQueries(hrs.queries).Create(ctx, response)
}

func (hrs HttpResponseService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponse, error) {
	return hrs.reader.GetByHttpID(ctx, httpID)
}

func (hrs HttpResponseService) CreateHeader(ctx context.Context, header mhttp.HTTPResponseHeader) error {
	return NewHttpResponseWriterFromQueries(hrs.queries).CreateHeader(ctx, header)
}

func (hrs HttpResponseService) GetHeadersByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponseHeader, error) {
	return hrs.reader.GetHeadersByHttpID(ctx, httpID)
}

func (hrs HttpResponseService) GetAssertsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponseAssert, error) {
	return hrs.reader.GetAssertsByHttpID(ctx, httpID)
}

func (hrs HttpResponseService) CreateAssert(ctx context.Context, assert mhttp.HTTPResponseAssert) error {
	return NewHttpResponseWriterFromQueries(hrs.queries).CreateAssert(ctx, assert)
}

func (hrs HttpResponseService) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTPResponse, error) {
	return hrs.reader.GetByWorkspaceID(ctx, workspaceID)
}

func (hrs HttpResponseService) GetHeadersByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTPResponseHeader, error) {
	return hrs.reader.GetHeadersByWorkspaceID(ctx, workspaceID)
}

func (hrs HttpResponseService) GetAssertsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mhttp.HTTPResponseAssert, error) {
	return hrs.reader.GetAssertsByWorkspaceID(ctx, workspaceID)
}

func (s HttpResponseService) Reader() *HttpResponseReader { return s.reader }
