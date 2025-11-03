package shttp

import (
	"context"
	"database/sql"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

type HttpBodyRawService struct {
	q *gen.Queries
}

func NewHttpBodyRawService(q *gen.Queries) *HttpBodyRawService {
	return &HttpBodyRawService{q: q}
}

func (s *HttpBodyRawService) Create(ctx context.Context, httpID idwrap.IDWrap, rawData []byte, contentType string) (*mhttp.HTTPBodyRaw, error) {
	// Check permissions
	_, err := s.q.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	// Create the body raw
	id := idwrap.NewNow()
	err = s.q.CreateHTTPBodyRaw(ctx, gen.CreateHTTPBodyRawParams{
		ID:              id,
		HttpID:          httpID,
		RawData:         rawData,
		ContentType:     contentType,
		CompressionType: 0, // No compression
		CreatedAt:       0, // Will be set by database
		UpdatedAt:       0, // Will be set by database
	})
	if err != nil {
		return nil, err
	}

	// Get the created record
	bodyRaw, err := s.q.GetHTTPBodyRawByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &mhttp.HTTPBodyRaw{
		ID:               bodyRaw.ID,
		HttpID:           bodyRaw.HttpID,
		RawData:          bodyRaw.RawData,
		ContentType:      bodyRaw.ContentType,
		CompressionType:  bodyRaw.CompressionType,
		ParentBodyRawID:  bodyRaw.ParentBodyRawID,
		IsDelta:          bodyRaw.IsDelta,
		DeltaRawData:     bodyRaw.DeltaRawData,
		DeltaContentType: bodyRaw.DeltaContentType,
		CreatedAt:        bodyRaw.CreatedAt,
		UpdatedAt:        bodyRaw.UpdatedAt,
	}, nil
}

func (s *HttpBodyRawService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	bodyRaw, err := s.q.GetHTTPBodyRawByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &mhttp.HTTPBodyRaw{
		ID:               bodyRaw.ID,
		HttpID:           bodyRaw.HttpID,
		RawData:          bodyRaw.RawData,
		ContentType:      bodyRaw.ContentType,
		CompressionType:  bodyRaw.CompressionType,
		ParentBodyRawID:  bodyRaw.ParentBodyRawID,
		IsDelta:          bodyRaw.IsDelta,
		DeltaRawData:     bodyRaw.DeltaRawData,
		DeltaContentType: bodyRaw.DeltaContentType,
		CreatedAt:        bodyRaw.CreatedAt,
		UpdatedAt:        bodyRaw.UpdatedAt,
	}, nil
}

func (s *HttpBodyRawService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	// Check permissions
	_, err := s.q.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	// Get the body raw for this HTTP
	bodyRaw, err := s.q.GetHTTPBodyRaw(ctx, httpID)
	if err != nil {
		return nil, err
	}

	return &mhttp.HTTPBodyRaw{
		ID:               bodyRaw.ID,
		HttpID:           bodyRaw.HttpID,
		RawData:          bodyRaw.RawData,
		ContentType:      bodyRaw.ContentType,
		CompressionType:  bodyRaw.CompressionType,
		ParentBodyRawID:  bodyRaw.ParentBodyRawID,
		IsDelta:          bodyRaw.IsDelta,
		DeltaRawData:     bodyRaw.DeltaRawData,
		DeltaContentType: bodyRaw.DeltaContentType,
		CreatedAt:        bodyRaw.CreatedAt,
		UpdatedAt:        bodyRaw.UpdatedAt,
	}, nil
}

func (s *HttpBodyRawService) Update(ctx context.Context, id idwrap.IDWrap, rawData []byte, contentType string) (*mhttp.HTTPBodyRaw, error) {
	// Check if exists and get permissions
	existing, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check permissions via the HTTP
	_, err = s.q.GetHTTP(ctx, existing.HttpID)
	if err != nil {
		return nil, err
	}

	// Update the body raw
	err = s.q.UpdateHTTPBodyRaw(ctx, gen.UpdateHTTPBodyRawParams{
		ID:              id,
		RawData:         rawData,
		ContentType:     contentType,
		CompressionType: 0, // No compression
	})
	if err != nil {
		return nil, err
	}

	// Get the updated record
	return s.Get(ctx, id)
}

func (s *HttpBodyRawService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	// Check if exists and get permissions
	existing, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	// Check permissions via the HTTP
	_, err = s.q.GetHTTP(ctx, existing.HttpID)
	if err != nil {
		return err
	}

	// Delete the body raw
	return s.q.DeleteHTTPBodyRaw(ctx, id)
}

func (s *HttpBodyRawService) TX(tx *sql.Tx) *HttpBodyRawService {
	return &HttpBodyRawService{
		q: s.q.WithTx(tx),
	}
}
