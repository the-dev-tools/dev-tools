package sgraphql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

var ErrNoGraphQLHeaderFound = errors.New("no graphql header found")

type GraphQLHeaderService struct {
	queries *gen.Queries
}

func NewGraphQLHeaderService(queries *gen.Queries) GraphQLHeaderService {
	return GraphQLHeaderService{queries: queries}
}

func (s GraphQLHeaderService) TX(tx *sql.Tx) GraphQLHeaderService {
	return GraphQLHeaderService{queries: s.queries.WithTx(tx)}
}

func (s GraphQLHeaderService) GetByGraphQLID(ctx context.Context, graphqlID idwrap.IDWrap) ([]mgraphql.GraphQLHeader, error) {
	headers, err := s.queries.GetGraphQLHeaders(ctx, graphqlID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLHeader{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLHeader, len(headers))
	for i, h := range headers {
		result[i] = ConvertToModelGraphQLHeader(h)
	}
	return result, nil
}

func (s GraphQLHeaderService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mgraphql.GraphQLHeader, error) {
	headers, err := s.queries.GetGraphQLHeadersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	result := make([]mgraphql.GraphQLHeader, len(headers))
	for i, h := range headers {
		result[i] = ConvertToModelGraphQLHeader(h)
	}
	return result, nil
}

func (s GraphQLHeaderService) Create(ctx context.Context, header *mgraphql.GraphQLHeader) error {
	now := dbtime.DBNow()
	header.CreatedAt = now.Unix()
	header.UpdatedAt = now.Unix()

	return s.queries.CreateGraphQLHeader(ctx, gen.CreateGraphQLHeaderParams{
		ID:           header.ID,
		GraphqlID:    header.GraphQLID,
		HeaderKey:    header.Key,
		HeaderValue:  header.Value,
		Description:  header.Description,
		Enabled:      header.Enabled,
		DisplayOrder: float64(header.DisplayOrder),
		CreatedAt:    header.CreatedAt,
		UpdatedAt:    header.UpdatedAt,
	})
}

func (s GraphQLHeaderService) Update(ctx context.Context, header *mgraphql.GraphQLHeader) error {
	return s.queries.UpdateGraphQLHeader(ctx, gen.UpdateGraphQLHeaderParams{
		ID:           header.ID,
		HeaderKey:    header.Key,
		HeaderValue:  header.Value,
		Description:  header.Description,
		Enabled:      header.Enabled,
		DisplayOrder: float64(header.DisplayOrder),
	})
}

func (s GraphQLHeaderService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.DeleteGraphQLHeader(ctx, id)
}
