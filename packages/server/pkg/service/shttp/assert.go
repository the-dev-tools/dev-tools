//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

var ErrNoHttpAssertFound = errors.New("no http assert found")

type HttpAssertService struct {
	reader  *AssertReader
	queries *gen.Queries
}

func NewHttpAssertService(queries *gen.Queries) *HttpAssertService {
	return &HttpAssertService{
		reader:  NewAssertReaderFromQueries(queries),
		queries: queries,
	}
}

func (s *HttpAssertService) TX(tx *sql.Tx) *HttpAssertService {
	newQueries := s.queries.WithTx(tx)
	return &HttpAssertService{
		reader:  NewAssertReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s *HttpAssertService) Create(ctx context.Context, assert *mhttp.HTTPAssert) error {
	return NewAssertWriterFromQueries(s.queries).Create(ctx, assert)
}

func (s *HttpAssertService) CreateBulk(ctx context.Context, asserts []mhttp.HTTPAssert) error {
	return NewAssertWriterFromQueries(s.queries).CreateBulk(ctx, asserts)
}

func (s *HttpAssertService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPAssert, error) {
	return s.reader.GetByID(ctx, id)
}

func (s *HttpAssertService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	return s.reader.GetByHttpID(ctx, httpID)
}

func (s *HttpAssertService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	return s.reader.GetByHttpIDOrdered(ctx, httpID)
}

func (s *HttpAssertService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPAssert, error) {
	return s.reader.GetByIDs(ctx, ids)
}

func (s *HttpAssertService) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPAssert, error) {
	return s.reader.GetByHttpIDs(ctx, httpIDs)
}

func (s *HttpAssertService) Update(ctx context.Context, assert *mhttp.HTTPAssert) error {
	return NewAssertWriterFromQueries(s.queries).Update(ctx, assert)
}

func (s *HttpAssertService) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float32) error {
	return NewAssertWriterFromQueries(s.queries).UpdateOrder(ctx, id, httpID, order)
}

func (s *HttpAssertService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return NewAssertWriterFromQueries(s.queries).UpdateDelta(ctx, id, deltaValue, deltaEnabled, deltaDescription, deltaOrder)
}

func (s *HttpAssertService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return NewAssertWriterFromQueries(s.queries).Delete(ctx, id)
}

func (s *HttpAssertService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	return NewAssertWriterFromQueries(s.queries).DeleteByHttpID(ctx, httpID)
}

func (s *HttpAssertService) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	return NewAssertWriterFromQueries(s.queries).ResetDelta(ctx, id)
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