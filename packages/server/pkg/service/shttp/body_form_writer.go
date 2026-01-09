package shttp

import (
	"context"
	"slices"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/translate/tgeneric"
)

type BodyFormWriter struct {
	queries *gen.Queries
	reader  *BodyFormReader
}

func NewBodyFormWriter(tx gen.DBTX) *BodyFormWriter {
	queries := gen.New(tx)
	return &BodyFormWriter{
		queries: queries,
		reader:  NewBodyFormReaderFromQueries(queries),
	}
}

func NewBodyFormWriterFromQueries(queries *gen.Queries) *BodyFormWriter {
	return &BodyFormWriter{
		queries: queries,
		reader:  NewBodyFormReaderFromQueries(queries),
	}
}

func (w *BodyFormWriter) Create(ctx context.Context, body *mhttp.HTTPBodyForm) error {
	bf := SerializeBodyFormModelToGen(*body)
	return w.queries.CreateHTTPBodyForm(ctx, gen.CreateHTTPBodyFormParams{
		ID:                   bf.ID,
		HttpID:               bf.HttpID,
		Key:                  bf.Key,
		Value:                bf.Value,
		Description:          bf.Description,
		Enabled:              bf.Enabled,
		DisplayOrder:         bf.DisplayOrder,
		ParentHttpBodyFormID: bf.ParentHttpBodyFormID,
		IsDelta:              bf.IsDelta,
		DeltaKey:             bf.DeltaKey,
		DeltaValue:           bf.DeltaValue,
		DeltaDescription:     bf.DeltaDescription,
		DeltaEnabled:         bf.DeltaEnabled,
		DeltaDisplayOrder:    bf.DeltaDisplayOrder,
		CreatedAt:            bf.CreatedAt,
		UpdatedAt:            bf.UpdatedAt,
	})
}

func (w *BodyFormWriter) CreateBulk(ctx context.Context, bodyForms []mhttp.HTTPBodyForm) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyForms, SerializeBodyFormModelToGen)

	for bodyFormChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, bodyForm := range bodyFormChunk {
			err := w.createRaw(ctx, bodyForm)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *BodyFormWriter) createRaw(ctx context.Context, bf gen.HttpBodyForm) error {
	return w.queries.CreateHTTPBodyForm(ctx, gen.CreateHTTPBodyFormParams{
		ID:                   bf.ID,
		HttpID:               bf.HttpID,
		Key:                  bf.Key,
		Value:                bf.Value,
		Description:          bf.Description,
		Enabled:              bf.Enabled,
		DisplayOrder:         bf.DisplayOrder,
		ParentHttpBodyFormID: bf.ParentHttpBodyFormID,
		IsDelta:              bf.IsDelta,
		DeltaKey:             bf.DeltaKey,
		DeltaValue:           bf.DeltaValue,
		DeltaDescription:     bf.DeltaDescription,
		DeltaEnabled:         bf.DeltaEnabled,
		DeltaDisplayOrder:    bf.DeltaDisplayOrder,
		CreatedAt:            bf.CreatedAt,
		UpdatedAt:            bf.UpdatedAt,
	})
}

func (w *BodyFormWriter) Update(ctx context.Context, body *mhttp.HTTPBodyForm) error {
	return w.queries.UpdateHTTPBodyForm(ctx, gen.UpdateHTTPBodyFormParams{
		Key:          body.Key,
		Value:        body.Value,
		Description:  body.Description,
		Enabled:      body.Enabled,
		DisplayOrder: float64(body.DisplayOrder),
		ID:           body.ID,
	})
}

func (w *BodyFormWriter) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float32) error {
	return w.queries.UpdateHTTPBodyFormOrder(ctx, gen.UpdateHTTPBodyFormOrderParams{
		DisplayOrder: float64(order),
		ID:           id,
		HttpID:       httpID,
	})
}

func (w *BodyFormWriter) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return w.queries.UpdateHTTPBodyFormDelta(ctx, gen.UpdateHTTPBodyFormDeltaParams{
		DeltaKey:          stringToNull(deltaKey),
		DeltaValue:        stringToNull(deltaValue),
		DeltaDescription:  deltaDescription,
		DeltaEnabled:      deltaEnabled,
		DeltaDisplayOrder: float32ToNullFloat64(deltaOrder),
		ID:                id,
	})
}

func (w *BodyFormWriter) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteHTTPBodyForm(ctx, id)
}

func (w *BodyFormWriter) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	forms, err := w.reader.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, form := range forms {
		if err := w.Delete(ctx, form.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w *BodyFormWriter) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.ResetHTTPBodyFormDelta(ctx, id)
}
