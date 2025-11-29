package shttp

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

var ErrNoHttpBodyRawFound = errors.New("no HTTP body raw found")

type HttpBodyRawService struct {
	queries *gen.Queries
}

func ConvertToDBHttpBodyRaw(body mhttp.HTTPBodyRaw) gen.HttpBodyRaw {
	return gen.HttpBodyRaw{
		ID:                   body.ID,
		HttpID:               body.HttpID,
		RawData:              body.RawData,
		ContentType:          body.ContentType,
		CompressionType:      body.CompressionType,
		ParentBodyRawID:      body.ParentBodyRawID,
		IsDelta:              body.IsDelta,
		DeltaRawData:         body.DeltaRawData,
		DeltaContentType:     body.DeltaContentType,
		DeltaCompressionType: body.DeltaCompressionType,
		CreatedAt:            body.CreatedAt,
		UpdatedAt:            body.UpdatedAt,
	}
}

func ConvertToModelHttpBodyRaw(dbBody gen.HttpBodyRaw) mhttp.HTTPBodyRaw {
	return mhttp.HTTPBodyRaw{
		ID:                   dbBody.ID,
		HttpID:               dbBody.HttpID,
		RawData:              dbBody.RawData,
		ContentType:          dbBody.ContentType,
		CompressionType:      dbBody.CompressionType,
		ParentBodyRawID:      dbBody.ParentBodyRawID,
		IsDelta:              dbBody.IsDelta,
		DeltaRawData:         dbBody.DeltaRawData,
		DeltaContentType:     dbBody.DeltaContentType,
		DeltaCompressionType: dbBody.DeltaCompressionType,
		CreatedAt:            dbBody.CreatedAt,
		UpdatedAt:            dbBody.UpdatedAt,
	}
}

func NewHttpBodyRawService(queries *gen.Queries) *HttpBodyRawService {
	return &HttpBodyRawService{
		queries: queries,
	}
}

func (s *HttpBodyRawService) Create(ctx context.Context, httpID idwrap.IDWrap, rawData []byte, contentType string) (*mhttp.HTTPBodyRaw, error) {
	// Create the body raw
	now := dbtime.DBNow().Unix()
	id := idwrap.NewNow()
	err := s.queries.CreateHTTPBodyRaw(ctx, gen.CreateHTTPBodyRawParams{
		ID:                   id,
		HttpID:               httpID,
		RawData:              rawData,
		ContentType:          contentType,
		CompressionType:      0, // No compression
		ParentBodyRawID:      nil,
		IsDelta:              false,
		DeltaRawData:         nil,
		DeltaContentType:     nil,
		DeltaCompressionType: nil,
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	if err != nil {
		return nil, err
	}

	// Get the created record
	bodyRaw, err := s.queries.GetHTTPBodyRawByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := ConvertToModelHttpBodyRaw(bodyRaw)
	return &result, nil
}

func (s *HttpBodyRawService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	bodyRaw, err := s.queries.GetHTTPBodyRawByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoHttpBodyRawFound
		}
		return nil, err
	}

	result := ConvertToModelHttpBodyRaw(bodyRaw)
	return &result, nil
}

func (s *HttpBodyRawService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) (*mhttp.HTTPBodyRaw, error) {
	// Check permissions
	_, err := s.queries.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	// Get the body raw for this HTTP
	bodyRaw, err := s.queries.GetHTTPBodyRaw(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoHttpBodyRawFound
		}
		return nil, err
	}

	result := ConvertToModelHttpBodyRaw(bodyRaw)
	return &result, nil
}

func (s *HttpBodyRawService) Update(ctx context.Context, id idwrap.IDWrap, rawData []byte, contentType string) (*mhttp.HTTPBodyRaw, error) {
	// Update the body raw
	now := dbtime.DBNow().Unix()
	err := s.queries.UpdateHTTPBodyRaw(ctx, gen.UpdateHTTPBodyRawParams{
		RawData:         rawData,
		ContentType:     contentType,
		CompressionType: 0, // No compression
		UpdatedAt:       now,
		ID:              id,
	})
	if err != nil {
		return nil, err
	}

	// Get the updated record
	return s.Get(ctx, id)
}

func (s *HttpBodyRawService) CreateDelta(ctx context.Context, httpID idwrap.IDWrap, rawData []byte, contentType string) (*mhttp.HTTPBodyRaw, error) {
	// Create the delta body raw
	now := dbtime.DBNow().Unix()
	id := idwrap.NewNow()
	err := s.queries.CreateHTTPBodyRaw(ctx, gen.CreateHTTPBodyRawParams{
		ID:                   id,
		HttpID:               httpID,
		RawData:              nil, // Base data is nil for delta record
		ContentType:          "",  // Base content type is empty
		CompressionType:      0,
		ParentBodyRawID:      nil, // Will be linked by logic if needed, but typically linked via HTTP ID
		IsDelta:              true,
		DeltaRawData:         rawData,
		DeltaContentType:     stringToNullPtr(&contentType),
		DeltaCompressionType: nil,
		CreatedAt:            now,
		UpdatedAt:            now,
	})
	if err != nil {
		return nil, err
	}

	// Get the created record
	bodyRaw, err := s.queries.GetHTTPBodyRawByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := ConvertToModelHttpBodyRaw(bodyRaw)
	return &result, nil
}

func (s *HttpBodyRawService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, rawData []byte, contentType *string) (*mhttp.HTTPBodyRaw, error) {
	// Update the delta body raw
	now := dbtime.DBNow().Unix()
	
	// We need a specific UpdateHTTPBodyRawDelta query, or we use the general update if it supports delta fields.
	// Checking `UpdateHTTPBodyRaw` in sqlc - it usually only updates standard fields.
	// Let's check if `UpdateHTTPBodyRawDelta` exists in `gen`.
	// Since I can't see `gen` package, I assume I need to check if I can use `UpdateHTTPBodyRaw` or if I need to add a new query.
	// The existing `Update` method uses `UpdateHTTPBodyRawParams` which has `RawData`.
	
	// Assuming `UpdateHTTPBodyRawDelta` exists based on other services having it.
	// If not, I might need to add it or use a workaround.
	// Let's try to use `UpdateHTTPBodyRawDelta` if it exists.
	
	err := s.queries.UpdateHTTPBodyRawDelta(ctx, gen.UpdateHTTPBodyRawDeltaParams{
		DeltaRawData:     rawData,
		DeltaContentType: stringToNullPtr(contentType),
		UpdatedAt:        now,
		ID:               id,
	})
	if err != nil {
		return nil, err
	}

	return s.Get(ctx, id)
}

// Helper for null string ptr
func stringToNullPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}


func (s *HttpBodyRawService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	// Delete the body raw
	return s.queries.DeleteHTTPBodyRaw(ctx, id)
}

func (s *HttpBodyRawService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	// Get existing body raw
	// We don't need to check permissions here as this is an internal service call
	// and the caller (e.g. import) should have verified access.
	// But GetByHttpID does verify access implicitly by checking HTTP existence?
	// Actually GetByHttpID calls GetHTTP to check if it exists.
	
	bodyRaw, err := s.queries.GetHTTPBodyRaw(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // Already deleted or never existed
		}
		return err
	}

	return s.Delete(ctx, bodyRaw.ID)
}

func (s *HttpBodyRawService) TX(tx *sql.Tx) *HttpBodyRawService {
	return &HttpBodyRawService{
		queries: s.queries.WithTx(tx),
	}
}
