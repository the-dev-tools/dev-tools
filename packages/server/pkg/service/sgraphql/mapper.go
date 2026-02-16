package sgraphql

import (
	"time"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
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

func interfaceToStringPtr(v interface{}) *string {
	if v == nil {
		return nil
	}
	if str, ok := v.(string); ok {
		return &str
	}
	return nil
}

func interfaceToBoolPtr(v interface{}) *bool {
	if v == nil {
		return nil
	}
	if b, ok := v.(bool); ok {
		return &b
	}
	return nil
}

func interfaceToFloat32Ptr(v interface{}) *float32 {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case float32:
		return &val
	case float64:
		f32 := float32(val)
		return &f32
	default:
		return nil
	}
}

func bytesToIDWrapPtr(b []byte) *idwrap.IDWrap {
	if b == nil || len(b) == 0 {
		return nil
	}
	id, err := idwrap.NewFromBytes(b)
	if err != nil {
		return nil
	}
	return &id
}

func idWrapPtrToBytes(id *idwrap.IDWrap) []byte {
	if id == nil {
		return nil
	}
	return id.Bytes()
}

func stringPtrToInterface(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func boolPtrToInterface(b *bool) interface{} {
	if b == nil {
		return nil
	}
	return *b
}

func float32PtrToInterface(f *float32) interface{} {
	if f == nil {
		return nil
	}
	return float64(*f)
}

func ConvertToDBGraphQL(gql mgraphql.GraphQL) gen.Graphql {
	var lastRunAt interface{}
	if gql.LastRunAt != nil {
		lastRunAt = *gql.LastRunAt
	}

	return gen.Graphql{
		ID:               gql.ID,
		WorkspaceID:      gql.WorkspaceID,
		FolderID:         gql.FolderID,
		Name:             gql.Name,
		Url:              gql.Url,
		Query:            gql.Query,
		Variables:        gql.Variables,
		Description:      gql.Description,
		ParentGraphqlID:  idWrapPtrToBytes(gql.ParentGraphQLID),
		IsDelta:          gql.IsDelta,
		IsSnapshot:       gql.IsSnapshot,
		DeltaName:        stringPtrToInterface(gql.DeltaName),
		DeltaUrl:         stringPtrToInterface(gql.DeltaUrl),
		DeltaQuery:       stringPtrToInterface(gql.DeltaQuery),
		DeltaVariables:   stringPtrToInterface(gql.DeltaVariables),
		DeltaDescription: stringPtrToInterface(gql.DeltaDescription),
		LastRunAt:        lastRunAt,
		CreatedAt:        gql.CreatedAt,
		UpdatedAt:        gql.UpdatedAt,
	}
}

func ConvertToModelGraphQL(gql gen.Graphql) *mgraphql.GraphQL {
	return &mgraphql.GraphQL{
		ID:               gql.ID,
		WorkspaceID:      gql.WorkspaceID,
		FolderID:         gql.FolderID,
		Name:             gql.Name,
		Url:              gql.Url,
		Query:            gql.Query,
		Variables:        gql.Variables,
		Description:      gql.Description,
		ParentGraphQLID:  bytesToIDWrapPtr(gql.ParentGraphqlID),
		IsDelta:          gql.IsDelta,
		IsSnapshot:       gql.IsSnapshot,
		DeltaName:        interfaceToStringPtr(gql.DeltaName),
		DeltaUrl:         interfaceToStringPtr(gql.DeltaUrl),
		DeltaQuery:       interfaceToStringPtr(gql.DeltaQuery),
		DeltaVariables:   interfaceToStringPtr(gql.DeltaVariables),
		DeltaDescription: interfaceToStringPtr(gql.DeltaDescription),
		LastRunAt:        interfaceToInt64Ptr(gql.LastRunAt),
		CreatedAt:        gql.CreatedAt,
		UpdatedAt:        gql.UpdatedAt,
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
		ID:                    h.ID,
		GraphQLID:             h.GraphqlID,
		Key:                   h.HeaderKey,
		Value:                 h.HeaderValue,
		Enabled:               h.Enabled,
		Description:           h.Description,
		DisplayOrder:          float32(h.DisplayOrder),
		ParentGraphQLHeaderID: bytesToIDWrapPtr(h.ParentGraphqlHeaderID),
		IsDelta:               h.IsDelta,
		DeltaKey:              interfaceToStringPtr(h.DeltaHeaderKey),
		DeltaValue:            interfaceToStringPtr(h.DeltaHeaderValue),
		DeltaEnabled:          interfaceToBoolPtr(h.DeltaEnabled),
		DeltaDescription:      interfaceToStringPtr(h.DeltaDescription),
		DeltaDisplayOrder:     interfaceToFloat32Ptr(h.DeltaDisplayOrder),
		CreatedAt:             h.CreatedAt,
		UpdatedAt:             h.UpdatedAt,
	}
}

func ConvertToModelGraphQLAssert(a gen.GraphqlAssert) mgraphql.GraphQLAssert {
	id, _ := idwrap.NewFromBytes(a.ID)
	graphqlID, _ := idwrap.NewFromBytes(a.GraphqlID)

	return mgraphql.GraphQLAssert{
		ID:                    id,
		GraphQLID:             graphqlID,
		Value:                 a.Value,
		Enabled:               a.Enabled,
		Description:           a.Description,
		DisplayOrder:          float32(a.DisplayOrder),
		ParentGraphQLAssertID: bytesToIDWrapPtr(a.ParentGraphqlAssertID),
		IsDelta:               a.IsDelta,
		DeltaValue:            interfaceToStringPtr(a.DeltaValue),
		DeltaEnabled:          interfaceToBoolPtr(a.DeltaEnabled),
		DeltaDescription:      interfaceToStringPtr(a.DeltaDescription),
		DeltaDisplayOrder:     interfaceToFloat32Ptr(a.DeltaDisplayOrder),
		CreatedAt:             a.CreatedAt,
		UpdatedAt:             a.UpdatedAt,
	}
}
