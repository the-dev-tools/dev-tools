package shttp

import (
	"context"
	"database/sql"
	"errors"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
)

type HttpBodyUrlencodedService struct {
	q *gen.Queries
}

func NewHttpBodyUrlencodedService(q *gen.Queries) *HttpBodyUrlencodedService {
	return &HttpBodyUrlencodedService{q: q}
}

func ConvertToModelHttpBodyUrlencoded(dbBody gen.HttpBodyUrlencoded) mhttp.HTTPBodyUrlencoded {
	var parentID *idwrap.IDWrap
	if len(dbBody.ParentHttpBodyUrlencodedID) > 0 {
		wrappedID := idwrap.NewFromBytesMust(dbBody.ParentHttpBodyUrlencodedID)
		parentID = &wrappedID
	}

	var deltaKey *string
	if dbBody.DeltaKey.Valid {
		deltaKey = &dbBody.DeltaKey.String
	}

	var deltaValue *string
	if dbBody.DeltaValue.Valid {
		deltaValue = &dbBody.DeltaValue.String
	}

	return mhttp.HTTPBodyUrlencoded{
		ID:                     dbBody.ID,
		HttpID:                 dbBody.HttpID,
		UrlencodedKey:          dbBody.Key,
		UrlencodedValue:        dbBody.Value,
		Description:            dbBody.Description,
		Enabled:                dbBody.Enabled,
		ParentBodyUrlencodedID: parentID,
		IsDelta:                dbBody.IsDelta,
		DeltaUrlencodedKey:     deltaKey,
		DeltaUrlencodedValue:   deltaValue,
		DeltaDescription:       dbBody.DeltaDescription,
		DeltaEnabled:           dbBody.DeltaEnabled,
		CreatedAt:              dbBody.CreatedAt,
		UpdatedAt:              dbBody.UpdatedAt,
	}
}

func (s *HttpBodyUrlencodedService) Create(ctx context.Context, httpID idwrap.IDWrap, key string, value string, description string) (*mhttp.HTTPBodyUrlencoded, error) {
	// Check permissions
	_, err := s.q.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	// Create the body urlencoded
	id := idwrap.NewNow()
	err = s.q.CreateHTTPBodyUrlEncoded(ctx, gen.CreateHTTPBodyUrlEncodedParams{
		ID:                         id,
		HttpID:                     httpID,
		Key:                        key,
		Value:                      value,
		Description:                description,
		Enabled:                    true,
		Order:                      0,
		ParentHttpBodyUrlencodedID: nil,
		IsDelta:                    false,
	})
	if err != nil {
		return nil, err
	}

	// Get the created record
	bodyUrlencodeds, err := s.q.GetHTTPBodyUrlEncodedsByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(bodyUrlencodeds) == 0 {
		return nil, errors.New("failed to retrieve created body urlencoded")
	}

	bodyUrlencoded := bodyUrlencodeds[0]
	result := ConvertToModelHttpBodyUrlencoded(bodyUrlencoded)
	return &result, nil
}

func (s *HttpBodyUrlencodedService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyUrlencoded, error) {
	bodyUrlencodeds, err := s.q.GetHTTPBodyUrlEncodedsByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(bodyUrlencodeds) == 0 {
		return nil, sql.ErrNoRows
	}

	bodyUrlencoded := bodyUrlencodeds[0]
	result := ConvertToModelHttpBodyUrlencoded(bodyUrlencoded)
	return &result, nil
}

func (s *HttpBodyUrlencodedService) Update(ctx context.Context, id idwrap.IDWrap, key string, value string, description string, enabled bool) (*mhttp.HTTPBodyUrlencoded, error) {
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

	// Update the body urlencoded
	err = s.q.UpdateHTTPBodyUrlEncoded(ctx, gen.UpdateHTTPBodyUrlEncodedParams{
		ID:          id,
		Key:         key,
		Value:       value,
		Description: description,
		Enabled:     enabled,
	})
	if err != nil {
		return nil, err
	}

	// Get the updated record
	return s.Get(ctx, id)
}

func (s *HttpBodyUrlencodedService) Delete(ctx context.Context, id idwrap.IDWrap) error {
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

	// Delete the body urlencoded
	return s.q.DeleteHTTPBodyUrlEncoded(ctx, id)
}

func (s *HttpBodyUrlencodedService) List(ctx context.Context, httpID idwrap.IDWrap) ([]*mhttp.HTTPBodyUrlencoded, error) {
	// Check permissions
	_, err := s.q.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	// Get all body urlencoded for this HTTP
	bodyUrlencodeds, err := s.q.GetHTTPBodyUrlEncodedByHttpID(ctx, httpID)
	if err != nil {
		return nil, err
	}

	result := make([]*mhttp.HTTPBodyUrlencoded, len(bodyUrlencodeds))
	for i, bodyUrlencoded := range bodyUrlencodeds {
		converted := ConvertToModelHttpBodyUrlencoded(bodyUrlencoded)
		result[i] = &converted
	}

	return result, nil
}

func (s *HttpBodyUrlencodedService) TX(tx *sql.Tx) *HttpBodyUrlencodedService {
	return &HttpBodyUrlencodedService{
		q: s.q.WithTx(tx),
	}
}
