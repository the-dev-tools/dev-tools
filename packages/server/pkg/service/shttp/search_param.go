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

var ErrNoHttpSearchParamFound = errors.New("no HttpSearchParam found")

type HttpSearchParamService struct {
	queries *gen.Queries
}

func NewHttpSearchParamService(queries *gen.Queries) *HttpSearchParamService {
	return &HttpSearchParamService{
		queries: queries,
	}
}

func (s *HttpSearchParamService) TX(tx *sql.Tx) *HttpSearchParamService {
	return &HttpSearchParamService{
		queries: s.queries.WithTx(tx),
	}
}

func (s *HttpSearchParamService) Create(ctx context.Context, param *mhttp.HTTPSearchParam) error {
	dbParam := SerializeSearchParamModelToGen(*param)
	return s.queries.CreateHTTPSearchParam(ctx, dbParam)
}

func (s *HttpSearchParamService) CreateBulk(ctx context.Context, httpID idwrap.IDWrap, params []mhttp.HTTPSearchParam) error {
	if len(params) == 0 {
		return nil
	}

	// Set the httpID for all params
	for i := range params {
		params[i].HttpID = httpID
	}

	// Convert to DB types
	dbParams := tgeneric.MassConvert(params, SerializeSearchParamModelToGen)

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

func (s *HttpSearchParamService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	dbParams, err := s.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPSearchParam{}, nil
		}
		return nil, err
	}

	params := tgeneric.MassConvert(dbParams, DeserializeSearchParamGenToModel)
	return params, nil
}

func (s *HttpSearchParamService) GetByHttpIDOrdered(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	dbParams, err := s.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []mhttp.HTTPSearchParam{}, nil
		}
		return nil, err
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

	params := tgeneric.MassConvert(dbParams, DeserializeSearchParamGenToModel)
	return params, nil
}

func (s *HttpSearchParamService) GetByIDs(ctx context.Context, ids []idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	if len(ids) == 0 {
		return []mhttp.HTTPSearchParam{}, nil
	}

	rows, err := s.queries.GetHTTPSearchParamsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	params := tgeneric.MassConvert(rows, deserializeSearchParamByIDsRowToModel)
	return params, nil
}

func (s *HttpSearchParamService) GetByID(ctx context.Context, id idwrap.IDWrap) (*mhttp.HTTPSearchParam, error) {
	rows, err := s.queries.GetHTTPSearchParamsByIDs(ctx, []idwrap.IDWrap{id})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNoHttpSearchParamFound
	}

	model := deserializeSearchParamByIDsRowToModel(rows[0])
	return &model, nil
}

func (s *HttpSearchParamService) GetByHttpIDs(ctx context.Context, httpIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mhttp.HTTPSearchParam, error) {
	result := make(map[idwrap.IDWrap][]mhttp.HTTPSearchParam, len(httpIDs))
	if len(httpIDs) == 0 {
		return result, nil
	}

	for _, httpID := range httpIDs {
		rows, err := s.queries.GetHTTPSearchParams(ctx, httpID)
		if err != nil {
			if err == sql.ErrNoRows {
				result[httpID] = []mhttp.HTTPSearchParam{}
				continue
			}
			return nil, err
		}

		var params []mhttp.HTTPSearchParam
		for _, row := range rows {
			model := DeserializeSearchParamGenToModel(row)
			params = append(params, model)
		}
		result[httpID] = params
	}

	return result, nil
}

func (s *HttpSearchParamService) Update(ctx context.Context, param *mhttp.HTTPSearchParam) error {
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

func (s *HttpSearchParamService) UpdateDelta(ctx context.Context, id idwrap.IDWrap, deltaKey *string, deltaValue *string, deltaEnabled *bool, deltaDescription *string, deltaOrder *float64) error {
	return s.queries.UpdateHTTPSearchParamDelta(ctx, gen.UpdateHTTPSearchParamDeltaParams{
		DeltaKey:         stringToNull(deltaKey),
		DeltaValue:       stringToNull(deltaValue),
		DeltaDescription: deltaDescription,
		DeltaEnabled:     deltaEnabled,
		ID:               id,
	})
}

func (s *HttpSearchParamService) UpdateOrder(ctx context.Context, id idwrap.IDWrap, httpID idwrap.IDWrap, order float64) error {
	return s.queries.UpdateHTTPSearchParamOrder(ctx, gen.UpdateHTTPSearchParamOrderParams{
		Order:  order,
		ID:     id,
		HttpID: httpID,
	})
}

func (s *HttpSearchParamService) Delete(ctx context.Context, paramID idwrap.IDWrap) error {
	err := s.queries.DeleteHTTPSearchParam(ctx, paramID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoHttpSearchParamFound
		}
		return err
	}
	return nil
}

func (s *HttpSearchParamService) DeleteByHttpID(ctx context.Context, httpID idwrap.IDWrap) error {
	params, err := s.GetByHttpID(ctx, httpID)
	if err != nil {
		return err
	}

	for _, param := range params {
		if err := s.Delete(ctx, param.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *HttpSearchParamService) GetStreaming(ctx context.Context, httpIDs []idwrap.IDWrap, updatedAt int64) ([]gen.GetHTTPSearchParamsStreamingRow, error) {
	return s.queries.GetHTTPSearchParamsStreaming(ctx, gen.GetHTTPSearchParamsStreamingParams{
		HttpIds:   httpIDs,
		UpdatedAt: updatedAt,
	})
}

func (s *HttpSearchParamService) ResetDelta(ctx context.Context, id idwrap.IDWrap) error {
	// Get the current param to preserve main fields
	param, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Update the main fields to clear delta status
	err = s.Update(ctx, &mhttp.HTTPSearchParam{
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
	return s.UpdateDelta(ctx, id, nil, nil, nil, nil, nil)
}

// Conversion functions

func SerializeSearchParamModelToGen(param mhttp.HTTPSearchParam) gen.CreateHTTPSearchParamParams {
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

func DeserializeSearchParamGenToModel(dbParam gen.GetHTTPSearchParamsRow) mhttp.HTTPSearchParam {
	return mhttp.HTTPSearchParam{
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

func deserializeSearchParamByIDsRowToModel(row gen.GetHTTPSearchParamsByIDsRow) mhttp.HTTPSearchParam {
	return mhttp.HTTPSearchParam{
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

// Note: Helper functions (stringToNull, nullToString, idWrapToBytes, bytesToIDWrap)
// are defined in utils.go
