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

var ErrNoHttpBodyFormFound = errors.New("no http body form found")

type HttpBodyFormService struct {
	reader  *BodyFormReader
	queries *gen.Queries
}

func NewHttpBodyFormService(queries *gen.Queries) *HttpBodyFormService {
	return &HttpBodyFormService{
		reader:  NewBodyFormReaderFromQueries(queries),
		queries: queries,
	}
}

func (s *HttpBodyFormService) TX(tx *sql.Tx) *HttpBodyFormService {
	newQueries := s.queries.WithTx(tx)
	return &HttpBodyFormService{
		reader:  NewBodyFormReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func (s *HttpBodyFormService) Create(ctx context.Context, body *mhttp.HTTPBodyForm) error {
	return NewBodyFormWriterFromQueries(s.queries).Create(ctx, body)
}

func (s *HttpBodyFormService) CreateBulk(ctx context.Context, bodyForms []mhttp.HTTPBodyForm) error {
	return NewBodyFormWriterFromQueries(s.queries).CreateBulk(ctx, bodyForms)
}

func (s *HttpBodyFormService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyForm, error) {
	return s.reader.GetByID(ctx, id)
}

func (s *HttpBodyFormService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	return s.reader.GetByHttpID(ctx, httpID)
}

func (s *HttpBodyFormService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	return s.reader.GetByHttpIDOrdered(ctx, httpID)
}

func (s *HttpBodyFormService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	return s.reader.GetByIDs(ctx, ids)
}

func (s *HttpBodyFormService) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPBodyForm, error) {
	return s.reader.GetByHttpIDs(ctx, httpIDs)
}

func (s *HttpBodyFormService) Update(ctx context.Context, body *mhttp.HTTPBodyForm) error {
	return NewBodyFormWriterFromQueries(s.queries).Update(ctx, body)
}

func (s *HttpBodyFormService) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float32) error {
	return NewBodyFormWriterFromQueries(s.queries).UpdateOrder(ctx, id, httpID, order)
}

func (s *HttpBodyFormService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return NewBodyFormWriterFromQueries(s.queries).UpdateDelta(ctx, id, deltaKey, deltaValue, deltaEnabled, deltaDescription, deltaOrder)
}

func (s *HttpBodyFormService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return NewBodyFormWriterFromQueries(s.queries).Delete(ctx, id)
}

func (s *HttpBodyFormService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	return NewBodyFormWriterFromQueries(s.queries).DeleteByHttpID(ctx, httpID)
}

func (s *HttpBodyFormService) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	return NewBodyFormWriterFromQueries(s.queries).ResetDelta(ctx, id)
}

func (s *HttpBodyFormService) GetStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]gen.GetHTTPBodyFormStreamingRow, error) {
	return s.reader.GetStreaming(ctx, httpIDs, updatedAt)
}

// Conversion functions

func float32ToNullFloat64(f *float32) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(*f), Valid: true}
}

func nullFloat64ToFloat32(nf sql.NullFloat64) *float32 {
	if !nf.Valid {
		return nil
	}
	f := float32(nf.Float64)
	return &f
}

func SerializeBodyFormModelToGen(body mhttp.HTTPBodyForm) gen.HttpBodyForm {
	return gen.HttpBodyForm{
		ID:                   body.ID,
		HttpID:               body.HttpID,
		Key:                  body.Key,
		Value:                body.Value,
		Enabled:              body.Enabled,
		Description:          body.Description,
		DisplayOrder:         float64(body.DisplayOrder),
		ParentHttpBodyFormID: idWrapToBytes(body.ParentHttpBodyFormID),
		IsDelta:              body.IsDelta,
		DeltaKey:             stringToNull(body.DeltaKey),
		DeltaValue:           stringToNull(body.DeltaValue),
		DeltaEnabled:         body.DeltaEnabled,
		DeltaDescription:     body.DeltaDescription,
		DeltaDisplayOrder:    float32ToNullFloat64(body.DeltaDisplayOrder),
		CreatedAt:            body.CreatedAt,
		UpdatedAt:            body.UpdatedAt,
	}
}

func DeserializeBodyFormGenToModel(row gen.GetHTTPBodyFormsRow) mhttp.HTTPBodyForm {
	return mhttp.HTTPBodyForm{
		ID:                   row.ID,
		HttpID:               row.HttpID,
		Key:                  row.Key,
		Value:                row.Value,
		Enabled:              row.Enabled,
		Description:          row.Description,
		DisplayOrder:         float32(row.DisplayOrder),
		ParentHttpBodyFormID: bytesToIDWrap(row.ParentHttpBodyFormID),
		IsDelta:              row.IsDelta,
		DeltaKey:             nullToString(row.DeltaKey),
		DeltaValue:           nullToString(row.DeltaValue),
		DeltaEnabled:         row.DeltaEnabled,
		DeltaDescription:     row.DeltaDescription,
		DeltaDisplayOrder:    nil, // Not available in row
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}

func deserializeBodyFormByIDsRowToModel(row gen.GetHTTPBodyFormsByIDsRow) mhttp.HTTPBodyForm {
	return mhttp.HTTPBodyForm{
		ID:                   row.ID,
		HttpID:               row.HttpID,
		Key:                  row.Key,
		Value:                row.Value,
		Enabled:              row.Enabled,
		Description:          row.Description,
		DisplayOrder:         float32(row.DisplayOrder),
		ParentHttpBodyFormID: bytesToIDWrap(row.ParentHttpBodyFormID),
		IsDelta:              row.IsDelta,
		DeltaKey:             nullToString(row.DeltaKey),
		DeltaValue:           nullToString(row.DeltaValue),
		DeltaEnabled:         row.DeltaEnabled,
		DeltaDescription:     row.DeltaDescription,
		DeltaDisplayOrder:    nullFloat64ToFloat32(row.DeltaDisplayOrder),
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}
