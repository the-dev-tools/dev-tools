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

func (s *HttpBodyUrlencodedService) Create(ctx context.Context, httpID idwrap.IDWrap, key string, value string, description string) (*mhttp.HTTPBodyUrlencoded, error) {
	// Check permissions
	_, err := s.q.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	// Create the body urlencoded
	id := idwrap.NewNow()
	err = s.q.CreateHTTPBodyUrlencoded(ctx, gen.CreateHTTPBodyUrlencodedParams{
		ID:              id,
		HttpID:          httpID,
		UrlencodedKey:   key,
		UrlencodedValue: value,
		Description:     description,
		Enabled:         true,
		CreatedAt:       0, // Will be set by database
		UpdatedAt:       0, // Will be set by database
	})
	if err != nil {
		return nil, err
	}

	// Get the created record
	bodyUrlencodeds, err := s.q.GetHTTPBodyUrlencodedByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(bodyUrlencodeds) == 0 {
		return nil, errors.New("failed to retrieve created body urlencoded")
	}

	bodyUrlencoded := bodyUrlencodeds[0]
	return &mhttp.HTTPBodyUrlencoded{
		ID:                     bodyUrlencoded.ID,
		HttpID:                 bodyUrlencoded.HttpID,
		UrlencodedKey:          bodyUrlencoded.UrlencodedKey,
		UrlencodedValue:        bodyUrlencoded.UrlencodedValue,
		Description:            bodyUrlencoded.Description,
		Enabled:                bodyUrlencoded.Enabled,
		ParentBodyUrlencodedID: bodyUrlencoded.ParentBodyUrlencodedID,
		IsDelta:                bodyUrlencoded.IsDelta,
		DeltaUrlencodedKey:     bodyUrlencoded.DeltaUrlencodedKey,
		DeltaUrlencodedValue:   bodyUrlencoded.DeltaUrlencodedValue,
		DeltaDescription:       bodyUrlencoded.DeltaDescription,
		DeltaEnabled:           bodyUrlencoded.DeltaEnabled,
		Prev:                   bodyUrlencoded.Prev,
		Next:                   bodyUrlencoded.Next,
		CreatedAt:              bodyUrlencoded.CreatedAt,
		UpdatedAt:              bodyUrlencoded.UpdatedAt,
	}, nil
}

func (s *HttpBodyUrlencodedService) Get(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyUrlencoded, error) {
	bodyUrlencodeds, err := s.q.GetHTTPBodyUrlencodedByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(bodyUrlencodeds) == 0 {
		return nil, sql.ErrNoRows
	}

	bodyUrlencoded := bodyUrlencodeds[0]
	return &mhttp.HTTPBodyUrlencoded{
		ID:                     bodyUrlencoded.ID,
		HttpID:                 bodyUrlencoded.HttpID,
		UrlencodedKey:          bodyUrlencoded.UrlencodedKey,
		UrlencodedValue:        bodyUrlencoded.UrlencodedValue,
		Description:            bodyUrlencoded.Description,
		Enabled:                bodyUrlencoded.Enabled,
		ParentBodyUrlencodedID: bodyUrlencoded.ParentBodyUrlencodedID,
		IsDelta:                bodyUrlencoded.IsDelta,
		DeltaUrlencodedKey:     bodyUrlencoded.DeltaUrlencodedKey,
		DeltaUrlencodedValue:   bodyUrlencoded.DeltaUrlencodedValue,
		DeltaDescription:       bodyUrlencoded.DeltaDescription,
		DeltaEnabled:           bodyUrlencoded.DeltaEnabled,
		Prev:                   bodyUrlencoded.Prev,
		Next:                   bodyUrlencoded.Next,
		CreatedAt:              bodyUrlencoded.CreatedAt,
		UpdatedAt:              bodyUrlencoded.UpdatedAt,
	}, nil
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
	err = s.q.UpdateHTTPBodyUrlencoded(ctx, gen.UpdateHTTPBodyUrlencodedParams{
		ID:              id,
		UrlencodedKey:   key,
		UrlencodedValue: value,
		Description:     description,
		Enabled:         enabled,
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
	return s.q.DeleteHTTPBodyUrlencoded(ctx, id)
}

func (s *HttpBodyUrlencodedService) List(ctx context.Context, httpID idwrap.IDWrap) ([]*mhttp.HTTPBodyUrlencoded, error) {
	// Check permissions
	_, err := s.q.GetHTTP(ctx, httpID)
	if err != nil {
		return nil, err
	}

	// Get all body urlencoded for this HTTP
	bodyUrlencodeds, err := s.q.GetHTTPBodyUrlencoded(ctx, httpID)
	if err != nil {
		return nil, err
	}

	result := make([]*mhttp.HTTPBodyUrlencoded, len(bodyUrlencodeds))
	for i, bodyUrlencoded := range bodyUrlencodeds {
		result[i] = &mhttp.HTTPBodyUrlencoded{
			ID:                     bodyUrlencoded.ID,
			HttpID:                 bodyUrlencoded.HttpID,
			UrlencodedKey:          bodyUrlencoded.UrlencodedKey,
			UrlencodedValue:        bodyUrlencoded.UrlencodedValue,
			Description:            bodyUrlencoded.Description,
			Enabled:                bodyUrlencoded.Enabled,
			ParentBodyUrlencodedID: bodyUrlencoded.ParentBodyUrlencodedID,
			IsDelta:                bodyUrlencoded.IsDelta,
			DeltaUrlencodedKey:     bodyUrlencoded.DeltaUrlencodedKey,
			DeltaUrlencodedValue:   bodyUrlencoded.DeltaUrlencodedValue,
			DeltaDescription:       bodyUrlencoded.DeltaDescription,
			DeltaEnabled:           bodyUrlencoded.DeltaEnabled,
			Prev:                   bodyUrlencoded.Prev,
			Next:                   bodyUrlencoded.Next,
			CreatedAt:              bodyUrlencoded.CreatedAt,
			UpdatedAt:              bodyUrlencoded.UpdatedAt,
		}
	}

	return result, nil
}

func (s *HttpBodyUrlencodedService) TX(tx *sql.Tx) *HttpBodyUrlencodedService {
	return &HttpBodyUrlencodedService{
		q: s.q.WithTx(tx),
	}
}
