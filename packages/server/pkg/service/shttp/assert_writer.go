package shttp

import (
	"context"
	"slices"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type AssertWriter struct {
	queries *gen.Queries
	reader  *AssertReader
}

func NewAssertWriter(tx gen.DBTX) *AssertWriter {
	queries := gen.New(tx)
	return &AssertWriter{
		queries: queries,
		reader:  NewAssertReaderFromQueries(queries),
	}
}

func NewAssertWriterFromQueries(queries *gen.Queries) *AssertWriter {
	return &AssertWriter{
		queries: queries,
		reader:  NewAssertReaderFromQueries(queries),
	}
}

func (w *AssertWriter) Create(ctx context.Context, assert *mhttp.HTTPAssert) error {
	af := SerializeAssertModelToGen(*assert)
	return w.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams(af))
}

func (w *AssertWriter) CreateBulk(ctx context.Context, asserts []mhttp.HTTPAssert) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(asserts, SerializeAssertModelToGen)

	for assertChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, assert := range assertChunk {
			err := w.createRaw(ctx, assert)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *AssertWriter) createRaw(ctx context.Context, af gen.HttpAssert) error {
	return w.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams(af))
}

func (w *AssertWriter) Update(ctx context.Context, assert *mhttp.HTTPAssert) error {
	currentAssert, err := w.reader.GetByID(ctx, assert.ID)
	if err != nil {
		return err
	}

	return w.queries.UpdateHTTPAssert(ctx, gen.UpdateHTTPAssertParams{
		Value:             assert.Value,
		Description:       assert.Description,
		Enabled:           assert.Enabled,
		DisplayOrder:      float64(assert.DisplayOrder),
		DeltaValue:        stringToNull(currentAssert.DeltaValue),
		DeltaEnabled:      currentAssert.DeltaEnabled,
		DeltaDescription:  stringToNull(currentAssert.DeltaDescription),
		DeltaDisplayOrder: float32ToNullFloat64Assert(currentAssert.DeltaDisplayOrder),
		UpdatedAt:         time.Now().Unix(),
		ID:                assert.ID,
	})
}

func (w *AssertWriter) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float32) error {
	assert, err := w.reader.GetByID(ctx, id)
	if err != nil {
		return err
	}

	return w.queries.UpdateHTTPAssert(ctx, gen.UpdateHTTPAssertParams{
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

func (w *AssertWriter) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return w.queries.UpdateHTTPAssertDelta(ctx, gen.UpdateHTTPAssertDeltaParams{
		DeltaValue:        stringToNull(deltaValue),
		DeltaDescription:  stringToNull(deltaDescription),
		DeltaEnabled:      deltaEnabled,
		DeltaDisplayOrder: float32ToNullFloat64Assert(deltaOrder),
		ID:                id,
	})
}

func (w *AssertWriter) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteHTTPAssert(ctx, id)
}

func (w *AssertWriter) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	asserts, err := w.reader.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, assert := range asserts {
		if err := w.Delete(ctx, assert.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w *AssertWriter) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	assert, err := w.reader.GetByID(ctx, id)
	if err != nil {
		return err
	}

	assert.ParentHttpAssertID = nil
	assert.IsDelta = false
	assert.DeltaValue = nil
	assert.DeltaEnabled = nil
	assert.DeltaDescription = nil
	assert.DeltaDisplayOrder = nil
	assert.UpdatedAt = time.Now().Unix()

	err = w.queries.DeleteHTTPAssert(ctx, id)
	if err != nil {
		return err
	}

	return w.queries.CreateHTTPAssert(ctx, gen.CreateHTTPAssertParams{
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
