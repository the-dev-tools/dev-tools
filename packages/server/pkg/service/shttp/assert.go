//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHttpAssertFound = errors.New("no http assert found")

type HttpAssertService struct {
	queries *gen.Queries
}

func NewHttpAssertService(queries *gen.Queries) *HttpAssertService {
	return &HttpAssertService{queries: queries}
}

func (s *HttpAssertService) TX(tx *sql.Tx) *HttpAssertService {
	return &HttpAssertService{queries: s.queries.WithTx(tx)}
}

func (s *HttpAssertService) Create(ctx context.Context, assert *mhttp.HTTPAssert) error {
	af := SerializeAssertModelToGen(*assert)
	return s.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams(af))
}

func (s *HttpAssertService) CreateBulk(ctx context.Context, asserts []mhttp.HTTPAssert) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(asserts, SerializeAssertModelToGen)

	for assertChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, assert := range assertChunk {
			err := s.createRaw(ctx, assert)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *HttpAssertService) createRaw(ctx context.Context, af gen.HttpAssert) error {
	return s.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams(af))
}

func (s *HttpAssertService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPAssert, error) {
	assert, err := s.queries.GetHTTPAssert(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHttpAssertFound
		}
		return nil, err
	}

	model := DeserializeAssertGenToModel(assert)
	return &model, nil
}

func (s *HttpAssertService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	rows, err := s.queries.GetHTTPAssertsByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPAssert{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPAssert, len(rows))
	for i, row := range rows {
		result[i] = DeserializeAssertGenToModel(row)
	}
	return result, nil
}

func (s *HttpAssertService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	rows, err := s.queries.GetHTTPAssertsByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPAssert{}, nil
		}
		return nil, err
	}

	// Sort by order field
	slices.SortFunc(rows, func(a, b gen.HttpAssert) int {
		if a.DisplayOrder < b.DisplayOrder {
			return -1
		}
		if a.DisplayOrder > b.DisplayOrder {
			return 1
		}
		return 0
	})

	result := make([]mhttp.HTTPAssert, len(rows))
	for i, row := range rows {
		result[i] = DeserializeAssertGenToModel(row)
	}
	return result, nil
}

func (s *HttpAssertService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPAssert{}, nil
	}

	rows, err := s.queries.GetHTTPAssertsByIDs(ctx, ids)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPAssert{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPAssert, len(rows))
	for i, row := range rows {
		result[i] = DeserializeAssertGenToModel(row)
	}
	return result, nil
}

func (s *HttpAssertService) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPAssert, error) {
	result := make(map[idwrap.IDWrap][]mhttp.HTTPAssert, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	// For each HttpID, get the asserts and group them
	for _, httpID := range httpIDs {
		asserts, err := s.GetByHttpID(ctx, httpID)
		if err != nil {
			return nil, err
		}
		if len(asserts) > 0 {
			result[httpID] = asserts
		}
	}

	return result, nil
}

func (s *HttpAssertService) Update(ctx context.Context, assert *mhttp.HTTPAssert) error {
	// Get current assert to preserve delta fields
	currentAssert, err := s.GetByID(ctx, assert.ID)
	if err != nil {
		return err
	}

	return s.queries.UpdateHTTPAssert(ctx, gen.UpdateHTTPAssertParams{
		Value:             assert.Value,
		Description:       assert.Description,
		Enabled:           assert.Enabled,
		DisplayOrder:      float64(currentAssert.DisplayOrder),
		DeltaValue:        stringToNull(currentAssert.DeltaValue),
		DeltaEnabled:      currentAssert.DeltaEnabled,
		DeltaDescription:  stringToNull(currentAssert.DeltaDescription),
		DeltaDisplayOrder: float32ToNullFloat64Assert(currentAssert.DeltaDisplayOrder),
		UpdatedAt:         time.Now().Unix(),
		ID:                assert.ID,
	})
}

func (s *HttpAssertService) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float32) error {
	assert, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return s.queries.UpdateHTTPAssert(ctx, gen.UpdateHTTPAssertParams{
		Value:             assert.Value,
		Description:       assert.Description,
		Enabled:           assert.Enabled,
		DisplayOrder:      float64(order),
		DeltaValue:        stringToNull(assert.DeltaValue),
		DeltaEnabled:      assert.DeltaEnabled,
		DeltaDescription:  stringToNull(assert.DeltaDescription),
		DeltaDisplayOrder: float32ToNullFloat64Assert(assert.DeltaDisplayOrder),
		UpdatedAt:         time.Now().Unix(),
		ID:                assert.ID,
	})
}

func (s *HttpAssertService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return s.queries.UpdateHTTPAssertDelta(ctx, gen.UpdateHTTPAssertDeltaParams{
		DeltaValue:        stringToNull(deltaValue),
		DeltaDescription:  stringToNull(deltaDescription),
		DeltaEnabled:      deltaEnabled,
		DeltaDisplayOrder: float32ToNullFloat64Assert(deltaOrder),
		ID:                id,
	})
}

func (s *HttpAssertService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.DeleteHTTPAssert(ctx, id)
}

func (s *HttpAssertService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	asserts, err := s.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, assert := range asserts {
		if err := s.Delete(ctx, assert.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *HttpAssertService) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	assert, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Reset all delta fields
	assert.ParentHttpAssertID = nil
	assert.IsDelta = false
	assert.DeltaValue = nil
	assert.DeltaEnabled = nil
	assert.DeltaDescription = nil
	assert.DeltaDisplayOrder = nil
	assert.UpdatedAt = time.Now().Unix()

	// Delete and recreate since UpdateHTTPAssert doesn't include is_delta field
	err = s.queries.DeleteHTTPAssert(ctx, id)
	if err != nil {
		return err
	}

	return s.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams{
		ID:                 assert.ID,
		HttpID:             assert.HttpID,
		Value:              assert.Value,
		Enabled:            assert.Enabled,
		Description:        assert.Description,
		DisplayOrder:       float64(assert.DisplayOrder),
		ParentHttpAssertID: idWrapToBytes(assert.ParentHttpAssertID),
		IsDelta:            assert.IsDelta,
		DeltaValue:         stringToNull(assert.DeltaValue),
		DeltaEnabled:       assert.DeltaEnabled,
		DeltaDescription:   stringToNull(assert.DeltaDescription),
		DeltaDisplayOrder:  float32ToNullFloat64Assert(assert.DeltaDisplayOrder),
		CreatedAt:          assert.CreatedAt,
		UpdatedAt:          assert.UpdatedAt,
	})
}

// Note: GetStreaming method is not available - no GetHTTPAssertStreaming query exists

// Conversion functions

func float32ToNullFloat64Assert(f *float32) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(*f), Valid: true}
}

func nullFloat64ToFloat32Assert(nf sql.NullFloat64) *float32 {
	if !nf.Valid {
		return nil
	}
	f := float32(nf.Float64)
	return &f
}

func SerializeAssertModelToGen(assert mhttp.HTTPAssert) gen.HttpAssert {
	return gen.HttpAssert{
		ID:                 assert.ID,
		HttpID:             assert.HttpID,
		Value:              assert.Value,
		Enabled:            assert.Enabled,
		Description:        assert.Description,
		DisplayOrder:       float64(assert.DisplayOrder),
		ParentHttpAssertID: idWrapToBytes(assert.ParentHttpAssertID),
		IsDelta:            assert.IsDelta,
		DeltaValue:         stringToNull(assert.DeltaValue),
		DeltaEnabled:       assert.DeltaEnabled,
		DeltaDescription:   stringToNull(assert.DeltaDescription),
		DeltaDisplayOrder:  float32ToNullFloat64Assert(assert.DeltaDisplayOrder),
		CreatedAt:          assert.CreatedAt,
		UpdatedAt:          assert.UpdatedAt,
	}
}

func DeserializeAssertGenToModel(assert gen.HttpAssert) mhttp.HTTPAssert {
	return mhttp.HTTPAssert{
		ID:                 assert.ID,
		HttpID:             assert.HttpID,
		Value:              assert.Value,
		Enabled:            assert.Enabled,
		Description:        assert.Description,
		DisplayOrder:       float32(assert.DisplayOrder),
		ParentHttpAssertID: bytesToIDWrap(assert.ParentHttpAssertID),
		IsDelta:            assert.IsDelta,
		DeltaValue:         nullToString(assert.DeltaValue),
		DeltaEnabled:       assert.DeltaEnabled,
		DeltaDescription:   nullToString(assert.DeltaDescription),
		DeltaDisplayOrder:  nullFloat64ToFloat32Assert(assert.DeltaDisplayOrder),
		CreatedAt:          assert.CreatedAt,
		UpdatedAt:          assert.UpdatedAt,
	}
}
