package sgraphql

import (
	"context"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

type Writer struct {
	queries *gen.Queries
}

func NewWriterFromQueries(queries *gen.Queries) *Writer {
	return &Writer{queries: queries}
}

func (w *Writer) Create(ctx context.Context, gql *mgraphql.GraphQL) error {
	now := dbtime.DBNow()
	gql.CreatedAt = now.Unix()
	gql.UpdatedAt = now.Unix()

	dbGQL := ConvertToDBGraphQL(*gql)
	return w.queries.CreateGraphQL(ctx, gen.CreateGraphQLParams(dbGQL))
}

func (w *Writer) Update(ctx context.Context, gql *mgraphql.GraphQL) error {
	gql.UpdatedAt = dbtime.DBNow().Unix()

	dbGQL := ConvertToDBGraphQL(*gql)

	if gql.IsDelta {
		// Update delta fields for delta records
		if err := w.queries.UpdateGraphQLDelta(ctx, gen.UpdateGraphQLDeltaParams{
			ID:               dbGQL.ID,
			DeltaName:        dbGQL.DeltaName,
			DeltaUrl:         dbGQL.DeltaUrl,
			DeltaQuery:       dbGQL.DeltaQuery,
			DeltaVariables:   dbGQL.DeltaVariables,
			DeltaDescription: dbGQL.DeltaDescription,
		}); err != nil {
			return err
		}
		// Fallthrough to update common fields (like LastRunAt)
	}

	var lastRunAt interface{}
	if gql.LastRunAt != nil {
		lastRunAt = *gql.LastRunAt
	}

	// Update base fields
	return w.queries.UpdateGraphQL(ctx, gen.UpdateGraphQLParams{
		ID:          gql.ID,
		Name:        gql.Name,
		Url:         gql.Url,
		Query:       gql.Query,
		Variables:   gql.Variables,
		Description: gql.Description,
		LastRunAt:   lastRunAt,
	})
}

func (w *Writer) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteGraphQL(ctx, id)
}

func (w *Writer) CreateGraphQLVersion(ctx context.Context, graphqlID, createdBy idwrap.IDWrap, versionName, versionDescription string) (*mgraphql.GraphQLVersion, error) {
	id := idwrap.NewNow()
	now := dbtime.DBNow().Unix()

	err := w.queries.CreateGraphQLVersion(ctx, gen.CreateGraphQLVersionParams{
		ID:                 id.Bytes(),
		GraphqlID:          graphqlID.Bytes(),
		VersionName:        versionName,
		VersionDescription: versionDescription,
		IsActive:           true,
		CreatedAt:          now,
		CreatedBy:          createdBy.Bytes(),
	})
	if err != nil {
		return nil, err
	}

	return &mgraphql.GraphQLVersion{
		ID:                 id,
		GraphQLID:          graphqlID,
		VersionName:        versionName,
		VersionDescription: versionDescription,
		IsActive:           true,
		CreatedAt:          now,
		CreatedBy:          &createdBy,
	}, nil
}
