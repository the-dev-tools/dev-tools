package sgraphql

import (
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
)

func interfaceToInt64Ptr(v interface{}) *int64 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case int64:
		return &val
	case int:
		i := int64(val)
		return &i
	default:
		return nil
	}
}

func interfaceToInt32(v interface{}) int32 {
	switch val := v.(type) {
	case int32:
		return val
	case int64:
		return int32(val) //nolint:gosec // G115
	default:
		return 0
	}
}

func ConvertToDBGraphQL(gql mgraphql.GraphQL) gen.Graphql {
	var lastRunAt interface{}
	if gql.LastRunAt != nil {
		lastRunAt = *gql.LastRunAt
	}

	return gen.Graphql{
		ID:          gql.ID,
		WorkspaceID: gql.WorkspaceID,
		FolderID:    gql.FolderID,
		Name:        gql.Name,
		Url:         gql.Url,
		Query:       gql.Query,
		Variables:   gql.Variables,
		Description: gql.Description,
		LastRunAt:   lastRunAt,
		CreatedAt:   gql.CreatedAt,
		UpdatedAt:   gql.UpdatedAt,
	}
}

func ConvertToModelGraphQL(gql gen.Graphql) *mgraphql.GraphQL {
	return &mgraphql.GraphQL{
		ID:          gql.ID,
		WorkspaceID: gql.WorkspaceID,
		FolderID:    gql.FolderID,
		Name:        gql.Name,
		Url:         gql.Url,
		Query:       gql.Query,
		Variables:   gql.Variables,
		Description: gql.Description,
		LastRunAt:   interfaceToInt64Ptr(gql.LastRunAt),
		CreatedAt:   gql.CreatedAt,
		UpdatedAt:   gql.UpdatedAt,
	}
}

func ConvertToDBGraphQLResponse(resp mgraphql.GraphQLResponse) gen.GraphqlResponse {
	return gen.GraphqlResponse{
		ID:        resp.ID,
		GraphqlID: resp.GraphQLID,
		Status:    resp.Status,
		Body:      resp.Body,
		Time:      time.Unix(resp.Time, 0),
		Duration:  resp.Duration,
		Size:      resp.Size,
		CreatedAt: resp.CreatedAt,
	}
}

func ConvertToModelGraphQLResponse(resp gen.GraphqlResponse) mgraphql.GraphQLResponse {
	return mgraphql.GraphQLResponse{
		ID:        resp.ID,
		GraphQLID: resp.GraphqlID,
		Status:    interfaceToInt32(resp.Status),
		Body:      resp.Body,
		Time:      resp.Time.Unix(),
		Duration:  interfaceToInt32(resp.Duration),
		Size:      interfaceToInt32(resp.Size),
		CreatedAt: resp.CreatedAt,
	}
}

func ConvertToModelGraphQLHeader(h gen.GraphqlHeader) mgraphql.GraphQLHeader {
	return mgraphql.GraphQLHeader{
		ID:           h.ID,
		GraphQLID:    h.GraphqlID,
		Key:          h.HeaderKey,
		Value:        h.HeaderValue,
		Enabled:      h.Enabled,
		Description:  h.Description,
		DisplayOrder: float32(h.DisplayOrder),
		CreatedAt:    h.CreatedAt,
		UpdatedAt:    h.UpdatedAt,
	}
}
