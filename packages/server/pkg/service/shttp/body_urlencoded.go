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

var ErrNoHttpBodyUrlEncodedFound = errors.New("no http body url encoded found")

type HttpBodyUrlEncodedService struct {
	reader  *BodyUrlEncodedReader
	queries *gen.Queries
}

func NewHttpBodyUrlEncodedService(queries *gen.Queries) *HttpBodyUrlEncodedService {
	return &HttpBodyUrlEncodedService{
		reader:  NewBodyUrlEncodedReaderFromQueries(queries),
		queries: queries,
	}
}

func (s *HttpBodyUrlEncodedService) TX(tx *sql.Tx) *HttpBodyUrlEncodedService {
	newQueries := s.queries.WithTx(tx)
	return &HttpBodyUrlEncodedService{
		reader:  NewBodyUrlEncodedReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s *HttpBodyUrlEncodedService) Create(ctx context.Context, body *mhttp.HTTPBodyUrlencoded) error {
	return NewBodyUrlEncodedWriterFromQueries(s.queries).Create(ctx, body)
}

func (s *HttpBodyUrlEncodedService) CreateBulk(ctx context.Context, bodyUrlEncodeds []mhttp.HTTPBodyUrlencoded) error {
	return NewBodyUrlEncodedWriterFromQueries(s.queries).CreateBulk(ctx, bodyUrlEncodeds)
}

func (s *HttpBodyUrlEncodedService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyUrlencoded, error) {
	return s.reader.GetByID(ctx, id)
}

func (s *HttpBodyUrlEncodedService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	return s.reader.GetByHttpID(ctx, httpID)
}

func (s *HttpBodyUrlEncodedService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	return s.reader.GetByHttpIDOrdered(ctx, httpID)
}

func (s *HttpBodyUrlEncodedService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	return s.reader.GetByIDs(ctx, ids)
}

func (s *HttpBodyUrlEncodedService) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded, error) {
	return s.reader.GetByHttpIDs(ctx, httpIDs)
}

func (s *HttpBodyUrlEncodedService) Update(ctx context.Context, body *mhttp.HTTPBodyUrlencoded) error {
	return NewBodyUrlEncodedWriterFromQueries(s.queries).Update(ctx, body)
}

func (s *HttpBodyUrlEncodedService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return NewBodyUrlEncodedWriterFromQueries(s.queries).UpdateDelta(ctx, id, deltaKey, deltaValue, deltaEnabled, deltaDescription, deltaOrder)
}

func (s *HttpBodyUrlEncodedService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return NewBodyUrlEncodedWriterFromQueries(s.queries).Delete(ctx, id)
}

func (s *HttpBodyUrlEncodedService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	return NewBodyUrlEncodedWriterFromQueries(s.queries).DeleteByHttpID(ctx, httpID)
}

func (s *HttpBodyUrlEncodedService) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	return NewBodyUrlEncodedWriterFromQueries(s.queries).ResetDelta(ctx, id)
}

// Note: GetStreaming is not available for HTTPBodyUrlEncoded
// Streaming queries would need to be added to the SQL schema if needed

// Conversion functions

func float32ToNullFloat64UrlEncoded(f *float32) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(*f), Valid: true}
}

func nullFloat64ToFloat32UrlEncoded(nf sql.NullFloat64) *float32 {
	if !nf.Valid {
		return nil
	}
	f := float32(nf.Float64)
	return &f
}

func SerializeBodyUrlEncodedModelToGen(body mhttp.HTTPBodyUrlencoded) gen.HttpBodyUrlencoded {
	return gen.HttpBodyUrlencoded{
		ID:                         body.ID,
		HttpID:                     body.HttpID,
		Key:                        body.Key,
		Value:                      body.Value,
		Enabled:                    body.Enabled,
		Description:                body.Description,
		DisplayOrder:               float64(body.DisplayOrder),
		ParentHttpBodyUrlencodedID: idWrapToBytes(body.ParentHttpBodyUrlEncodedID),
		IsDelta:                    body.IsDelta,
		DeltaKey:                   stringToNull(body.DeltaKey),
		DeltaValue:                 stringToNull(body.DeltaValue),
		DeltaEnabled:               body.DeltaEnabled,
		DeltaDescription:           body.DeltaDescription,
		DeltaDisplayOrder:          float32ToNullFloat64UrlEncoded(body.DeltaDisplayOrder),
		CreatedAt:                  body.CreatedAt,
		UpdatedAt:                  body.UpdatedAt,
	}
}

func DeserializeBodyUrlEncodedGenToModel(body gen.HttpBodyUrlencoded) mhttp.HTTPBodyUrlencoded {
	return mhttp.HTTPBodyUrlencoded{
		ID:                         body.ID,
		HttpID:                     body.HttpID,
		Key:                        body.Key,
		Value:                      body.Value,
		Enabled:                    body.Enabled,
		Description:                body.Description,
		DisplayOrder:               float32(body.DisplayOrder),
		ParentHttpBodyUrlEncodedID: bytesToIDWrap(body.ParentHttpBodyUrlencodedID),
		IsDelta:                    body.IsDelta,
		DeltaKey:                   nullToString(body.DeltaKey),
		DeltaValue:                 nullToString(body.DeltaValue),
		DeltaEnabled:               body.DeltaEnabled,
		DeltaDescription:           body.DeltaDescription,
		DeltaDisplayOrder:          nullFloat64ToFloat32UrlEncoded(body.DeltaDisplayOrder),
		CreatedAt:                  body.CreatedAt,
		UpdatedAt:                  body.UpdatedAt,
	}
}
