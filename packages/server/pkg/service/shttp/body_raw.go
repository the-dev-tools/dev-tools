//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"

	"github.com/the-dev-tools/dev-tools/packages/db/pkg/sqlc/gen"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/model/mhttp"
)

var ErrNoHttpBodyRawFound = errors.New("no HTTP body raw found")

type HttpBodyRawService struct {
	reader  *BodyRawReader
	queries *gen.Queries
}

func ConvertToDBHttpBodyRaw(body mhttp.HTTPBodyRaw) gen.HttpBodyRaw {
	return gen.HttpBodyRaw{
		ID:                   body.ID,
		HttpID:               body.HttpID,
		RawData:              body.RawData,
		CompressionType:      body.CompressionType,
		ParentBodyRawID:      body.ParentBodyRawID,
		IsDelta:              body.IsDelta,
		DeltaRawData:         body.DeltaRawData,
		DeltaCompressionType: body.DeltaCompressionType,
		CreatedAt:            body.CreatedAt,
		UpdatedAt:            body.UpdatedAt,
	}
}

func ConvertToModelHttpBodyRaw(dbBody gen.HttpBodyRaw) mhttp.HTTPBodyRaw {
	var deltaRawData []byte
	if dbBody.DeltaRawData != nil {
		deltaRawData = dbBody.DeltaRawData.([]byte)
	}

	return mhttp.HTTPBodyRaw{
		ID:                   dbBody.ID,
		HttpID:               dbBody.HttpID,
		RawData:              dbBody.RawData,
		CompressionType:      dbBody.CompressionType,
		ParentBodyRawID:      dbBody.ParentBodyRawID,
		IsDelta:              dbBody.IsDelta,
		DeltaRawData:         deltaRawData,
		DeltaCompressionType: dbBody.DeltaCompressionType,
		CreatedAt:            dbBody.CreatedAt,
		UpdatedAt:            dbBody.UpdatedAt,
	}
}

func NewHttpBodyRawService(queries *gen.Queries) *HttpBodyRawService {
	return &HttpBodyRawService{
		reader:  NewBodyRawReaderFromQueries(queries),
		queries: queries,
	}
}

func (s *HttpBodyRawService) TX(tx *sql.Tx) *HttpBodyRawService {
	newQueries := s.queries.WithTx(tx)
	return &HttpBodyRawService{
		reader:  NewBodyRawReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s *HttpBodyRawService) Create(ctx context.Context, httpID idwrap.IDWrap, rawData []byte) (*mhttp.HTTPBodyRaw, error) {
	return NewBodyRawWriterFromQueries(s.queries).Create(ctx, httpID, rawData)
}

func (s *HttpBodyRawService) CreateFull(ctx context.Context, body *mhttp.HTTPBodyRaw) (*mhttp.HTTPBodyRaw, error) {
	return NewBodyRawWriterFromQueries(s.queries).CreateFull(ctx, body)
}

func (s *HttpBodyRawService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	return s.reader.Get(ctx, id)
}

func (s *HttpBodyRawService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	return s.reader.GetByHttpID(ctx, httpID)
}

func (s *HttpBodyRawService) Update(ctx context.Context, id idwrap.IDWrap, rawData []byte) (*mhttp.HTTPBodyRaw, error) {
	return NewBodyRawWriterFromQueries(s.queries).Update(ctx, id, rawData)
}

func (s *HttpBodyRawService) CreateDelta(ctx context.Context, httpID idwrap.IDWrap, rawData []byte) (*mhttp.HTTPBodyRaw, error) {
	return NewBodyRawWriterFromQueries(s.queries).CreateDelta(ctx, httpID, rawData)
}

func (s *HttpBodyRawService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, rawData []byte) (*mhttp.HTTPBodyRaw, error) {
	return NewBodyRawWriterFromQueries(s.queries).UpdateDelta(ctx, id, rawData)
}

func (s *HttpBodyRawService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return NewBodyRawWriterFromQueries(s.queries).Delete(ctx, id)
}

func (s *HttpBodyRawService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	return NewBodyRawWriterFromQueries(s.queries).DeleteByHttpID(ctx, httpID)
}

func (s *HttpBodyRawService) Reader() *BodyRawReader { return s.reader }
