package sgraphql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

type Reader struct {
	queries *gen.Queries
	logger  *slog.Logger
}

func NewReader(db *sql.DB, logger *slog.Logger) *Reader {
	return &Reader{
		queries: gen.New(db),
		logger:  logger,
	}
}

func NewReaderFromQueries(queries *gen.Queries, logger *slog.Logger) *Reader {
	return &Reader{
		queries: queries,
		logger:  logger,
	}
}

func (r *Reader) Get(ctx context.Context, id idwrap.IDWrap) (*mgraphql.GraphQL, error) {
	gql, err := r.queries.GetGraphQL(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if r.logger != nil {
				r.logger.DebugContext(ctx, fmt.Sprintf("GraphQL ID: %s not found", id.String()))
			}
			return nil, ErrNoGraphQLFound
		}
		return nil, err
	}
	return ConvertToModelGraphQL(gql), nil
}

func (r *Reader) GetByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mgraphql.GraphQL, error) {
	gqls, err := r.queries.GetGraphQLsByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQL{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQL, len(gqls))
	for i, gql := range gqls {
		result[i] = *ConvertToModelGraphQL(gql)
	}
	return result, nil
}

func (r *Reader) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	workspaceID, err := r.queries.GetGraphQLWorkspaceID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return idwrap.IDWrap{}, ErrNoGraphQLFound
		}
		return idwrap.IDWrap{}, err
	}
	return workspaceID, nil
}

func (r *Reader) GetDeltasByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mgraphql.GraphQL, error) {
	gqls, err := r.queries.GetGraphQLDeltasByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQL{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQL, len(gqls))
	for i, gql := range gqls {
		result[i] = *ConvertToModelGraphQL(gql)
	}
	return result, nil
}

func (r *Reader) GetDeltasByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mgraphql.GraphQL, error) {
	gqls, err := r.queries.GetGraphQLDeltasByParentID(ctx, parentID.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQL{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQL, len(gqls))
	for i, gql := range gqls {
		result[i] = *ConvertToModelGraphQL(gql)
	}
	return result, nil
}

func (r *Reader) GetGraphQLVersionsByGraphQLID(ctx context.Context, graphqlID idwrap.IDWrap) ([]mgraphql.GraphQLVersion, error) {
	versions, err := r.queries.GetGraphQLVersionsByGraphQLID(ctx, graphqlID.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLVersion{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLVersion, len(versions))
	for i, v := range versions {
		var createdBy *idwrap.IDWrap
		if len(v.CreatedBy) > 0 {
			id, err := idwrap.NewFromBytes(v.CreatedBy)
			if err == nil {
				createdBy = &id
			}
		}

		id, _ := idwrap.NewFromBytes(v.ID)
		gqlID, _ := idwrap.NewFromBytes(v.GraphqlID)

		result[i] = mgraphql.GraphQLVersion{
			ID:                 id,
			GraphQLID:          gqlID,
			VersionName:        v.VersionName,
			VersionDescription: v.VersionDescription,
			IsActive:           v.IsActive,
			CreatedAt:          v.CreatedAt,
			CreatedBy:          createdBy,
		}
	}
	return result, nil
}
