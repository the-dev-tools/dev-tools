package shttp

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHttpHeaderFound = errors.New("no HTTP header found")

type HttpHeaderService struct {
	queries *gen.Queries
}

func ConvertToDBHttpHeader(header mhttp.HTTPHeader) gen.HttpHeader {
	return gen.HttpHeader{
		ID:               header.ID,
		HttpID:           header.HttpID,
		HeaderKey:        header.HeaderKey,
		HeaderValue:      header.HeaderValue,
		Description:      header.Description,
		Enabled:          header.Enabled,
		ParentHeaderID:   header.ParentHeaderID,
		IsDelta:          header.IsDelta,
		DeltaHeaderKey:   header.DeltaHeaderKey,
		DeltaHeaderValue: header.DeltaHeaderValue,
		DeltaDescription: header.DeltaDescription,
		DeltaEnabled:     header.DeltaEnabled,
		CreatedAt:        header.CreatedAt,
		UpdatedAt:        header.UpdatedAt,
	}
}

func ConvertToModelHttpHeader(dbHeader gen.HttpHeader) mhttp.HTTPHeader {
	return mhttp.HTTPHeader{
		ID:               dbHeader.ID,
		HttpID:           dbHeader.HttpID,
		HeaderKey:        dbHeader.HeaderKey,
		HeaderValue:      dbHeader.HeaderValue,
		Description:      dbHeader.Description,
		Enabled:          dbHeader.Enabled,
		ParentHeaderID:   dbHeader.ParentHeaderID,
		IsDelta:          dbHeader.IsDelta,
		DeltaHeaderKey:   dbHeader.DeltaHeaderKey,
		DeltaHeaderValue: dbHeader.DeltaHeaderValue,
		DeltaDescription: dbHeader.DeltaDescription,
		DeltaEnabled:     dbHeader.DeltaEnabled,
		CreatedAt:        dbHeader.CreatedAt,
		UpdatedAt:        dbHeader.UpdatedAt,
	}
}

func (hhs HttpHeaderService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPHeader, error) {
	headers, err := hhs.queries.GetHTTPHeaders(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPHeader{}, nil
		}
		return nil, err
	}
	return tgeneric.MassConvert(headers, ConvertToModelHttpHeader), nil
}

func NewHttpHeaderService(queries *gen.Queries) *HttpHeaderService {
	return &HttpHeaderService{
		queries: queries,
	}
}

func (hhs HttpHeaderService) Create(ctx context.Context, header mhttp.HTTPHeader) (*mhttp.HTTPHeader, error) {
	// Check permissions
	_, err := hhs.queries.GetHTTP(ctx, header.HttpID)
	if err != nil {
		return nil, err
	}

	// Create the header
	dbHeader := ConvertToDBHttpHeader(header)
	err = hhs.queries.CreateHTTPHeader(ctx, gen.CreateHTTPHeaderParams{
		ID:               dbHeader.ID,
		HttpID:           dbHeader.HttpID,
		HeaderKey:        dbHeader.HeaderKey,
		HeaderValue:      dbHeader.HeaderValue,
		Description:      dbHeader.Description,
		Enabled:          dbHeader.Enabled,
		ParentHeaderID:   dbHeader.ParentHeaderID,
		IsDelta:          dbHeader.IsDelta,
		DeltaHeaderKey:   dbHeader.DeltaHeaderKey,
		DeltaHeaderValue: dbHeader.DeltaHeaderValue,
		DeltaDescription: dbHeader.DeltaDescription,
		DeltaEnabled:     dbHeader.DeltaEnabled,
		CreatedAt:        dbHeader.CreatedAt,
		UpdatedAt:        dbHeader.UpdatedAt,
	})
	if err != nil {
		return nil, err
	}

	// Get the created record
	createdHeaders, err := hhs.queries.GetHTTPHeadersByIDs(ctx, []idwrap.IDWrap{dbHeader.ID})
	if err != nil {
		return nil, err
	}
	if len(createdHeaders) == 0 {
		return nil, ErrNoHttpHeaderFound
	}

	result := ConvertToModelHttpHeader(createdHeaders[0])
	return &result, nil
}

func (hhs HttpHeaderService) TX(tx *sql.Tx) *HttpHeaderService {
	return &HttpHeaderService{
		queries: hhs.queries.WithTx(tx),
	}
}
