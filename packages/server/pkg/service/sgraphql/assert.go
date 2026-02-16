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

var ErrNoGraphQLAssertFound = errors.New("no graphql assert found")

type GraphQLAssertService struct {
	queries *gen.Queries
}

func NewGraphQLAssertService(queries *gen.Queries) GraphQLAssertService {
	return GraphQLAssertService{queries: queries}
}

func (s GraphQLAssertService) TX(tx *sql.Tx) GraphQLAssertService {
	return GraphQLAssertService{queries: s.queries.WithTx(tx)}
}

func (s GraphQLAssertService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mgraphql.GraphQLAssert, error) {
	assert, err := s.queries.GetGraphQLAssert(ctx, id.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoGraphQLAssertFound
		}
		return nil, err
	}

	result := convertGetGraphQLAssertRowToModel(assert)
	return &result, nil
}

func (s GraphQLAssertService) GetByGraphQLID(ctx context.Context, graphqlID idwrap.IDWrap) ([]mgraphql.GraphQLAssert, error) {
	asserts, err := s.queries.GetGraphQLAssertsByGraphQLID(ctx, graphqlID.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLAssert{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLAssert, len(asserts))
	for i, a := range asserts {
		result[i] = convertGetGraphQLAssertsByGraphQLIDRowToModel(a)
	}
	return result, nil
}

func (s GraphQLAssertService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mgraphql.GraphQLAssert, error) {
	// Convert IDWraps to [][]byte
	idBytes := make([][]byte, len(ids))
	for i, id := range ids {
		idBytes[i] = id.Bytes()
	}

	asserts, err := s.queries.GetGraphQLAssertsByIDs(ctx, idBytes)
	if err != nil {
		return nil, err
	}

	result := make([]mgraphql.GraphQLAssert, len(asserts))
	for i, a := range asserts {
		result[i] = convertGetGraphQLAssertsByIDsRowToModel(a)
	}
	return result, nil
}

func (s GraphQLAssertService) Create(ctx context.Context, assert *mgraphql.GraphQLAssert) error {
	now := dbtime.DBNow()
	assert.CreatedAt = now.Unix()
	assert.UpdatedAt = now.Unix()

	params := gen.CreateGraphQLAssertParams{
		ID:           assert.ID.Bytes(),
		GraphqlID:    assert.GraphQLID.Bytes(),
		Value:        assert.Value,
		Enabled:      assert.Enabled,
		Description:  assert.Description,
		DisplayOrder: float64(assert.DisplayOrder),
		CreatedAt:    assert.CreatedAt,
		UpdatedAt:    assert.UpdatedAt,
	}

	// Handle delta fields
	if assert.ParentGraphQLAssertID != nil {
		params.ParentGraphqlAssertID = assert.ParentGraphQLAssertID.Bytes()
	}
	params.IsDelta = assert.IsDelta
	params.DeltaValue = stringPtrToNullString(assert.DeltaValue)
	params.DeltaEnabled = boolPtrToNullBool(assert.DeltaEnabled)
	params.DeltaDescription = stringPtrToNullString(assert.DeltaDescription)
	params.DeltaDisplayOrder = float32PtrToNullFloat64(assert.DeltaDisplayOrder)

	return s.queries.CreateGraphQLAssert(ctx, params)
}

func (s GraphQLAssertService) Update(ctx context.Context, assert *mgraphql.GraphQLAssert) error {
	return s.queries.UpdateGraphQLAssert(ctx, gen.UpdateGraphQLAssertParams{
		ID:           assert.ID.Bytes(),
		Value:        assert.Value,
		Enabled:      assert.Enabled,
		Description:  assert.Description,
		DisplayOrder: float64(assert.DisplayOrder),
		UpdatedAt:    dbtime.DBNow().Unix(),
	})
}

func (s GraphQLAssertService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaDisplayOrder *float32) error {
	return s.queries.UpdateGraphQLAssertDelta(ctx, gen.UpdateGraphQLAssertDeltaParams{
		ID:                id.Bytes(),
		DeltaValue:        stringPtrToNullString(deltaValue),
		DeltaEnabled:      boolPtrToNullBool(deltaEnabled),
		DeltaDescription:  stringPtrToNullString(deltaDescription),
		DeltaDisplayOrder: float32PtrToNullFloat64(deltaDisplayOrder),
		UpdatedAt:         dbtime.DBNow().Unix(),
	})
}

func (s GraphQLAssertService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.DeleteGraphQLAssert(ctx, id.Bytes())
}

// Delta methods
func (s GraphQLAssertService) GetDeltasByWorkspaceID(ctx context.Context, workspaceID idwrap.IDWrap) ([]mgraphql.GraphQLAssert, error) {
	asserts, err := s.queries.GetGraphQLAssertDeltasByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLAssert{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLAssert, len(asserts))
	for i, a := range asserts {
		result[i] = convertGetGraphQLAssertDeltasByWorkspaceIDRowToModel(a)
	}
	return result, nil
}

func (s GraphQLAssertService) GetDeltasByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mgraphql.GraphQLAssert, error) {
	asserts, err := s.queries.GetGraphQLAssertDeltasByParentID(ctx, parentID.Bytes())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mgraphql.GraphQLAssert{}, nil
		}
		return nil, err
	}

	result := make([]mgraphql.GraphQLAssert, len(asserts))
	for i, a := range asserts {
		result[i] = convertGetGraphQLAssertDeltasByParentIDRowToModel(a)
	}
	return result, nil
}

// Row conversion helpers - convert sqlc row types to model
func convertGetGraphQLAssertRowToModel(row gen.GetGraphQLAssertRow) mgraphql.GraphQLAssert {
	id, _ := idwrap.NewFromBytes(row.ID)
	graphqlID, _ := idwrap.NewFromBytes(row.GraphqlID)

	return mgraphql.GraphQLAssert{
		ID:                    id,
		GraphQLID:             graphqlID,
		Value:                 row.Value,
		Enabled:               row.Enabled,
		Description:           row.Description,
		DisplayOrder:          float32(row.DisplayOrder),
		ParentGraphQLAssertID: bytesToIDWrapPtr(row.ParentGraphqlAssertID),
		IsDelta:               row.IsDelta,
		DeltaValue:            interfaceToStringPtr(row.DeltaValue),
		DeltaEnabled:          interfaceToBoolPtr(row.DeltaEnabled),
		DeltaDescription:      interfaceToStringPtr(row.DeltaDescription),
		DeltaDisplayOrder:     interfaceToFloat32Ptr(row.DeltaDisplayOrder),
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

func convertGetGraphQLAssertsByGraphQLIDRowToModel(row gen.GetGraphQLAssertsByGraphQLIDRow) mgraphql.GraphQLAssert {
	id, _ := idwrap.NewFromBytes(row.ID)
	graphqlID, _ := idwrap.NewFromBytes(row.GraphqlID)

	return mgraphql.GraphQLAssert{
		ID:                    id,
		GraphQLID:             graphqlID,
		Value:                 row.Value,
		Enabled:               row.Enabled,
		Description:           row.Description,
		DisplayOrder:          float32(row.DisplayOrder),
		ParentGraphQLAssertID: bytesToIDWrapPtr(row.ParentGraphqlAssertID),
		IsDelta:               row.IsDelta,
		DeltaValue:            interfaceToStringPtr(row.DeltaValue),
		DeltaEnabled:          interfaceToBoolPtr(row.DeltaEnabled),
		DeltaDescription:      interfaceToStringPtr(row.DeltaDescription),
		DeltaDisplayOrder:     interfaceToFloat32Ptr(row.DeltaDisplayOrder),
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

func convertGetGraphQLAssertsByIDsRowToModel(row gen.GetGraphQLAssertsByIDsRow) mgraphql.GraphQLAssert {
	id, _ := idwrap.NewFromBytes(row.ID)
	graphqlID, _ := idwrap.NewFromBytes(row.GraphqlID)

	return mgraphql.GraphQLAssert{
		ID:                    id,
		GraphQLID:             graphqlID,
		Value:                 row.Value,
		Enabled:               row.Enabled,
		Description:           row.Description,
		DisplayOrder:          float32(row.DisplayOrder),
		ParentGraphQLAssertID: bytesToIDWrapPtr(row.ParentGraphqlAssertID),
		IsDelta:               row.IsDelta,
		DeltaValue:            interfaceToStringPtr(row.DeltaValue),
		DeltaEnabled:          interfaceToBoolPtr(row.DeltaEnabled),
		DeltaDescription:      interfaceToStringPtr(row.DeltaDescription),
		DeltaDisplayOrder:     interfaceToFloat32Ptr(row.DeltaDisplayOrder),
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

func convertGetGraphQLAssertDeltasByWorkspaceIDRowToModel(row gen.GetGraphQLAssertDeltasByWorkspaceIDRow) mgraphql.GraphQLAssert {
	id, _ := idwrap.NewFromBytes(row.ID)
	graphqlID, _ := idwrap.NewFromBytes(row.GraphqlID)

	return mgraphql.GraphQLAssert{
		ID:                    id,
		GraphQLID:             graphqlID,
		Value:                 row.Value,
		Enabled:               row.Enabled,
		Description:           row.Description,
		DisplayOrder:          float32(row.DisplayOrder),
		ParentGraphQLAssertID: bytesToIDWrapPtr(row.ParentGraphqlAssertID),
		IsDelta:               row.IsDelta,
		DeltaValue:            interfaceToStringPtr(row.DeltaValue),
		DeltaEnabled:          interfaceToBoolPtr(row.DeltaEnabled),
		DeltaDescription:      interfaceToStringPtr(row.DeltaDescription),
		DeltaDisplayOrder:     interfaceToFloat32Ptr(row.DeltaDisplayOrder),
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

func convertGetGraphQLAssertDeltasByParentIDRowToModel(row gen.GetGraphQLAssertDeltasByParentIDRow) mgraphql.GraphQLAssert {
	id, _ := idwrap.NewFromBytes(row.ID)
	graphqlID, _ := idwrap.NewFromBytes(row.GraphqlID)

	return mgraphql.GraphQLAssert{
		ID:                    id,
		GraphQLID:             graphqlID,
		Value:                 row.Value,
		Enabled:               row.Enabled,
		Description:           row.Description,
		DisplayOrder:          float32(row.DisplayOrder),
		ParentGraphQLAssertID: bytesToIDWrapPtr(row.ParentGraphqlAssertID),
		IsDelta:               row.IsDelta,
		DeltaValue:            interfaceToStringPtr(row.DeltaValue),
		DeltaEnabled:          interfaceToBoolPtr(row.DeltaEnabled),
		DeltaDescription:      interfaceToStringPtr(row.DeltaDescription),
		DeltaDisplayOrder:     interfaceToFloat32Ptr(row.DeltaDisplayOrder),
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
	}
}

// Conversion helpers
func stringPtrToNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func boolPtrToNullBool(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{Valid: false}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func float32PtrToNullFloat64(f *float32) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(*f), Valid: true}
}
