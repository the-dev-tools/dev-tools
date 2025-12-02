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

var ErrNoHttpBodyFormFound = errors.New("no http body form found")

type HttpBodyFormService struct {
	queries *gen.Queries
}

func NewHttpBodyFormService(queries *gen.Queries) *HttpBodyFormService {
	return &HttpBodyFormService{queries: queries}
}

func (s *HttpBodyFormService) TX(tx *sql.Tx) *HttpBodyFormService {
	return &HttpBodyFormService{queries: s.queries.WithTx(tx)}
}

func (s *HttpBodyFormService) Create(ctx context.Context, body *mhttp.HTTPBodyForm) error {
	bf := SerializeBodyFormModelToGen(*body)
	return s.queries.CreateHTTPBodyForm(ctx, gen.CreateHTTPBodyFormParams{
		ID:                   bf.ID,
		HttpID:               bf.HttpID,
		Key:                  bf.Key,
		Value:                bf.Value,
		Description:          bf.Description,
		Enabled:              bf.Enabled,
		Order:                bf.Order,
		ParentHttpBodyFormID: bf.ParentHttpBodyFormID,
		IsDelta:              bf.IsDelta,
		DeltaKey:             bf.DeltaKey,
		DeltaValue:           bf.DeltaValue,
		DeltaDescription:     bf.DeltaDescription,
		DeltaEnabled:         bf.DeltaEnabled,
		CreatedAt:            bf.CreatedAt,
		UpdatedAt:            bf.UpdatedAt,
	})
}

func (s *HttpBodyFormService) CreateBulk(ctx context.Context, bodyForms []mhttp.HTTPBodyForm) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyForms, SerializeBodyFormModelToGen)

	for bodyFormChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, bodyForm := range bodyFormChunk {
			err := s.createRaw(ctx, bodyForm)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *HttpBodyFormService) createRaw(ctx context.Context, bf gen.HttpBodyForm) error {
	return s.queries.CreateHTTPBodyForm(ctx, gen.CreateHTTPBodyFormParams{
		ID:                   bf.ID,
		HttpID:               bf.HttpID,
		Key:                  bf.Key,
		Value:                bf.Value,
		Description:          bf.Description,
		Enabled:              bf.Enabled,
		Order:                bf.Order,
		ParentHttpBodyFormID: bf.ParentHttpBodyFormID,
		IsDelta:              bf.IsDelta,
		DeltaKey:             bf.DeltaKey,
		DeltaValue:           bf.DeltaValue,
		DeltaDescription:     bf.DeltaDescription,
		DeltaEnabled:         bf.DeltaEnabled,
		CreatedAt:            bf.CreatedAt,
		UpdatedAt:            bf.UpdatedAt,
	})
}

func (s *HttpBodyFormService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPBodyForm, error) {
	rows, err := s.queries.GetHTTPBodyFormsByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoHttpBodyFormFound
	}

	model := deserializeBodyFormByIDsRowToModel(rows[0])
	return &model, nil
}

func (s *HttpBodyFormService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	rows, err := s.queries.GetHTTPBodyForms(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPBodyForm{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPBodyForm, len(rows))
	for i, row := range rows {
		result[i] = DeserializeBodyFormGenToModel(row)
	}
	return result, nil
}

func (s *HttpBodyFormService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	rows, err := s.queries.GetHTTPBodyForms(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPBodyForm{}, nil
		}
		return nil, err
	}

	// Sort by order field
	slices.SortFunc(rows, func(a, b gen.GetHTTPBodyFormsRow) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return 0
	})

	result := make([]mhttp.HTTPBodyForm, len(rows))
	for i, row := range rows {
		result[i] = DeserializeBodyFormGenToModel(row)
	}
	return result, nil
}

func (s *HttpBodyFormService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPBodyForm, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPBodyForm{}, nil
	}

	rows, err := s.queries.GetHTTPBodyFormsByIDs(ctx, ids)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPBodyForm{}, nil
		}
		return nil, err
	}

	return tgeneric.MassConvert(rows, deserializeBodyFormByIDsRowToModel), nil
}

func (s *HttpBodyFormService) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPBodyForm, error) {
	result := make(map[idwrap.IDWrap][]mhttp.HTTPBodyForm, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	rows, err := s.queries.GetHTTPBodyFormsByIDs(ctx, httpIDs)
	if err != nil {
		if err == sql.ErrNoRows {
			return result, nil
		}
		return nil, err
	}

	for _, row := range rows {
		model := deserializeBodyFormByIDsRowToModel(row)
		httpID := model.HttpID
		result[httpID] = append(result[httpID], model)
	}

	return result, nil
}

func (s *HttpBodyFormService) Update(ctx context.Context, body *mhttp.HTTPBodyForm) error {
	return s.queries.UpdateHTTPBodyForm(ctx, gen.UpdateHTTPBodyFormParams{
		Key:         body.Key,
		Value:       body.Value,
		Description: body.Description,
		Enabled:     body.Enabled,
		Order:       float64(body.Order),
		ID:          body.ID,
	})
}

func (s *HttpBodyFormService) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float32) error {
	return s.queries.UpdateHTTPBodyFormOrder(ctx, gen.UpdateHTTPBodyFormOrderParams{
		Order:  float64(order),
		ID:     id,
		HttpID: httpID,
	})
}

func (s *HttpBodyFormService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return s.queries.UpdateHTTPBodyFormDelta(ctx, gen.UpdateHTTPBodyFormDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               id,
	})
}

func (s *HttpBodyFormService) Delete(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.DeleteHTTPBodyForm(ctx, id)
}

func (s *HttpBodyFormService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	forms, err := s.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, form := range forms {
		if err := s.Delete(ctx, form.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *HttpBodyFormService) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	return s.queries.ResetHTTPBodyFormDelta(ctx, id)
}

func (s *HttpBodyFormService) GetStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]gen.GetHTTPBodyFormStreamingRow, error) {
	return s.queries.GetHTTPBodyFormStreaming(ctx, gen.GetHTTPBodyFormStreamingParams{
		HttpIds:   httpIDs,
		UpdatedAt: updatedAt,
	})
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
		Order:                float64(body.Order),
		ParentHttpBodyFormID: idWrapToBytes(body.ParentHttpBodyFormID),
		IsDelta:              body.IsDelta,
		DeltaKey:             stringToNull(body.DeltaKey),
		DeltaValue:           stringToNull(body.DeltaValue),
		DeltaEnabled:         body.DeltaEnabled,
		DeltaDescription:     body.DeltaDescription,
		DeltaOrder:           float32ToNullFloat64(body.DeltaOrder),
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
		Order:                float32(row.Order),
		ParentHttpBodyFormID: bytesToIDWrap(row.ParentHttpBodyFormID),
		IsDelta:              row.IsDelta,
		DeltaKey:             nullToString(row.DeltaKey),
		DeltaValue:           nullToString(row.DeltaValue),
		DeltaEnabled:         row.DeltaEnabled,
		DeltaDescription:     row.DeltaDescription,
		DeltaOrder:           nil, // Not available in row
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
		Order:                float32(row.Order),
		ParentHttpBodyFormID: bytesToIDWrap(row.ParentHttpBodyFormID),
		IsDelta:              row.IsDelta,
		DeltaKey:             nullToString(row.DeltaKey),
		DeltaValue:           nullToString(row.DeltaValue),
		DeltaEnabled:         row.DeltaEnabled,
		DeltaDescription:     row.DeltaDescription,
		DeltaOrder:           nil, // Not available in row
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}
