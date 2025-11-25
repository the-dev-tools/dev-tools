package shttp

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"time"
)

var ErrNoHttpResponseFound = errors.New("no HTTP response found")

type HttpResponseService struct {
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
		status = int32(s)
	}

	var duration int32
	if d, ok := response.Duration.(int32); ok {
		duration = d
	} else if d, ok := response.Duration.(int64); ok {
		duration = int32(d)
	}

	var size int32
	if s, ok := response.Size.(int32); ok {
		size = s
	} else if s, ok := response.Size.(int64); ok {
		size = int32(s)
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
	return HttpResponseService{queries: queries}
}

func (hrs HttpResponseService) TX(tx *sql.Tx) HttpResponseService {
	return HttpResponseService{queries: hrs.queries.WithTx(tx)}
}

func (hrs HttpResponseService) Create(ctx context.Context, response mhttp.HTTPResponse) error {
	dbResponse := ConvertToDBHttpResponse(response)
	return hrs.queries.CreateHTTPResponse(ctx, gen.CreateHTTPResponseParams{
		ID:        dbResponse.ID,
		HttpID:    dbResponse.HttpID,
		Status:    dbResponse.Status,
		Body:      dbResponse.Body,
		Time:      dbResponse.Time,
		Duration:  dbResponse.Duration,
		Size:      dbResponse.Size,
		CreatedAt: dbResponse.CreatedAt,
	})
}

func (hrs HttpResponseService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponse, error) {
	responses, err := hrs.queries.GetHTTPResponsesByHttpID(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
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

func (hrs HttpResponseService) CreateHeader(ctx context.Context, header mhttp.HTTPResponseHeader) error {
	return hrs.queries.CreateHTTPResponseHeader(ctx, gen.CreateHTTPResponseHeaderParams{
		ID:         header.ID,
		ResponseID: header.ResponseID,
		Key:        header.HeaderKey,
		Value:      header.HeaderValue,
		CreatedAt:  header.CreatedAt,
	})
}

func (hrs HttpResponseService) GetHeadersByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponseHeader, error) {

	headers, err := hrs.queries.GetHTTPResponseHeadersByHttpID(ctx, httpID)

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



func (hrs HttpResponseService) GetAssertsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPResponseAssert, error) {

	asserts, err := hrs.queries.GetHTTPResponseAssertsByHttpID(ctx, httpID)

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



func (hrs HttpResponseService) CreateAssert(ctx context.Context, assert mhttp.HTTPResponseAssert) error {

	return hrs.queries.CreateHTTPResponseAssert(ctx, gen.CreateHTTPResponseAssertParams{

		ID:         assert.ID,

		ResponseID: assert.ResponseID,

		Value:      assert.Value,

		Success:    assert.Success,

		CreatedAt:  assert.CreatedAt,

	})

}
