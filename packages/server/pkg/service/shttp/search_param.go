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

var ErrNoHttpSearchParamFound = errors.New("no HTTP search param found")

type HttpSearchParamService struct {
	queries *gen.Queries
}

func ConvertToDBHttpSearchParam(param mhttp.HTTPSearchParam) gen.HttpSearchParam {
	var parentID []byte
	if param.ParentSearchParamID != nil {
		parentID = param.ParentSearchParamID.Bytes()
	}

	var deltaKey sql.NullString
	if param.DeltaParamKey != nil {
		deltaKey = sql.NullString{String: *param.DeltaParamKey, Valid: true}
	}

	var deltaValue sql.NullString
	if param.DeltaParamValue != nil {
		deltaValue = sql.NullString{String: *param.DeltaParamValue, Valid: true}
	}

	return gen.HttpSearchParam{
		ID:                      param.ID,
		HttpID:                  param.HttpID,
		Key:                     param.ParamKey,
		Value:                   param.ParamValue,
		Description:             param.Description,
		Enabled:                 param.Enabled,
		ParentHttpSearchParamID: parentID,
		IsDelta:                 param.IsDelta,
		DeltaKey:                deltaKey,
		DeltaValue:              deltaValue,
		DeltaEnabled:            param.DeltaEnabled,
		DeltaDescription:        param.DeltaDescription,
		DeltaOrder:              sql.NullFloat64{}, // No order delta in model
		CreatedAt:               param.CreatedAt,
		UpdatedAt:               param.UpdatedAt,
	}
}

func ConvertToModelHttpSearchParam(param gen.HttpSearchParam) mhttp.HTTPSearchParam {
	var parentID *idwrap.IDWrap
	if len(param.ParentHttpSearchParamID) > 0 {
		wrappedID := idwrap.NewFromBytesMust(param.ParentHttpSearchParamID)
		parentID = &wrappedID
	}

	var deltaKey *string
	if param.DeltaKey.Valid {
		deltaKey = &param.DeltaKey.String
	}

	var deltaValue *string
	if param.DeltaValue.Valid {
		deltaValue = &param.DeltaValue.String
	}

	return mhttp.HTTPSearchParam{
		ID:                  param.ID,
		HttpID:              param.HttpID,
		ParamKey:            param.Key,
		ParamValue:          param.Value,
		Description:         param.Description,
		Enabled:             param.Enabled,
		ParentSearchParamID: parentID,
		IsDelta:             param.IsDelta,
		DeltaParamKey:       deltaKey,
		DeltaParamValue:     deltaValue,
		DeltaDescription:    param.DeltaDescription,
		DeltaEnabled:        param.DeltaEnabled,
		CreatedAt:           param.CreatedAt,
		UpdatedAt:           param.UpdatedAt,
	}
}

func ConvertRowToModelHttpSearchParam(param gen.GetHTTPSearchParamsRow) mhttp.HTTPSearchParam {
	var parentID *idwrap.IDWrap
	if len(param.ParentHttpSearchParamID) > 0 {
		wrappedID := idwrap.NewFromBytesMust(param.ParentHttpSearchParamID)
		parentID = &wrappedID
	}

	var deltaKey *string
	if param.DeltaKey.Valid {
		deltaKey = &param.DeltaKey.String
	}

	var deltaValue *string
	if param.DeltaValue.Valid {
		deltaValue = &param.DeltaValue.String
	}

	return mhttp.HTTPSearchParam{
		ID:                  param.ID,
		HttpID:              param.HttpID,
		ParamKey:            param.Key,
		ParamValue:          param.Value,
		Description:         param.Description,
		Enabled:             param.Enabled,
		ParentSearchParamID: parentID,
		IsDelta:             param.IsDelta,
		DeltaParamKey:       deltaKey,
		DeltaParamValue:     deltaValue,
		DeltaDescription:    param.DeltaDescription,
		DeltaEnabled:        param.DeltaEnabled,
		CreatedAt:           param.CreatedAt,
		UpdatedAt:           param.UpdatedAt,
	}
}

func NewHttpSearchParamService(queries *gen.Queries) *HttpSearchParamService {
	return &HttpSearchParamService{
		queries: queries,
	}
}

func (hsps HttpSearchParamService) CreateBulk(ctx context.Context, params []mhttp.HTTPSearchParam) error {
	now := dbtime.DBNow().Unix()

	// Set timestamps for all params
	for i := range params {
		params[i].CreatedAt = now
		params[i].UpdatedAt = now
	}

	// Convert and create each param individually
	for _, param := range params {
		dbParam := ConvertToDBHttpSearchParam(param)
		err := hsps.queries.CreateHTTPSearchParam(ctx, gen.CreateHTTPSearchParamParams{
			ID:                      dbParam.ID,
			HttpID:                  dbParam.HttpID,
			Key:                     dbParam.Key,
			Value:                   dbParam.Value,
			Description:             dbParam.Description,
			Enabled:                 dbParam.Enabled,
			Order:                   0, // Default order
			ParentHttpSearchParamID: dbParam.ParentHttpSearchParamID,
			IsDelta:                 dbParam.IsDelta,
			DeltaKey:                dbParam.DeltaKey,
			DeltaValue:              dbParam.DeltaValue,
			DeltaEnabled:            dbParam.DeltaEnabled,
			DeltaDescription:        dbParam.DeltaDescription,
			CreatedAt:               dbParam.CreatedAt,
			UpdatedAt:               dbParam.UpdatedAt,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (hsps HttpSearchParamService) Create(ctx context.Context, param mhttp.HTTPSearchParam) error {
	now := dbtime.DBNow().Unix()
	param.CreatedAt = now
	param.UpdatedAt = now

	dbParam := ConvertToDBHttpSearchParam(param)
	return hsps.queries.CreateHTTPSearchParam(ctx, gen.CreateHTTPSearchParamParams{
		ID:                      dbParam.ID,
		HttpID:                  dbParam.HttpID,
		Key:                     dbParam.Key,
		Value:                   dbParam.Value,
		Description:             dbParam.Description,
		Enabled:                 dbParam.Enabled,
		Order:                   0, // Default order
		ParentHttpSearchParamID: dbParam.ParentHttpSearchParamID,
		IsDelta:                 dbParam.IsDelta,
		DeltaKey:                dbParam.DeltaKey,
		DeltaValue:              dbParam.DeltaValue,
		DeltaEnabled:            dbParam.DeltaEnabled,
		DeltaDescription:        dbParam.DeltaDescription,
		CreatedAt:               dbParam.CreatedAt,
		UpdatedAt:               dbParam.UpdatedAt,
	})
}

func (hsps HttpSearchParamService) GetByHttpID(ctx context.Context, httpID idwrap.IDWrap) ([]mhttp.HTTPSearchParam, error) {
	params, err := hsps.queries.GetHTTPSearchParams(ctx, httpID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mhttp.HTTPSearchParam{}, nil
		}
		return nil, err
	}

	result := make([]mhttp.HTTPSearchParam, len(params))
	for i, param := range params {
		result[i] = ConvertRowToModelHttpSearchParam(param)
	}
	return result, nil
}

func (hsps HttpSearchParamService) TX(tx *sql.Tx) *HttpSearchParamService {
	return &HttpSearchParamService{
		queries: hsps.queries.WithTx(tx),
	}
}
