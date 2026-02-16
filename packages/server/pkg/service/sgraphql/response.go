package sgraphql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

var ErrNoGraphQLResponseFound = errors.New("no graphql response found")

type GraphQLResponseService struct {
	queries *gen.Queries
}

func NewGraphQLResponseService(queries *gen.Queries) GraphQLResponseService {
	return GraphQLResponseService{queries: queries}
}

func (s GraphQLResponseService) TX(tx *sql.Tx) GraphQLResponseService {
	return GraphQLResponseService{queries: s.queries.WithTx(tx)}
}

func (s GraphQLResponseService) Create(ctx context.Context, resp mgraphql.GraphQLResponse) error {
	return s.queries.CreateGraphQLResponse(ctx, gen.CreateGraphQLResponseParams{
		ID:        resp.ID,
		GraphqlID: resp.GraphQLID,
		Status:    resp.Status,
		Body:      resp.Body,
		Time:      time.Unix(resp.Time, 0),
		Duration:  resp.Duration,
		Size:      resp.Size,
		CreatedAt: resp.CreatedAt,
	})
}

func (s GraphQLResponseService) GetByGraphQLID(ctx context.Context, graphqlID idwrap.IDWrap) ([]mgraphql.GraphQLResponse, error) {
	responses, err := s.queries.GetGraphQLResponsesByGraphQLID(ctx, graphqlID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLResponse{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLResponse, len(responses))
	for i, resp := range responses {
		result[i] = ConvertToModelGraphQLResponse(resp)
	}
	return result, nil
}

func (s GraphQLResponseService) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mgraphql.GraphQLResponse, error) {
	responses, err := s.queries.GetGraphQLResponsesByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLResponse{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLResponse, len(responses))
	for i, resp := range responses {
		result[i] = ConvertToModelGraphQLResponse(resp)
	}
	return result, nil
}

func (s GraphQLResponseService) CreateHeader(ctx context.Context, header mgraphql.GraphQLResponseHeader) error {
	return s.queries.CreateGraphQLResponseHeader(ctx, gen.CreateGraphQLResponseHeaderParams{
		ID:         header.ID,
		ResponseID: header.ResponseID,
		Key:        header.HeaderKey,
		Value:      header.HeaderValue,
		CreatedAt:  header.CreatedAt,
	})
}

func (s GraphQLResponseService) GetHeadersByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mgraphql.GraphQLResponseHeader, error) {
	headers, err := s.queries.GetGraphQLResponseHeadersByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLResponseHeader{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLResponseHeader, len(headers))
	for i, h := range headers {
		result[i] = mgraphql.GraphQLResponseHeader{
			ID:          h.ID,
			ResponseID:  h.ResponseID,
			HeaderKey:   h.Key,
			HeaderValue: h.Value,
			CreatedAt:   h.CreatedAt,
		}
	}
	return result, nil
}

func (s GraphQLResponseService) GetHeadersByResponseID(ctx context.Context, responseID idwrap.IDWrap) ([]mgraphql.GraphQLResponseHeader, error) {
	headers, err := s.queries.GetGraphQLResponseHeadersByResponseID(ctx, responseID)
	if err != nil {
		return nil, err
	}

	result := make([]mgraphql.GraphQLResponseHeader, len(headers))
	for i, h := range headers {
		result[i] = mgraphql.GraphQLResponseHeader{
			ID:          h.ID,
			ResponseID:  h.ResponseID,
			HeaderKey:   h.Key,
			HeaderValue: h.Value,
			CreatedAt:   h.CreatedAt,
		}
	}
	return result, nil
}


func (s GraphQLResponseService) CreateAssert(ctx context.Context, assert mgraphql.GraphQLResponseAssert) error {
	return s.queries.CreateGraphQLResponseAssert(ctx, gen.CreateGraphQLResponseAssertParams{
		ID:         assert.ID.Bytes(),
		ResponseID: assert.ResponseID.Bytes(),
		Value:      assert.Value,
		Success:    assert.Success,
		CreatedAt:  assert.CreatedAt,
	})
}

func (s GraphQLResponseService) GetAssertsByResponseID(ctx context.Context, responseID idwrap.IDWrap) ([]mgraphql.GraphQLResponseAssert, error) {
	asserts, err := s.queries.GetGraphQLResponseAssertsByResponseID(ctx, responseID.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLResponseAssert{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLResponseAssert, len(asserts))
	for i, a := range asserts {
		id, _ := idwrap.NewFromBytes(a.ID)
		respID, _ := idwrap.NewFromBytes(a.ResponseID)

		result[i] = mgraphql.GraphQLResponseAssert{
			ID:         id,
			ResponseID: respID,
			Value:      a.Value,
			Success:    a.Success,
			CreatedAt:  a.CreatedAt,
		}
	}
	return result, nil
}

func (s GraphQLResponseService) GetAssertsByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mgraphql.GraphQLResponseAssert, error) {
	asserts, err := s.queries.GetGraphQLResponseAssertsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLResponseAssert{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLResponseAssert, len(asserts))
	for i, a := range asserts {
		id, _ := idwrap.NewFromBytes(a.ID)
		respID, _ := idwrap.NewFromBytes(a.ResponseID)

		result[i] = mgraphql.GraphQLResponseAssert{
			ID:         id,
			ResponseID: respID,
			Value:      a.Value,
			Success:    a.Success,
			CreatedAt:  a.CreatedAt,
		}
	}
	return result, nil
}

