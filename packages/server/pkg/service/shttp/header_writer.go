package shttp

import (
	"context"
	"errors"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

type HeaderWriter struct {
	queries *gen.Queries
	reader  *HeaderReader
}

func NewHeaderWriter(tx gen.DBTX) *HeaderWriter {
	queries := gen.New(tx)
	return &HeaderWriter{
		queries: queries,
		reader:  NewHeaderReaderFromQueries(queries),
	}
}

func NewHeaderWriterFromQueries(queries *gen.Queries) *HeaderWriter {
	return &HeaderWriter{
		queries: queries,
		reader:  NewHeaderReaderFromQueries(queries),
	}
}

func (w *HeaderWriter) Create(ctx context.Context, header *mhttp.HTTPHeader) error {
	if header == nil {
		return errors.New("header cannot be nil")
	}

	now := time.Now().Unix()
	header.CreatedAt = now
	header.UpdatedAt = now

	dbHeader := SerializeHeaderModelToGen(*header)
	return w.queries.CreateHTTPHeader(ctx, gen.CreateHTTPHeaderParams(dbHeader))
}

func (w *HeaderWriter) CreateBulk(ctx context.Context, httpID idwrap.IDWrap, headers []mhttp.HTTPHeader) error {
	if len(headers) == 0 {
		return nil
	}

	for _, header := range headers {
		header.HttpID = httpID
		if err := w.Create(ctx, &header); err != nil {
			return err
		}
	}

	return nil
}

func (w *HeaderWriter) Update(ctx context.Context, header *mhttp.HTTPHeader) error {
	if header == nil {
		return errors.New("header cannot be nil")
	}

	dbHeader := SerializeHeaderModelToGen(*header)
	return w.queries.UpdateHTTPHeader(ctx, gen.UpdateHTTPHeaderParams{
		HeaderKey:    dbHeader.HeaderKey,
		HeaderValue:  dbHeader.HeaderValue,
		Description:  dbHeader.Description,
		Enabled:      dbHeader.Enabled,
		DisplayOrder: dbHeader.DisplayOrder,
		ID:           dbHeader.ID,
	})
}

func (w *HeaderWriter) UpdateDelta(ctx context.Context, headerID idwrap.IDWrap, deltaKey, deltaValue, deltaDescription *string, deltaEnabled *bool) error {
	return w.queries.UpdateHTTPHeaderDelta(ctx, gen.UpdateHTTPHeaderDeltaParams{
		DeltaHeaderKey:   deltaKey,
		DeltaHeaderValue: deltaValue,
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               headerID,
	})
}

func (w *HeaderWriter) Delete(ctx context.Context, headerID idwrap.IDWrap) error {
	return w.queries.DeleteHTTPHeader(ctx, headerID)
}

func (w *HeaderWriter) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	// Use internal reader
	headers, err := w.reader.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, header := range headers {
		if err := w.Delete(ctx, header.ID); err != nil {
			return err
		}
	}
	return nil
}

func (w *HeaderWriter) UpdateOrder(ctx context.Context, headerID idwrap.IDWrap, displayOrder float64) error {
	// Use internal reader
	header, err := w.reader.GetByID(ctx, headerID)
	if err != nil {
		return err
	}

	return w.queries.UpdateHTTPHeaderOrder(ctx, gen.UpdateHTTPHeaderOrderParams{
		DisplayOrder: displayOrder,
		ID:           headerID,
		HttpID:       header.HttpID,
	})
}
