package shttp

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/dbtime"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

type BodyRawWriter struct {
	queries *gen.Queries
	reader  *BodyRawReader
}

func NewBodyRawWriter(tx gen.DBTX) *BodyRawWriter {
	queries := gen.New(tx)
	return &BodyRawWriter{
		queries: queries,
		reader:  NewBodyRawReaderFromQueries(queries),
	}
}

func NewBodyRawWriterFromQueries(queries *gen.Queries) *BodyRawWriter {
	return &BodyRawWriter{
		queries: queries,
		reader:  NewBodyRawReaderFromQueries(queries),
	}
}

func (w *BodyRawWriter) Create(ctx context.Context, httpID idwrap.IDWrap, rawData []byte) (*mhttp.HTTPBodyRaw, error) {
	now := dbtime.DBNow().Unix()
	id := idwrap.NewNow()
	err := w.queries.CreateHTTPBodyRaw(ctx, gen.CreateHTTPBodyRawParams{
		ID:                   id,
		HttpID:               httpID,
		RawData:              rawData,
		CompressionType:      0,
		ParentBodyRawID:      nil,
		IsDelta:              false,
		DeltaRawData:         nil,
		DeltaCompressionType: nil,
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	if err != nil {
		return nil, err
	}

	return w.reader.Get(ctx, id)
}

func (w *BodyRawWriter) CreateFull(ctx context.Context, body *mhttp.HTTPBodyRaw) (*mhttp.HTTPBodyRaw, error) {
	now := dbtime.DBNow().Unix()

	id := body.ID
	if id == (idwrap.IDWrap{}) {
		id = idwrap.NewNow()
	}

	err := w.queries.CreateHTTPBodyRaw(ctx, gen.CreateHTTPBodyRawParams{
		ID:                   id,
		HttpID:               body.HttpID,
		RawData:              body.RawData,
		CompressionType:      body.CompressionType,
		ParentBodyRawID:      body.ParentBodyRawID,
		IsDelta:              body.IsDelta,
		DeltaRawData:         body.DeltaRawData,
		DeltaCompressionType: body.DeltaCompressionType,
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	if err != nil {
		return nil, err
	}

	return w.reader.Get(ctx, id)
}

func (w *BodyRawWriter) Update(ctx context.Context, id idwrap.IDWrap, rawData []byte) (*mhttp.HTTPBodyRaw, error) {
	now := dbtime.DBNow().Unix()
	err := w.queries.UpdateHTTPBodyRaw(ctx, gen.UpdateHTTPBodyRawParams{
		RawData:         rawData,
		CompressionType: 0,
		UpdatedAt:       now,
		ID:              id,
	})
	if err != nil {
		return nil, err
	}

	return w.reader.Get(ctx, id)
}

func (w *BodyRawWriter) CreateDelta(ctx context.Context, httpID idwrap.IDWrap, rawData []byte) (*mhttp.HTTPBodyRaw, error) {
	// Need a transactional reader check here
	httpEntry, err := w.queries.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	if !httpEntry.IsDelta || httpEntry.ParentHttpID == nil {
		return nil, errors.New("cannot create delta body for non-delta HTTP request")
	}

	parentHttpID := httpEntry.ParentHttpID
	parentBody, err := w.queries.GetHTTPBodyRaw(ctx, *parentHttpID)

	var parentBodyID *idwrap.IDWrap
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			now := dbtime.DBNow().Unix()
			newParentID := idwrap.NewNow()
			err = w.queries.CreateHTTPBodyRaw(ctx, gen.CreateHTTPBodyRawParams{
				ID:              newParentID,
				HttpID:          *parentHttpID,
				RawData:         []byte{},
				CompressionType: 0,
				CreatedAt:       now,
				UpdatedAt:       now,
			})
			if err != nil {
				return nil, err
			}
			parentBodyID = &newParentID
		} else {
			return nil, err
		}
	} else {
		id := parentBody.ID
		parentBodyID = &id
	}

	now := dbtime.DBNow().Unix()
	id := idwrap.NewNow()
	err = w.queries.CreateHTTPBodyRaw(ctx, gen.CreateHTTPBodyRawParams{
		ID:                   id,
		HttpID:               httpID,
		RawData:              nil,
		CompressionType:      0,
		ParentBodyRawID:      parentBodyID,
		IsDelta:              true,
		DeltaRawData:         rawData,
		DeltaCompressionType: nil,
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	if err != nil {
		return nil, err
	}

	return w.reader.Get(ctx, id)
}

func (w *BodyRawWriter) UpdateDelta(ctx context.Context, id idwrap.IDWrap, rawData []byte) (*mhttp.HTTPBodyRaw, error) {
	now := dbtime.DBNow().Unix()
	err := w.queries.UpdateHTTPBodyRawDelta(ctx, gen.UpdateHTTPBodyRawDeltaParams{
		DeltaRawData: rawData,
		UpdatedAt:    now,
		ID:           id,
	})
	if err != nil {
		return nil, err
	}

	return w.reader.Get(ctx, id)
}

func (w *BodyRawWriter) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return w.queries.DeleteHTTPBodyRaw(ctx, id)
}

func (w *BodyRawWriter) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	bodyRaw, err := w.queries.GetHTTPBodyRaw(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}

	return w.Delete(ctx, bodyRaw.ID)
}
