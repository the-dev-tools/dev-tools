package shttp

import (
	"context"
	"slices"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type BodyUrlEncodedWriter struct {
	queries *gen.Queries
	reader  *BodyUrlEncodedReader
}

func NewBodyUrlEncodedWriter(tx gen.DBTX) *BodyUrlEncodedWriter {
	queries := gen.New(tx)
	return &BodyUrlEncodedWriter{
		queries: queries,
		reader:  NewBodyUrlEncodedReaderFromQueries(queries),
	}
}

func NewBodyUrlEncodedWriterFromQueries(queries *gen.Queries) *BodyUrlEncodedWriter {
	return &BodyUrlEncodedWriter{
		queries: queries,
		reader:  NewBodyUrlEncodedReaderFromQueries(queries),
	}
}

func (w *BodyUrlEncodedWriter) Create(ctx context.Context, body *mhttp.HTTPBodyUrlencoded) error {
	bue := SerializeBodyUrlEncodedModelToGen(*body)
	return w.queries.CreateHTTPBodyUrlEncoded(ctx, gen.CreateHTTPBodyUrlEncodedParams(bue))
}

func (w *BodyUrlEncodedWriter) CreateBulk(ctx context.Context, bodyUrlEncodeds []mhttp.HTTPBodyUrlencoded) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyUrlEncodeds, SerializeBodyUrlEncodedModelToGen)

	for bodyUrlEncodedChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, bodyUrlEncoded := range bodyUrlEncodedChunk {
			err := w.createRaw(ctx, bodyUrlEncoded)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *BodyUrlEncodedWriter) createRaw(ctx context.Context, bue gen.HttpBodyUrlencoded) error {
	return w.queries.CreateHTTPBodyUrlEncoded(ctx, gen.CreateHTTPBodyUrlEncodedParams(bue))
}

func (w *BodyUrlEncodedWriter) Update(ctx context.Context, body *mhttp.HTTPBodyUrlencoded) error {
	return w.queries.UpdateHTTPBodyUrlEncoded(ctx, gen.UpdateHTTPBodyUrlEncodedParams{
		Key:          body.Key,
		Value:        body.Value,
		Description:  body.Description,
		Enabled:      body.Enabled,
		DisplayOrder: float64(body.DisplayOrder),
		ID:           body.ID,
	})
}

func (w *BodyUrlEncodedWriter) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return w.queries.UpdateHTTPBodyUrlEncodedDelta(ctx, gen.UpdateHTTPBodyUrlEncodedDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               id,
	})
}

func (w *BodyUrlEncodedWriter) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteHTTPBodyUrlEncoded(ctx, id)
}

func (w *BodyUrlEncodedWriter) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	bodies, err := w.reader.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, body := range bodies {
		if err := w.Delete(ctx, body.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w *BodyUrlEncodedWriter) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	// Reset delta fields by setting them to nil
	return w.UpdateDelta(ctx, id, nil, nil, nil, nil, nil)
}
