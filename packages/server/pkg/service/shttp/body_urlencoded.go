//nolint:revive // exported
package shttp

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHttpBodyUrlEncodedFound = errors.New("no http body url encoded found")

type HttpBodyUrlEncodedService struct {
	queries *gen.Queries
}

func NewHttpBodyUrlEncodedService(queries *gen.Queries) *HttpBodyUrlEncodedService {
	return &HttpBodyUrlEncodedService{queries: queries}
}

func (s *HttpBodyUrlEncodedService) TX(tx *sql.Tx) *HttpBodyUrlEncodedService {
	return &HttpBodyUrlEncodedService{queries: s.queries.WithTx(tx)}
}

func (s *HttpBodyUrlEncodedService) Create(ctx context.Context, body *mhttp.HTTPBodyUrlencoded) error {
	bue := SerializeBodyUrlEncodedModelToGen(*body)
	return s.queries.CreateHTTPBodyUrlEncoded(ctx, gen.CreateHTTPBodyUrlEncodedParams{
		ID:                         bue.ID,
		HttpID:                     bue.HttpID,
		Key:                        bue.Key,
		Value:                      bue.Value,
		Description:                bue.Description,
		Enabled:                    bue.Enabled,
		Order:                      bue.Order,
		ParentHttpBodyUrlencodedID: bue.ParentHttpBodyUrlencodedID,
		IsDelta:                    bue.IsDelta,
		DeltaKey:                   bue.DeltaKey,
		DeltaValue:                 bue.DeltaValue,
		DeltaDescription:           bue.DeltaDescription,
		DeltaEnabled:               bue.DeltaEnabled,
		CreatedAt:                  bue.CreatedAt,
		UpdatedAt:                  bue.UpdatedAt,
	})
}

func (s *HttpBodyUrlEncodedService) CreateBulk(ctx context.Context, bodyUrlEncodeds []mhttp.HTTPBodyUrlencoded) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyUrlEncodeds, SerializeBodyUrlEncodedModelToGen)

	for bodyUrlEncodedChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, bodyUrlEncoded := range bodyUrlEncodedChunk {
			err := s.createRaw(ctx, bodyUrlEncoded)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *HttpBodyUrlEncodedService) createRaw(ctx context.Context, bue gen.HttpBodyUrlencoded) error {
	return s.queries.CreateHTTPBodyUrlEncoded(ctx, gen.CreateHTTPBodyUrlEncodedParams{
		ID:                         bue.ID,
		HttpID:                     bue.HttpID,
		Key:                        bue.Key,
		Value:                      bue.Value,
		Description:                bue.Description,
		Enabled:                    bue.Enabled,
		Order:                      bue.Order,
		ParentHttpBodyUrlencodedID: bue.ParentHttpBodyUrlencodedID,
		IsDelta:                    bue.IsDelta,
		DeltaKey:                   bue.DeltaKey,
		DeltaValue:                 bue.DeltaValue,
		DeltaDescription:           bue.DeltaDescription,
		DeltaEnabled:               bue.DeltaEnabled,
		CreatedAt:                  bue.CreatedAt,
		UpdatedAt:                  bue.UpdatedAt,
	})
}

func (s *HttpBodyUrlEncodedService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyUrlencoded, error) {
	body, err := s.queries.GetHTTPBodyUrlEncoded(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHttpBodyUrlEncodedFound
		}
		return nil, err
	}

	model := DeserializeBodyUrlEncodedGenToModel(body)
	return &model, nil
}

func (s *HttpBodyUrlEncodedService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	bodies, err := s.queries.GetHTTPBodyUrlEncodedByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyUrlencoded{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPBodyUrlencoded, len(bodies))
	for i, body := range bodies {
		result[i] = DeserializeBodyUrlEncodedGenToModel(body)
	}
	return result, nil
}

func (s *HttpBodyUrlEncodedService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	rows, err := s.queries.GetHTTPBodyUrlEncodedByHttpID(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyUrlencoded{}, nil
		}
		return nil, err
	}

	// Sort by order field
	slices.SortFunc(rows, func(a, b gen.HttpBodyUrlencoded) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return 0
	})

	result := make([]mhttp.HTTPBodyUrlencoded, len(rows))
	for i, row := range rows {
		result[i] = DeserializeBodyUrlEncodedGenToModel(row)
	}
	return result, nil
}

func (s *HttpBodyUrlEncodedService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPBodyUrlencoded, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPBodyUrlencoded{}, nil
	}

	bodies, err := s.queries.GetHTTPBodyUrlEncodedsByIDs(ctx, ids)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPBodyUrlencoded{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPBodyUrlencoded, len(bodies))
	for i, body := range bodies {
		result[i] = DeserializeBodyUrlEncodedGenToModel(body)
	}
	return result, nil
}

func (s *HttpBodyUrlEncodedService) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded, error) {
	result := make(map[idwrap.IDWrap][]mhttp.HTTPBodyUrlencoded, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	bodies, err := s.queries.GetHTTPBodyUrlEncodedsByIDs(ctx, httpIDs)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return result, nil
		}
		return nil, err
	}

	for _, body := range bodies {
		model := DeserializeBodyUrlEncodedGenToModel(body)
		httpID := model.HttpID
		result[httpID] = append(result[httpID], model)
	}

	return result, nil
}

func (s *HttpBodyUrlEncodedService) Update(ctx context.Context, body *mhttp.HTTPBodyUrlencoded) error {
	return s.queries.UpdateHTTPBodyUrlEncoded(ctx, gen.UpdateHTTPBodyUrlEncodedParams{
		Key:         body.Key,
		Value:       body.Value,
		Description: body.Description,
		Enabled:     body.Enabled,
		Order:       float64(body.Order),
		ID:          body.ID,
	})
}

func (s *HttpBodyUrlEncodedService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return s.queries.UpdateHTTPBodyUrlEncodedDelta(ctx, gen.UpdateHTTPBodyUrlEncodedDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               id,
	})
}

func (s *HttpBodyUrlEncodedService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.DeleteHTTPBodyUrlEncoded(ctx, id)
}

func (s *HttpBodyUrlEncodedService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	bodies, err := s.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, body := range bodies {
		if err := s.Delete(ctx, body.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *HttpBodyUrlEncodedService) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	// Reset delta fields by setting them to nil
	return s.UpdateDelta(ctx, id, nil, nil, nil, nil, nil)
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
		Order:                      float64(body.Order),
		ParentHttpBodyUrlencodedID: idWrapToBytes(body.ParentHttpBodyUrlEncodedID),
		IsDelta:                    body.IsDelta,
		DeltaKey:                   stringToNull(body.DeltaKey),
		DeltaValue:                 stringToNull(body.DeltaValue),
		DeltaEnabled:               body.DeltaEnabled,
		DeltaDescription:           body.DeltaDescription,
		DeltaOrder:                 float32ToNullFloat64UrlEncoded(body.DeltaOrder),
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
		Order:                      float32(body.Order),
		ParentHttpBodyUrlEncodedID: bytesToIDWrap(body.ParentHttpBodyUrlencodedID),
		IsDelta:                    body.IsDelta,
		DeltaKey:                   nullToString(body.DeltaKey),
		DeltaValue:                 nullToString(body.DeltaValue),
		DeltaEnabled:               body.DeltaEnabled,
		DeltaDescription:           body.DeltaDescription,
		DeltaOrder:                 nullFloat64ToFloat32UrlEncoded(body.DeltaOrder),
		CreatedAt:                  body.CreatedAt,
		UpdatedAt:                  body.UpdatedAt,
	}
}
