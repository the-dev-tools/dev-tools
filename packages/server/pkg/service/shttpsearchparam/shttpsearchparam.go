package shttpsearchparam

import (
	"context"
	"database/sql"
	"errors"
	"slices"

	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttpsearchparam"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHttpSearchParamFound = errors.New("no HttpSearchParam found")

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

func boolToNull(b *bool) sql.NullBool {
	if b == nil {
		return sql.NullBool{Valid: false}
	}
	return sql.NullBool{Bool: *b, Valid: true}
}

func nullToBool(nb sql.NullBool) *bool {
	if !nb.Valid {
		return nil
	}
	return &nb.Bool
}

func float64ToNull(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{Valid: false}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

func nullToFloat64(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	return &nf.Float64
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

type HttpSearchParamService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) HttpSearchParamService {
	return HttpSearchParamService{
		queries: queries,
	}
}

func (s HttpSearchParamService) TX(tx *sql.Tx) HttpSearchParamService {
	return HttpSearchParamService{
		queries: s.queries.WithTx(tx),
	}
}

func (s HttpSearchParamService) Create(ctx context.Context, param *mhttpsearchparam.HttpSearchParam) error {
	dbParam := s.modelToDB(*param)
	err := s.queries.CreateHTTPSearchParam(ctx, dbParam)
	if err != nil {
		return err
	}
	return nil
}

func (s HttpSearchParamService) CreateBulk(ctx context.Context, httpID idwrap.IDWrap, params []mhttpsearchparam.HttpSearchParam) error {
	if len(params) == 0 {
		return nil
	}

	// Set the httpID for all params
	for i := range params {
		params[i].HttpID = httpID
	}

	// Convert to DB types
	dbParams := tgeneric.MassConvert(params, s.modelToDB)

	// Chunk the inserts to avoid hitting parameter limits
	chunkSize := 100
	for i := 0; i < len(dbParams); i += chunkSize {
		end := i + chunkSize
		if end > len(dbParams) {
			end = len(dbParams)
		}

		chunk := dbParams[i:end]
		for _, param := range chunk {
			err := s.queries.CreateHTTPSearchParam(ctx, param)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s HttpSearchParamService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttpsearchparam.HttpSearchParam, error) {
	dbParams, err := s.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHttpSearchParamFound
		}
		return nil, err
	}

	if len(dbParams) == 0 {
		return nil, ErrNoHttpSearchParamFound
	}

	params := tgeneric.MassConvert(dbParams, s.dbToModel)
	return params, nil
}

func (s HttpSearchParamService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttpsearchparam.HttpSearchParam, error) {
	dbParams, err := s.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoHttpSearchParamFound
		}
		return nil, err
	}

	if len(dbParams) == 0 {
		return nil, ErrNoHttpSearchParamFound
	}

	// Sort by order field
	slices.SortFunc(dbParams, func(a, b gen.GetHTTPSearchParamsRow) int {
		if a.Order < b.Order {
			return -1
		}
		if a.Order > b.Order {
			return 1
		}
		return 0
	})

	params := tgeneric.MassConvert(dbParams, s.dbToModel)
	return params, nil
}

func (s HttpSearchParamService) Update(ctx context.Context, param *mhttpsearchparam.HttpSearchParam) error {
	err := s.queries.UpdateHTTPSearchParam(ctx, gen.UpdateHTTPSearchParamParams{
		Key:         param.Key,
		Value:       param.Value,
		Description: param.Description,
		Enabled:     param.Enabled,
		ID:          param.ID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoHttpSearchParamFound
		}
		return err
	}
	return nil
}

func (s HttpSearchParamService) Delete(ctx context.Context, paramID idwrap.IDWrap) error {
	err := s.queries.DeleteHTTPSearchParam(ctx, paramID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoHttpSearchParamFound
		}
		return err
	}
	return nil
}

func (s HttpSearchParamService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	params, err := s.GetByHttpID(ctx, httpID)
	if err != nil {
		if err == ErrNoHttpSearchParamFound {
			return nil
		}
		return err
	}

	for _, param := range params {
		if err := s.Delete(ctx, param.ID); err != nil {
			return err
		}
	}
	return nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*HttpSearchParamService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := HttpSearchParamService{queries: queries}
	return &service, nil
}

func (s HttpSearchParamService) GetHttpSearchParam(ctx context.Context, id idwrap.IDWrap) (*mhttpsearchparam.HttpSearchParam, error) {
	// Note: There's no specific GetHTTPSearchParam by ID query in the generated code
	// We'll use GetHTTPSearchParamsByIDs and take the first result
	rows, err := s.queries.GetHTTPSearchParamsByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoHttpSearchParamFound
	}

	// Convert the first row to HttpSearchParam model
	model := s.dbToModelFromByIDs(rows[0])
	return &model, nil
}

func (s HttpSearchParamService) GetHttpSearchParamsByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttpsearchparam.HttpSearchParam, error) {
	rows, err := s.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttpsearchparam.HttpSearchParam{}, nil
		}
		return nil, err
	}

	result := make([]mhttpsearchparam.HttpSearchParam, len(rows))
	for i, row := range rows {
		result[i] = s.dbToModel(row)
	}
	return result, nil
}

func (s HttpSearchParamService) GetHttpSearchParamsByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttpsearchparam.HttpSearchParam, error) {
	result := make(map[idwrap.IDWrap][]mhttpsearchparam.HttpSearchParam, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	// For each HTTP ID, get its search params
	for _, httpID := range httpIDs {
		rows, err := s.queries.GetHTTPSearchParams(ctx, httpID)
		if err != nil {
			if err == sql.ErrNoRows {
				result[httpID] = []mhttpsearchparam.HttpSearchParam{}
				continue
			}
			return nil, err
		}

		var params []mhttpsearchparam.HttpSearchParam
		for _, row := range rows {
			model := s.dbToModel(row)
			params = append(params, model)
		}
		result[httpID] = params
	}

	return result, nil
}

func (s HttpSearchParamService) CreateHttpSearchParam(ctx context.Context, param *mhttpsearchparam.HttpSearchParam) error {
	return s.Create(ctx, param)
}

func (s HttpSearchParamService) CreateBulkHttpSearchParam(ctx context.Context, params []mhttpsearchparam.HttpSearchParam) error {
	if len(params) == 0 {
		return nil
	}

	// Convert to DB types
	dbParams := tgeneric.MassConvert(params, s.modelToDB)

	// Chunk the inserts to avoid hitting parameter limits
	chunkSize := 100
	for i := 0; i < len(dbParams); i += chunkSize {
		end := i + chunkSize
		if end > len(dbParams) {
			end = len(dbParams)
		}

		chunk := dbParams[i:end]
		for _, param := range chunk {
			err := s.queries.CreateHTTPSearchParam(ctx, param)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s HttpSearchParamService) CreateHttpSearchParamRaw(ctx context.Context, param gen.CreateHTTPSearchParamParams) error {
	return s.queries.CreateHTTPSearchParam(ctx, param)
}

func (s HttpSearchParamService) UpdateHttpSearchParam(ctx context.Context, param *mhttpsearchparam.HttpSearchParam) error {
	return s.Update(ctx, param)
}

func (s HttpSearchParamService) UpdateHttpSearchParamOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float64) error {
	return s.queries.UpdateHTTPSearchParamOrder(ctx, gen.UpdateHTTPSearchParamOrderParams{
		Order:  order,
		ID:     id,
		HttpID: httpID,
	})
}

func (s HttpSearchParamService) UpdateHttpSearchParamDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float64) error {
	return s.queries.UpdateHTTPSearchParamDelta(ctx, gen.UpdateHTTPSearchParamDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               id,
	})
}

func (s HttpSearchParamService) DeleteHttpSearchParam(ctx context.Context, id idwrap.IDWrap) error {
	return s.Delete(ctx, id)
}

func (s HttpSearchParamService) ResetHttpSearchParamDelta(ctx context.Context, id idwrap.IDWrap) error {
	// Get the current param to preserve main fields
	param, err := s.GetHttpSearchParam(ctx, id)
	if err != nil {
		return err
	}

	// Update the main fields to clear delta status
	err = s.Update(ctx, &mhttpsearchparam.HttpSearchParam{
		ID:          param.ID,
		HttpID:      param.HttpID,
		Key:         param.Key,
		Value:       param.Value,
		Enabled:     param.Enabled,
		Description: param.Description,
		Order:       param.Order,
		// Clear delta fields
		ParentHttpSearchParamID: nil,
		IsDelta:                 false,
		DeltaKey:                nil,
		DeltaValue:              nil,
		DeltaEnabled:            nil,
		DeltaDescription:        nil,
		DeltaOrder:              nil,
		CreatedAt:               param.CreatedAt,
		UpdatedAt:               param.UpdatedAt,
	})
	if err != nil {
		return err
	}

	// Also clear delta fields using the delta update query
	return s.UpdateHttpSearchParamDelta(ctx, id, nil, nil, nil, nil, nil)
}

func (s HttpSearchParamService) GetHttpSearchParamsByParentID(ctx context.Context, parentID idwrap.IDWrap) ([]mhttpsearchparam.HttpSearchParam, error) {
	// Note: There's no specific query for getting by parent ID in the generated code
	// This would need to be implemented using GetHTTPSearchParams and filtering
	// or by adding a new query to the SQL schema
	// For now, return empty slice as this is a specialized query
	return []mhttpsearchparam.HttpSearchParam{}, nil
}

func (s HttpSearchParamService) GetHttpSearchParamStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]gen.GetHTTPSearchParamsStreamingRow, error) {
	return s.queries.GetHTTPSearchParamsStreaming(ctx, gen.GetHTTPSearchParamsStreamingParams{
		HttpIds:   httpIDs,
		UpdatedAt: updatedAt,
	})
}

// Conversion functions

func (s HttpSearchParamService) modelToDB(param mhttpsearchparam.HttpSearchParam) gen.CreateHTTPSearchParamParams {
	return gen.CreateHTTPSearchParamParams{
		ID:                      param.ID,
		HttpID:                  param.HttpID,
		Key:                     param.Key,
		Value:                   param.Value,
		Description:             param.Description,
		Enabled:                 param.Enabled,
		Order:                   param.Order,
		ParentHttpSearchParamID: idWrapToBytes(param.ParentHttpSearchParamID),
		IsDelta:                 param.IsDelta,
		DeltaKey:                stringToNull(param.DeltaKey),
		DeltaValue:              stringToNull(param.DeltaValue),
		DeltaDescription:        param.DeltaDescription,
		DeltaEnabled:            param.DeltaEnabled,
		CreatedAt:               param.CreatedAt,
		UpdatedAt:               param.UpdatedAt,
	}
}

func (s HttpSearchParamService) dbToModel(dbParam gen.GetHTTPSearchParamsRow) mhttpsearchparam.HttpSearchParam {
	return mhttpsearchparam.HttpSearchParam{
		ID:                      dbParam.ID,
		HttpID:                  dbParam.HttpID,
		Key:                     dbParam.Key,
		Value:                   dbParam.Value,
		Description:             dbParam.Description,
		Enabled:                 dbParam.Enabled,
		Order:                   dbParam.Order,
		ParentHttpSearchParamID: bytesToIDWrap(dbParam.ParentHttpSearchParamID),
		IsDelta:                 dbParam.IsDelta,
		DeltaKey:                nullToString(dbParam.DeltaKey),
		DeltaValue:              nullToString(dbParam.DeltaValue),
		DeltaEnabled:            dbParam.DeltaEnabled,
		DeltaDescription:        dbParam.DeltaDescription,
		DeltaOrder:              &dbParam.Order,
		CreatedAt:               dbParam.CreatedAt,
		UpdatedAt:               dbParam.UpdatedAt,
	}
}

func (s HttpSearchParamService) dbToModelFromByIDs(row gen.GetHTTPSearchParamsByIDsRow) mhttpsearchparam.HttpSearchParam {
	return mhttpsearchparam.HttpSearchParam{
		ID:                      row.ID,
		HttpID:                  row.HttpID,
		Key:                     row.Key,
		Value:                   row.Value,
		Description:             row.Description,
		Enabled:                 row.Enabled,
		Order:                   row.Order,
		ParentHttpSearchParamID: bytesToIDWrap(row.ParentHttpSearchParamID),
		IsDelta:                 row.IsDelta,
		DeltaKey:                nullToString(row.DeltaKey),
		DeltaValue:              nullToString(row.DeltaValue),
		DeltaEnabled:            row.DeltaEnabled,
		DeltaDescription:        row.DeltaDescription,
		DeltaOrder:              nil, // Not available in ByIDs row
		CreatedAt:               row.CreatedAt,
		UpdatedAt:               row.UpdatedAt,
	}
}
