package shttpassert

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttpassert"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"time"
)

// Utility functions for null handling
func stringToNull(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func stringToNullPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullToString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func float32ToNull(f *float32) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(*f), Valid: true}
}

func nullToFloat32(nf sql.NullFloat64) *float32 {
	if !nf.Valid {
		return nil
	}
	f := float32(nf.Float64)
	return &f
}

func idWrapToBytes(id *idwrap.IDWrap) []byte {
	if id == nil {
		return nil
	}
	return id.Bytes()
}

func bytesToIDWrap(b []byte) *idwrap.IDWrap {
	if b == nil {
		return nil
	}
	id, err := idwrap.NewFromBytes(b)
	if err != nil {
		return nil
	}
	return &id
}

type HttpAssertService struct {
	queries *gen.Queries
}

var ErrNoHttpAssertFound = errors.New("no http assert found")

func SerializeModelToGen(assert mhttpassert.HttpAssert) gen.HttpAssert {
	return gen.HttpAssert{
		ID:                 assert.ID,
		HttpID:             assert.HttpID,
		Key:                assert.Key,
		Value:              assert.Value,
		Enabled:            assert.Enabled,
		Description:        assert.Description,
		Order:              float64(assert.Order),
		ParentHttpAssertID: idWrapToBytes(assert.ParentHttpAssertID),
		IsDelta:            assert.IsDelta,
		DeltaKey:           stringToNull(assert.DeltaKey),
		DeltaValue:         stringToNull(assert.DeltaValue),
		DeltaEnabled:       assert.DeltaEnabled,
		DeltaDescription:   stringToNull(assert.DeltaDescription),
		DeltaOrder:         float32ToNull(assert.DeltaOrder),
		CreatedAt:          assert.CreatedAt,
		UpdatedAt:          assert.UpdatedAt,
	}
}

func DeserializeGenToModel(assert gen.HttpAssert) mhttpassert.HttpAssert {
	return mhttpassert.HttpAssert{
		ID:                 assert.ID,
		HttpID:             assert.HttpID,
		Key:                assert.Key,
		Value:              assert.Value,
		Enabled:            assert.Enabled,
		Description:        assert.Description,
		Order:              float32(assert.Order),
		ParentHttpAssertID: bytesToIDWrap(assert.ParentHttpAssertID),
		IsDelta:            assert.IsDelta,
		DeltaKey:           nullToString(assert.DeltaKey),
		DeltaValue:         nullToString(assert.DeltaValue),
		DeltaEnabled:       assert.DeltaEnabled,
		DeltaDescription:   nullToString(assert.DeltaDescription),
		DeltaOrder:         nullToFloat32(assert.DeltaOrder),
		CreatedAt:          assert.CreatedAt,
		UpdatedAt:          assert.UpdatedAt,
	}
}

func New(queries *gen.Queries) HttpAssertService {
	return HttpAssertService{queries: queries}
}

func (has HttpAssertService) TX(tx *sql.Tx) HttpAssertService {
	return HttpAssertService{queries: has.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*HttpAssertService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := HttpAssertService{queries: queries}
	return &service, nil
}

func (has HttpAssertService) GetHttpAssert(ctx context.Context, id idwrap.IDWrap) (*mhttpassert.HttpAssert, error) {
	assert, err := has.queries.GetHTTPAssert(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoHttpAssertFound
		}
		return nil, err
	}

	model := DeserializeGenToModel(assert)
	return &model, nil
}

func (has HttpAssertService) GetHttpAssertsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttpassert.HttpAssert, error) {
	rows, err := has.queries.GetHTTPAssertsByHttpID(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttpassert.HttpAssert{}, nil
		}
		return nil, err
	}

	result := make([]mhttpassert.HttpAssert, len(rows))
	for i, row := range rows {
		result[i] = DeserializeGenToModel(row)
	}
	return result, nil
}

func (has HttpAssertService) GetHttpAssertsByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttpassert.HttpAssert, error) {
	result := make(map[idwrap.IDWrap][]mhttpassert.HttpAssert, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	// For each HttpID, get the asserts and group them
	for _, httpID := range httpIDs {
		asserts, err := has.GetHttpAssertsByHttpID(ctx, httpID)
		if err != nil {
			return nil, err
		}
		if len(asserts) > 0 {
			result[httpID] = asserts
		}
	}

	return result, nil
}

func (has HttpAssertService) CreateHttpAssert(ctx context.Context, assert *mhttpassert.HttpAssert) error {
	af := SerializeModelToGen(*assert)
	return has.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams{
		ID:                 af.ID,
		HttpID:             af.HttpID,
		Key:                af.Key,
		Value:              af.Value,
		Description:        af.Description,
		Enabled:            af.Enabled,
		Order:              af.Order,
		ParentHttpAssertID: af.ParentHttpAssertID,
		IsDelta:            af.IsDelta,
		DeltaKey:           af.DeltaKey,
		DeltaValue:         af.DeltaValue,
		DeltaEnabled:       af.DeltaEnabled,
		DeltaDescription:   af.DeltaDescription,
		DeltaOrder:         af.DeltaOrder,
		CreatedAt:          af.CreatedAt,
		UpdatedAt:          af.UpdatedAt,
	})
}

func (has HttpAssertService) CreateBulkHttpAssert(ctx context.Context, asserts []mhttpassert.HttpAssert) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(asserts, SerializeModelToGen)

	for assertChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, assert := range assertChunk {
			err := has.CreateHttpAssertRaw(ctx, assert)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (has HttpAssertService) CreateHttpAssertRaw(ctx context.Context, af gen.HttpAssert) error {
	return has.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams{
		ID:                 af.ID,
		HttpID:             af.HttpID,
		Key:                af.Key,
		Value:              af.Value,
		Description:        af.Description,
		Enabled:            af.Enabled,
		Order:              af.Order,
		ParentHttpAssertID: af.ParentHttpAssertID,
		IsDelta:            af.IsDelta,
		DeltaKey:           af.DeltaKey,
		DeltaValue:         af.DeltaValue,
		DeltaEnabled:       af.DeltaEnabled,
		DeltaDescription:   af.DeltaDescription,
		DeltaOrder:         af.DeltaOrder,
		CreatedAt:          af.CreatedAt,
		UpdatedAt:          af.UpdatedAt,
	})
}

func (has HttpAssertService) UpdateHttpAssert(ctx context.Context, assert *mhttpassert.HttpAssert) error {
	// First check if the assert exists
	_, err := has.GetHttpAssert(ctx, assert.ID)
	if err != nil {
		return err
	}

	// Get the current assert to preserve fields not being updated
	currentAssert, err := has.GetHttpAssert(ctx, assert.ID)
	if err != nil {
		return err
	}

	// Update with all fields, preserving delta fields from current assert
	return has.queries.UpdateHTTPAssert(ctx, gen.UpdateHTTPAssertParams{
		Key:              assert.Key,
		Value:            assert.Value,
		Description:      assert.Description,
		Enabled:          assert.Enabled,
		Order:            float64(currentAssert.Order), // Preserve current order
		DeltaKey:         stringToNull(currentAssert.DeltaKey),
		DeltaValue:       stringToNull(currentAssert.DeltaValue),
		DeltaEnabled:     currentAssert.DeltaEnabled,
		DeltaDescription: stringToNull(currentAssert.DeltaDescription),
		DeltaOrder:       float32ToNull(currentAssert.DeltaOrder),
		UpdatedAt:        time.Now().Unix(),
		ID:               assert.ID,
	})
}

func (has HttpAssertService) UpdateHttpAssertOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float32) error {
	// First check if the assert exists
	assert, err := has.GetHttpAssert(ctx, id)
	if err != nil {
		return err
	}

	// Update the order using the general UpdateHTTPAssert query
	assert.Order = order
	assert.UpdatedAt = time.Now().Unix()

	return has.queries.UpdateHTTPAssert(ctx, gen.UpdateHTTPAssertParams{
		Key:              assert.Key,
		Value:            assert.Value,
		Description:      assert.Description,
		Enabled:          assert.Enabled,
		Order:            float64(assert.Order),
		DeltaKey:         stringToNull(assert.DeltaKey),
		DeltaValue:       stringToNull(assert.DeltaValue),
		DeltaEnabled:     assert.DeltaEnabled,
		DeltaDescription: stringToNull(assert.DeltaDescription),
		DeltaOrder:       float32ToNull(assert.DeltaOrder),
		UpdatedAt:        assert.UpdatedAt,
		ID:               assert.ID,
	})
}

func (has HttpAssertService) UpdateHttpAssertDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return has.queries.UpdateHTTPAssertDelta(ctx, gen.UpdateHTTPAssertDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: stringToNull(deltaDescription),
		DeltaEnabled:     deltaEnabled,
		DeltaOrder:       float32ToNull(deltaOrder),
		ID:               id,
	})
}

func (has HttpAssertService) DeleteHttpAssert(ctx context.Context, id idwrap.IDWrap) error {
	// First check if the assert exists
	_, err := has.GetHttpAssert(ctx, id)
	if err != nil {
		return err
	}

	return has.queries.DeleteHTTPAssert(ctx, id)
}

func (has HttpAssertService) ResetHttpAssertDelta(ctx context.Context, id idwrap.IDWrap) error {
	assert, err := has.GetHttpAssert(ctx, id)
	if err != nil {
		return err
	}

	// Reset all delta fields
	assert.ParentHttpAssertID = nil
	assert.IsDelta = false
	assert.DeltaKey = nil
	assert.DeltaValue = nil
	assert.DeltaEnabled = nil
	assert.DeltaDescription = nil
	assert.DeltaOrder = nil
	assert.UpdatedAt = time.Now().Unix()

	// Since UpdateHTTPAssert doesn't include is_delta field, we need to delete and recreate
	// First delete the existing assert
	err = has.queries.DeleteHTTPAssert(ctx, id)
	if err != nil {
		return err
	}

	// Then recreate it with IsDelta = false
	return has.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams{
		ID:                 assert.ID,
		HttpID:             assert.HttpID,
		Key:                assert.Key,
		Value:              assert.Value,
		Enabled:            assert.Enabled,
		Description:        assert.Description,
		Order:              float64(assert.Order),
		ParentHttpAssertID: idWrapToBytes(assert.ParentHttpAssertID),
		IsDelta:            assert.IsDelta,
		DeltaKey:           stringToNull(assert.DeltaKey),
		DeltaValue:         stringToNull(assert.DeltaValue),
		DeltaEnabled:       assert.DeltaEnabled,
		DeltaDescription:   stringToNull(assert.DeltaDescription),
		DeltaOrder:         float32ToNull(assert.DeltaOrder),
		CreatedAt:          assert.CreatedAt,
		UpdatedAt:          assert.UpdatedAt,
	})
}

func (has HttpAssertService) GetHttpAssertsByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mhttpassert.HttpAssert, error) {
	// Note: There's no specific query for getting by parent ID in the generated code
	// This would need to be implemented using GetHTTPAsserts and filtering
	// For now, return empty slice as this is a specialized query
	return []mhttpassert.HttpAssert{}, nil
}

func (has HttpAssertService) GetHttpAssertStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]mhttpassert.HttpAssert, error) {
	// Note: GetHTTPAssertStreaming query doesn't exist in generated code
	// This would need to be implemented using GetHTTPAssertsByIDs and filtering
	// For now, return empty slice as this is a specialized query
	return []mhttpassert.HttpAssert{}, nil
}
