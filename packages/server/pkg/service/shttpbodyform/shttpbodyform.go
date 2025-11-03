package shttpbodyform

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttpbodyform"
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

type HttpBodyFormService struct {
	queries *gen.Queries
}

var ErrNoHttpBodyFormFound = errors.New("no http body form found")

func SerializeModelToGen(body mhttpbodyform.HttpBodyForm) gen.HttpBodyForm {
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
		DeltaOrder:           float32ToNull(body.DeltaOrder),
		CreatedAt:            body.CreatedAt,
		UpdatedAt:            body.UpdatedAt,
	}
}

func DeserializeGenToModel(body gen.HttpBodyForm) mhttpbodyform.HttpBodyForm {
	return mhttpbodyform.HttpBodyForm{
		ID:                   body.ID,
		HttpID:               body.HttpID,
		Key:                  body.Key,
		Value:                body.Value,
		Enabled:              body.Enabled,
		Description:          body.Description,
		Order:                float32(body.Order),
		ParentHttpBodyFormID: bytesToIDWrap(body.ParentHttpBodyFormID),
		IsDelta:              body.IsDelta,
		DeltaKey:             nullToString(body.DeltaKey),
		DeltaValue:           nullToString(body.DeltaValue),
		DeltaEnabled:         body.DeltaEnabled,
		DeltaDescription:     body.DeltaDescription,
		DeltaOrder:           nullToFloat32(body.DeltaOrder),
		CreatedAt:            body.CreatedAt,
		UpdatedAt:            body.UpdatedAt,
	}
}

func DeserializeGetRowToModel(row gen.GetHTTPBodyFormsRow) mhttpbodyform.HttpBodyForm {
	return mhttpbodyform.HttpBodyForm{
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
		DeltaOrder:           nullToFloat32(sql.NullFloat64{}), // Order is not in delta row
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}

func DeserializeGetByIDsRowToModel(row gen.GetHTTPBodyFormsByIDsRow) mhttpbodyform.HttpBodyForm {
	return mhttpbodyform.HttpBodyForm{
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
		DeltaOrder:           nullToFloat32(sql.NullFloat64{}), // Order is not in delta row
		CreatedAt:            row.CreatedAt,
		UpdatedAt:            row.UpdatedAt,
	}
}

func New(queries *gen.Queries) HttpBodyFormService {
	return HttpBodyFormService{queries: queries}
}

func (hfs HttpBodyFormService) TX(tx *sql.Tx) HttpBodyFormService {
	return HttpBodyFormService{queries: hfs.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*HttpBodyFormService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := HttpBodyFormService{queries: queries}
	return &service, nil
}

func (hfs HttpBodyFormService) GetHttpBodyForm(ctx context.Context, id idwrap.IDWrap) (*mhttpbodyform.HttpBodyForm, error) {
	// Note: There's no specific GetHTTPBodyForm by ID query in the generated code
	// We'll use GetHTTPBodyFormsByIDs and take the first result
	rows, err := hfs.queries.GetHTTPBodyFormsByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoHttpBodyFormFound
	}

	// Convert the first row to HttpBodyForm model
	body := gen.HttpBodyForm{
		ID:                   rows[0].ID,
		HttpID:               rows[0].HttpID,
		Key:                  rows[0].Key,
		Value:                rows[0].Value,
		Enabled:              rows[0].Enabled,
		Description:          rows[0].Description,
		Order:                rows[0].Order,
		ParentHttpBodyFormID: rows[0].ParentHttpBodyFormID,
		IsDelta:              rows[0].IsDelta,
		DeltaKey:             rows[0].DeltaKey,
		DeltaValue:           rows[0].DeltaValue,
		DeltaEnabled:         rows[0].DeltaEnabled,
		DeltaDescription:     rows[0].DeltaDescription,
		DeltaOrder:           sql.NullFloat64{}, // Not available in row
		CreatedAt:            rows[0].CreatedAt,
		UpdatedAt:            rows[0].UpdatedAt,
	}

	model := DeserializeGenToModel(body)
	return &model, nil
}

func (hfs HttpBodyFormService) GetHttpBodyFormsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttpbodyform.HttpBodyForm, error) {
	rows, err := hfs.queries.GetHTTPBodyForms(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttpbodyform.HttpBodyForm{}, nil
		}
		return nil, err
	}

	result := make([]mhttpbodyform.HttpBodyForm, len(rows))
	for i, row := range rows {
		result[i] = DeserializeGetRowToModel(row)
	}
	return result, nil
}

func (hfs HttpBodyFormService) GetHttpBodyFormsByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttpbodyform.HttpBodyForm, error) {
	result := make(map[idwrap.IDWrap][]mhttpbodyform.HttpBodyForm, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	rows, err := hfs.queries.GetHTTPBodyFormsByIDs(ctx, httpIDs)
	if err != nil {
		if err == sql.ErrNoRows {
			return result, nil
		}
		return nil, err
	}

	for _, row := range rows {
		model := DeserializeGetByIDsRowToModel(row)
		httpID := model.HttpID
		result[httpID] = append(result[httpID], model)
	}

	return result, nil
}

func (hfs HttpBodyFormService) CreateHttpBodyForm(ctx context.Context, body *mhttpbodyform.HttpBodyForm) error {
	bf := SerializeModelToGen(*body)
	return hfs.queries.CreateHTTPBodyForm(ctx, gen.CreateHTTPBodyFormParams{
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

func (hfs HttpBodyFormService) CreateBulkHttpBodyForm(ctx context.Context, bodyForms []mhttpbodyform.HttpBodyForm) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(bodyForms, SerializeModelToGen)

	for bodyFormChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		for _, bodyForm := range bodyFormChunk {
			err := hfs.CreateHttpBodyFormRaw(ctx, bodyForm)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (hfs HttpBodyFormService) CreateHttpBodyFormRaw(ctx context.Context, bf gen.HttpBodyForm) error {
	return hfs.queries.CreateHTTPBodyForm(ctx, gen.CreateHTTPBodyFormParams{
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

func (hfs HttpBodyFormService) UpdateHttpBodyForm(ctx context.Context, body *mhttpbodyform.HttpBodyForm) error {
	return hfs.queries.UpdateHTTPBodyForm(ctx, gen.UpdateHTTPBodyFormParams{
		Key:         body.Key,
		Value:       body.Value,
		Description: body.Description,
		Enabled:     body.Enabled,
		ID:          body.ID,
	})
}

func (hfs HttpBodyFormService) UpdateHttpBodyFormOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float32) error {
	return hfs.queries.UpdateHTTPBodyFormOrder(ctx, gen.UpdateHTTPBodyFormOrderParams{
		Order:  float64(order),
		ID:     id,
		HttpID: httpID,
	})
}

func (hfs HttpBodyFormService) UpdateHttpBodyFormDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float32) error {
	return hfs.queries.UpdateHTTPBodyFormDelta(ctx, gen.UpdateHTTPBodyFormDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               id,
	})
}

func (hfs HttpBodyFormService) DeleteHttpBodyForm(ctx context.Context, id idwrap.IDWrap) error {
	return hfs.queries.DeleteHTTPBodyForm(ctx, id)
}

func (hfs HttpBodyFormService) ResetHttpBodyFormDelta(ctx context.Context, id idwrap.IDWrap) error {
	bodyForm, err := hfs.GetHttpBodyForm(ctx, id)
	if err != nil {
		return err
	}

	bodyForm.ParentHttpBodyFormID = nil
	bodyForm.IsDelta = false
	bodyForm.DeltaKey = nil
	bodyForm.DeltaValue = nil
	bodyForm.DeltaEnabled = nil
	bodyForm.DeltaDescription = nil
	bodyForm.DeltaOrder = nil

	return hfs.UpdateHttpBodyFormDelta(ctx, id, bodyForm.DeltaKey, bodyForm.DeltaValue, bodyForm.DeltaEnabled, bodyForm.DeltaDescription, bodyForm.DeltaOrder)
}

func (hfs HttpBodyFormService) GetHttpBodyFormsByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mhttpbodyform.HttpBodyForm, error) {
	// Note: There's no specific query for getting by parent ID in the generated code
	// This would need to be implemented using GetHTTPBodyForms and filtering
	// or by adding a new query to the SQL schema
	// For now, return empty slice as this is a specialized query
	return []mhttpbodyform.HttpBodyForm{}, nil
}

func (hfs HttpBodyFormService) GetHttpBodyFormStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]gen.GetHTTPBodyFormStreamingRow, error) {
	return hfs.queries.GetHTTPBodyFormStreaming(ctx, gen.GetHTTPBodyFormStreamingParams{
		HttpIds:   httpIDs,
		UpdatedAt: updatedAt,
	})
}
