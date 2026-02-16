//nolint:revive // exported
package resolver

import (
	"context"
	"sort"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/delta"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mgraphql"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/sgraphql"
)

// GraphQLResolver defines the interface for resolving GraphQL requests with their delta overlays.
type GraphQLResolver interface {
	Resolve(ctx context.Context, baseID idwrap.IDWrap, deltaID *idwrap.IDWrap) (*delta.ResolveGraphQLOutput, error)
}

// StandardResolver implements GraphQLResolver using standard DB services.
type StandardResolver struct {
	graphqlService       *sgraphql.Reader
	graphqlHeaderService *sgraphql.GraphQLHeaderService
	graphqlAssertService *sgraphql.GraphQLAssertService
}

// NewStandardResolver creates a new instance of StandardResolver.
func NewStandardResolver(
	graphqlService *sgraphql.Reader,
	graphqlHeaderService *sgraphql.GraphQLHeaderService,
	graphqlAssertService *sgraphql.GraphQLAssertService,
) *StandardResolver {
	return &StandardResolver{
		graphqlService:       graphqlService,
		graphqlHeaderService: graphqlHeaderService,
		graphqlAssertService: graphqlAssertService,
	}
}

// Resolve fetches base and delta components and resolves them into a final GraphQL request.
func (r *StandardResolver) Resolve(ctx context.Context, baseID idwrap.IDWrap, deltaID *idwrap.IDWrap) (*delta.ResolveGraphQLOutput, error) {
	// 1. Fetch Base Components
	baseGraphQL, err := r.graphqlService.Get(ctx, baseID)
	if err != nil {
		return nil, err
	}

	baseHeaders, _ := r.graphqlHeaderService.GetByGraphQLID(ctx, baseID)
	baseAsserts, _ := r.graphqlAssertService.GetByGraphQLID(ctx, baseID)

	// 2. Fetch Delta Components (if present)
	var deltaGraphQL *mgraphql.GraphQL
	var deltaHeaders []mgraphql.GraphQLHeader
	var deltaAsserts []mgraphql.GraphQLAssert

	if deltaID != nil {
		d, err := r.graphqlService.Get(ctx, *deltaID)
		if err != nil {
			return nil, err
		}
		deltaGraphQL = d

		deltaHeaders, _ = r.graphqlHeaderService.GetByGraphQLID(ctx, *deltaID)
		deltaAsserts, _ = r.graphqlAssertService.GetByGraphQLID(ctx, *deltaID)
	}

	// 3. Prepare Input for Delta Resolution
	input := delta.ResolveGraphQLInput{
		Base:        *baseGraphQL,
		BaseHeaders: convertGraphQLHeaders(baseHeaders),
		BaseAsserts: convertGraphQLAsserts(baseAsserts),
	}

	if deltaGraphQL != nil {
		input.Delta = *deltaGraphQL
		input.DeltaHeaders = convertGraphQLHeaders(deltaHeaders)
		input.DeltaAsserts = convertGraphQLAsserts(deltaAsserts)
	}

	// 4. Resolve
	output := delta.ResolveGraphQL(input)
	return &output, nil
}

// Helper functions for type conversion

func convertGraphQLHeaders(in []mgraphql.GraphQLHeader) []mgraphql.GraphQLHeader {
	if in == nil {
		return []mgraphql.GraphQLHeader{}
	}
	out := make([]mgraphql.GraphQLHeader, len(in))
	for i, v := range in {
		out[i] = mgraphql.GraphQLHeader{
			ID:                     v.ID,
			GraphQLID:              v.GraphQLID,
			Key:                    v.Key,
			Value:                  v.Value,
			Description:            v.Description,
			Enabled:                v.Enabled,
			ParentGraphQLHeaderID:  v.ParentGraphQLHeaderID,
			IsDelta:                v.IsDelta,
			DeltaKey:               v.DeltaKey,
			DeltaValue:             v.DeltaValue,
			DeltaDescription:       v.DeltaDescription,
			DeltaEnabled:           v.DeltaEnabled,
			DisplayOrder:           v.DisplayOrder,
			CreatedAt:              v.CreatedAt,
			UpdatedAt:              v.UpdatedAt,
		}
	}
	return out
}

// convertGraphQLAsserts converts DB model asserts (ordered by float) to mgraphql model asserts.
func convertGraphQLAsserts(in []mgraphql.GraphQLAssert) []mgraphql.GraphQLAssert {
	if len(in) == 0 {
		return []mgraphql.GraphQLAssert{}
	}

	// Sort by DisplayOrder (DB model uses float ordering)
	sorted := make([]mgraphql.GraphQLAssert, len(in))
	copy(sorted, in)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].DisplayOrder < sorted[j].DisplayOrder
	})

	return sorted
}
