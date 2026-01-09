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

var ErrNoHttpHeaderFound = errors.New("no http header found")

type HttpHeaderService struct {
	reader  *HeaderReader
	queries *gen.Queries
}

func NewHttpHeaderService(queries *gen.Queries) HttpHeaderService {
	return HttpHeaderService{
		reader:  NewHeaderReaderFromQueries(queries),
		queries: queries,
	}
}

func (h HttpHeaderService) TX(tx *sql.Tx) HttpHeaderService {
	newQueries := h.queries.WithTx(tx)
	return HttpHeaderService{
		reader:  NewHeaderReaderFromQueries(newQueries),
		queries: newQueries,
	}
}

func NewHttpHeaderServiceTX(ctx context.Context, tx *sql.Tx) (*HttpHeaderService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	headerService := HttpHeaderService{
		reader:  NewHeaderReaderFromQueries(queries),
		queries: queries,
	}

	return &headerService, nil
}

// SerializeHeaderModelToGen converts model HTTPHeader to DB HttpHeader
func SerializeHeaderModelToGen(header mhttp.HTTPHeader) gen.HttpHeader {
	var deltaDisplayOrder sql.NullFloat64
	if header.DeltaDisplayOrder != nil {
		deltaDisplayOrder = sql.NullFloat64{Float64: float64(*header.DeltaDisplayOrder), Valid: true}
	}

	return gen.HttpHeader{
		ID:                header.ID,
		HttpID:            header.HttpID,
		HeaderKey:         header.Key,
		HeaderValue:       header.Value,
		Description:       header.Description,
		Enabled:           header.Enabled,
		ParentHeaderID:    header.ParentHttpHeaderID,
		IsDelta:           header.IsDelta,
		DeltaHeaderKey:    header.DeltaKey,
		DeltaHeaderValue:  header.DeltaValue,
		DeltaDescription:  header.DeltaDescription,
		DeltaEnabled:      header.DeltaEnabled,
		DeltaDisplayOrder: deltaDisplayOrder,
		DisplayOrder:      float64(header.DisplayOrder),
		CreatedAt:         header.CreatedAt,
		UpdatedAt:         header.UpdatedAt,
	}
}

// DeserializeHeaderGenToModel converts DB HttpHeader to model HTTPHeader
func DeserializeHeaderGenToModel(header gen.HttpHeader) mhttp.HTTPHeader {
	var deltaOrder *float32
	if header.DeltaDisplayOrder.Valid {
		val := float32(header.DeltaDisplayOrder.Float64)
		deltaOrder = &val
	}

	return mhttp.HTTPHeader{
		ID:                 header.ID,
		HttpID:             header.HttpID,
		Key:                header.HeaderKey,
		Value:              header.HeaderValue,
		Enabled:            header.Enabled,
		Description:        header.Description,
		DisplayOrder:       float32(header.DisplayOrder),
		ParentHttpHeaderID: header.ParentHeaderID,
		IsDelta:            header.IsDelta,
		DeltaKey:           header.DeltaHeaderKey,
		DeltaValue:         header.DeltaHeaderValue,
		DeltaDescription:   header.DeltaDescription,
		DeltaEnabled:       header.DeltaEnabled,
		DeltaDisplayOrder:  deltaOrder,
		CreatedAt:          header.CreatedAt,
		UpdatedAt:          header.UpdatedAt,
	}
}

func (h HttpHeaderService) Create(ctx context.Context, header *mhttp.HTTPHeader) error {
	return NewHeaderWriterFromQueries(h.queries).Create(ctx, header)
}

func (h HttpHeaderService) CreateBulk(ctx context.Context, httpID idwrap.IDWrap, headers []mhttp.HTTPHeader) error {
	return NewHeaderWriterFromQueries(h.queries).CreateBulk(ctx, httpID, headers)
}

func (h HttpHeaderService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	return h.reader.GetByHttpID(ctx, httpID)
}

func (h HttpHeaderService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	return h.reader.GetByHttpIDOrdered(ctx, httpID)
}

func (h HttpHeaderService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	return h.reader.GetByIDs(ctx, ids)
}

func (h HttpHeaderService) GetByID(ctx context.Context, headerID idwrap.IDWrap) (mhttp.HTTPHeader, error) {
	return h.reader.GetByID(ctx, headerID)
}

func (h HttpHeaderService) Update(ctx context.Context, header *mhttp.HTTPHeader) error {
	return NewHeaderWriterFromQueries(h.queries).Update(ctx, header)
}

func (h HttpHeaderService) UpdateDelta(ctx context.Context, headerID idwrap.IDWrap, deltaKey, deltaValue, deltaDescription *string, deltaEnabled *bool, deltaOrder *float32) error {
	return NewHeaderWriterFromQueries(h.queries).UpdateDelta(ctx, headerID, deltaKey, deltaValue, deltaDescription, deltaEnabled, deltaOrder)
}

func (h HttpHeaderService) Delete(ctx context.Context, headerID idwrap.IDWrap) error {
	return NewHeaderWriterFromQueries(h.queries).Delete(ctx, headerID)
}

func (h HttpHeaderService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	return NewHeaderWriterFromQueries(h.queries).DeleteByHttpID(ctx, httpID)
}

func (h HttpHeaderService) UpdateOrder(ctx context.Context, headerID idwrap.IDWrap, displayOrder float64) error {
	return NewHeaderWriterFromQueries(h.queries).UpdateOrder(ctx, headerID, displayOrder)
}

func float32ToNullFloat64Header(f *float32) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(*f), Valid: true}
}
