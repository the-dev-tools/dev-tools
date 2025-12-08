//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

var ErrNoHttpHeaderFound = errors.New("no http header found")

type HttpHeaderService struct {
	queries *gen.Queries
}

func NewHttpHeaderService(queries *gen.Queries) HttpHeaderService {
	return HttpHeaderService{queries: queries}
}

func (h HttpHeaderService) TX(tx *sql.Tx) HttpHeaderService {
	return HttpHeaderService{queries: h.queries.WithTx(tx)}
}

func NewHttpHeaderServiceTX(ctx context.Context, tx *sql.Tx) (*HttpHeaderService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	headerService := HttpHeaderService{
		queries: queries,
	}

	return &headerService, nil
}

// SerializeHeaderModelToGen converts model HTTPHeader to DB HttpHeader
func SerializeHeaderModelToGen(header mhttp.HTTPHeader) gen.HttpHeader {
	var deltaDisplayOrder sql.NullFloat64
	if header.DeltaOrder != nil {
		deltaDisplayOrder = sql.NullFloat64{Float64: float64(*header.DeltaOrder), Valid: true}
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
		DisplayOrder:      float64(header.Order),
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
		Order:              float32(header.DisplayOrder),
		ParentHttpHeaderID: header.ParentHeaderID,
		IsDelta:            header.IsDelta,
		DeltaKey:           header.DeltaHeaderKey,
		DeltaValue:         header.DeltaHeaderValue,
		DeltaDescription:   header.DeltaDescription,
		DeltaEnabled:       header.DeltaEnabled,
		DeltaOrder:         deltaOrder,
		CreatedAt:          header.CreatedAt,
		UpdatedAt:          header.UpdatedAt,
	}
}

func (h HttpHeaderService) Create(ctx context.Context, header *mhttp.HTTPHeader) error {
	if header == nil {
		return errors.New("header cannot be nil")
	}

	now := time.Now().Unix()
	header.CreatedAt = now
	header.UpdatedAt = now

	dbHeader := SerializeHeaderModelToGen(*header)
	return h.queries.CreateHTTPHeader(ctx, gen.CreateHTTPHeaderParams(dbHeader))
}

func (h HttpHeaderService) CreateBulk(ctx context.Context, httpID idwrap.IDWrap, headers []mhttp.HTTPHeader) error {
	if len(headers) == 0 {
		return nil
	}

	// Create headers individually since bulk operation doesn't exist
	for _, header := range headers {
		header.HttpID = httpID
		if err := h.Create(ctx, &header); err != nil {
			return err
		}
	}

	return nil
}

func (h HttpHeaderService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	dbHeaders, err := h.queries.GetHTTPHeaders(ctx, httpID)
	if err != nil {
		return nil, err
	}

	var headers []mhttp.HTTPHeader
	for _, dbHeader := range dbHeaders {
		header := DeserializeHeaderGenToModel(dbHeader)
		headers = append(headers, header)
	}

	return headers, nil
}

func (h HttpHeaderService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	// GetByHttpID now uses ORDER BY display_order in the query
	return h.GetByHttpID(ctx, httpID)
}

func (h HttpHeaderService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPHeader{}, nil
	}

	dbHeaders, err := h.queries.GetHTTPHeadersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	var headers []mhttp.HTTPHeader
	for _, dbHeader := range dbHeaders {
		header := DeserializeHeaderGenToModel(dbHeader)
		headers = append(headers, header)
	}

	return headers, nil
}

func (h HttpHeaderService) GetByID(ctx context.Context, headerID idwrap.IDWrap) (mhttp.HTTPHeader, error) {
	// Since there's no specific GetByID query for HTTP headers, we'll use GetByIDs
	headers, err := h.GetByIDs(ctx, []idwrap.IDWrap{headerID})
	if err != nil {
		return mhttp.HTTPHeader{}, err
	}

	if len(headers) == 0 {
		return mhttp.HTTPHeader{}, ErrNoHttpHeaderFound
	}

	return headers[0], nil
}

func (h HttpHeaderService) Update(ctx context.Context, header *mhttp.HTTPHeader) error {
	if header == nil {
		return errors.New("header cannot be nil")
	}

	dbHeader := SerializeHeaderModelToGen(*header)
	return h.queries.UpdateHTTPHeader(ctx, gen.UpdateHTTPHeaderParams{
		HeaderKey:    dbHeader.HeaderKey,
		HeaderValue:  dbHeader.HeaderValue,
		Description:  dbHeader.Description,
		Enabled:      dbHeader.Enabled,
		DisplayOrder: dbHeader.DisplayOrder,
		ID:           dbHeader.ID,
	})
}

func (h HttpHeaderService) UpdateDelta(ctx context.Context, headerID idwrap.IDWrap, deltaKey, deltaValue, deltaDescription *string, deltaEnabled *bool) error {
	return h.queries.UpdateHTTPHeaderDelta(ctx, gen.UpdateHTTPHeaderDeltaParams{
		DeltaHeaderKey:   deltaKey,
		DeltaHeaderValue: deltaValue,
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               headerID,
	})
}

func (h HttpHeaderService) Delete(ctx context.Context, headerID idwrap.IDWrap) error {
	return h.queries.DeleteHTTPHeader(ctx, headerID)
}

func (h HttpHeaderService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	// Since bulk delete might not be generated, we fetch and delete one by one
	// This is less efficient but safer without knowing available queries
	headers, err := h.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, header := range headers {
		if err := h.Delete(ctx, header.ID); err != nil {
			return err
		}
	}
	return nil
}

func (h HttpHeaderService) UpdateOrder(ctx context.Context, headerID idwrap.IDWrap, displayOrder float64) error {
	// First get the header to extract its HTTP ID for validation
	header, err := h.GetByID(ctx, headerID)
	if err != nil {
		return err
	}

	return h.queries.UpdateHTTPHeaderOrder(ctx, gen.UpdateHTTPHeaderOrderParams{
		DisplayOrder: displayOrder,
		ID:           headerID,
		HttpID:       header.HttpID,
	})
}
