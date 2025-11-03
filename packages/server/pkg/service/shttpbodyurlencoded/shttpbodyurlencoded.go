package shttpbodyurlencoded

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttpbodyurlencoded"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

// Utility functions for null handling
func stringToNull(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullToString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

func float32ToNull(f *float32) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: float64(*f), Valid: true}
}

func nullToFloat32(nf sql.NullFloat64) *float32 {
	if !nf.Valid {
		return nil
	}
	f := float32(nf.Float64)
	return &f
}

func idWrapToBytes(id *idwrap.IDWrap) []byte {
	if id == nil {
		return nil
	}
	return id.Bytes()
}

func bytesToIDWrap(b []byte) *idwrap.IDWrap {
	if b == nil {
		return nil
	}
	id, err := idwrap.NewFromBytes(b)
	if err != nil {
		return nil
	}
	return &id
}

type HttpBodyUrlEncodedService struct {
	queries *gen.Queries
}

var ErrNoHttpBodyUrlEncodedFound = errors.New("no http body url encoded found")

func SerializeModelToGen(body mhttpbodyurlencoded.HttpBodyUrlEncoded) gen.HttpBodyUrlencoded {
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
		DeltaOrder:                 float32ToNull(body.DeltaOrder),
		CreatedAt:                  body.CreatedAt,
		UpdatedAt:                  body.UpdatedAt,
	}
}

func DeserializeGenToModel(body gen.HttpBodyUrlencoded) mhttpbodyurlencoded.HttpBodyUrlEncoded {
	return mhttpbodyurlencoded.HttpBodyUrlEncoded{
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
		DeltaOrder:                 nullToFloat32(body.DeltaOrder),
		CreatedAt:                  body.CreatedAt,
		UpdatedAt:                  body.UpdatedAt,
	}
}

func New(queries *gen.Queries) HttpBodyUrlEncodedService {
	return HttpBodyUrlEncodedService{queries: queries}
}

func (hues HttpBodyUrlEncodedService) TX(tx *sql.Tx) HttpBodyUrlEncodedService {
	return HttpBodyUrlEncodedService{queries: hues.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*HttpBodyUrlEncodedService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := HttpBodyUrlEncodedService{queries: queries}
	return &service, nil
}

func (hues HttpBodyUrlEncodedService) GetHttpBodyUrlEncoded(ctx context.Context, id idwrap.IDWrap) (*mhttpbodyurlencoded.HttpBodyUrlEncoded, error) {
	body, err := hues.queries.GetHTTPBodyUrlEncoded(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoHttpBodyUrlEncodedFound
		}
		return nil, err
	}

	model := DeserializeGenToModel(body)
	return &model, nil
}

func (hues HttpBodyUrlEncodedService) GetHttpBodyUrlEncodedByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttpbodyurlencoded.HttpBodyUrlEncoded, error) {
	bodies, err := hues.queries.GetHTTPBodyUrlEncodedByHttpID(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttpbodyurlencoded.HttpBodyUrlEncoded{}, nil
		}
		return nil, err
	}

	result := make([]mhttpbodyurlencoded.HttpBodyUrlEncoded, len(bodies))
	for i, body := range bodies {
		result[i] = DeserializeGenToModel(body)
	}
	return result, nil
}

func (hues HttpBodyUrlEncodedService) GetHttpBodyUrlEncodedsByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttpbodyurlencoded.HttpBodyUrlEncoded, error) {
	if len(ids) == 0 {
		return []mhttpbodyurlencoded.HttpBodyUrlEncoded{}, nil
	}

	bodies, err := hues.queries.GetHTTPBodyUrlEncodedsByIDs(ctx, ids)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttpbodyurlencoded.HttpBodyUrlEncoded{}, nil
		}
		return nil, err
	}

	result := make([]mhttpbodyurlencoded.HttpBodyUrlEncoded, len(bodies))
	for i, body := range bodies {
		result[i] = DeserializeGenToModel(body)
	}
	return result, nil
}

func (hues HttpBodyUrlEncodedService) CreateHttpBodyUrlEncoded(ctx context.Context, body *mhttpbodyurlencoded.HttpBodyUrlEncoded) error {
	bue := SerializeModelToGen(*body)
	return hues.queries.CreateHTTPBodyUrlEncoded(ctx, gen.CreateHTTPBodyUrlEncodedParams{
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

func (hues HttpBodyUrlEncodedService) CreateBulkHttpBodyUrlEncoded(ctx context.Context, bodyUrlEncodeds []mhttpbodyurlencoded.HttpBodyUrlEncoded) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyUrlEncodeds, SerializeModelToGen)

	for bodyUrlEncodedChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, bodyUrlEncoded := range bodyUrlEncodedChunk {
			err := hues.CreateHttpBodyUrlEncodedRaw(ctx, bodyUrlEncoded)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (hues HttpBodyUrlEncodedService) CreateHttpBodyUrlEncodedRaw(ctx context.Context, bue gen.HttpBodyUrlencoded) error {
	return hues.queries.CreateHTTPBodyUrlEncoded(ctx, gen.CreateHTTPBodyUrlEncodedParams{
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

func (hues HttpBodyUrlEncodedService) UpdateHttpBodyUrlEncoded(ctx context.Context, body *mhttpbodyurlencoded.HttpBodyUrlEncoded) error {
	return hues.queries.UpdateHTTPBodyUrlEncoded(ctx, gen.UpdateHTTPBodyUrlEncodedParams{
		Key:         body.Key,
		Value:       body.Value,
		Description: body.Description,
		Enabled:     body.Enabled,
		ID:          body.ID,
	})
}

func (hues HttpBodyUrlEncodedService) UpdateHttpBodyUrlEncodedDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return hues.queries.UpdateHTTPBodyUrlEncodedDelta(ctx, gen.UpdateHTTPBodyUrlEncodedDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		DeltaOrder:       float32ToNull(deltaOrder),
		ID:               id,
	})
}

func (hues HttpBodyUrlEncodedService) DeleteHttpBodyUrlEncoded(ctx context.Context, id idwrap.IDWrap) error {
	return hues.queries.DeleteHTTPBodyUrlEncoded(ctx, id)
}

func (hues HttpBodyUrlEncodedService) ResetHttpBodyUrlEncodedDelta(ctx context.Context, id idwrap.IDWrap) error {
	bodyUrlEncoded, err := hues.GetHttpBodyUrlEncoded(ctx, id)
	if err != nil {
		return err
	}

	bodyUrlEncoded.ParentHttpBodyUrlEncodedID = nil
	bodyUrlEncoded.IsDelta = false
	bodyUrlEncoded.DeltaKey = nil
	bodyUrlEncoded.DeltaValue = nil
	bodyUrlEncoded.DeltaEnabled = nil
	bodyUrlEncoded.DeltaDescription = nil
	bodyUrlEncoded.DeltaOrder = nil

	return hues.UpdateHttpBodyUrlEncodedDelta(ctx, id, bodyUrlEncoded.DeltaKey, bodyUrlEncoded.DeltaValue, bodyUrlEncoded.DeltaEnabled, bodyUrlEncoded.DeltaDescription, bodyUrlEncoded.DeltaOrder)
}

// Note: UpdateHttpBodyUrlEncodedOrder is not available in the generated queries
// Order updates would need to be handled through the main UpdateHttpBodyUrlEncoded method
// or by adding a new query to the SQL schema if needed

func (hues HttpBodyUrlEncodedService) GetHttpBodyUrlEncodedsByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mhttpbodyurlencoded.HttpBodyUrlEncoded, error) {
	// Note: There's no specific query for getting by parent ID in the generated code
	// This would need to be implemented using GetHTTPBodyUrlEncodedByHttpID and filtering
	// or by adding a new query to the SQL schema
	// For now, return empty slice as this is a specialized query
	return []mhttpbodyurlencoded.HttpBodyUrlEncoded{}, nil
}
