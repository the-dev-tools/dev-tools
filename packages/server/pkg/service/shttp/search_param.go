package shttp

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/dbtime"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mhttp"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHttpSearchParamFound = errors.New("no HTTP search param found")

type HttpSearchParamService struct {
	queries *gen.Queries
}

func ConvertToDBHttpSearchParam(param mhttp.HTTPSearchParam) gen.HttpSearchParam {
	return gen.HttpSearchParam{
		ID:                  param.ID,
		HttpID:              param.HttpID,
		ParamKey:            param.ParamKey,
		ParamValue:          param.ParamValue,
		Description:         param.Description,
		Enabled:             param.Enabled,
		ParentSearchParamID: param.ParentSearchParamID,
		IsDelta:             param.IsDelta,
		DeltaParamKey:       param.DeltaParamKey,
		DeltaParamValue:     param.DeltaParamValue,
		DeltaDescription:    param.DeltaDescription,
		DeltaEnabled:        param.DeltaEnabled,
		Prev:                param.Prev,
		Next:                param.Next,
		CreatedAt:           param.CreatedAt,
		UpdatedAt:           param.UpdatedAt,
	}
}

func ConvertToModelHttpSearchParam(param gen.HttpSearchParam) mhttp.HTTPSearchParam {
	return mhttp.HTTPSearchParam{
		ID:                  param.ID,
		HttpID:              param.HttpID,
		ParamKey:            param.ParamKey,
		ParamValue:          param.ParamValue,
		Description:         param.Description,
		Enabled:             param.Enabled,
		ParentSearchParamID: param.ParentSearchParamID,
		IsDelta:             param.IsDelta,
		DeltaParamKey:       param.DeltaParamKey,
		DeltaParamValue:     param.DeltaParamValue,
		DeltaDescription:    param.DeltaDescription,
		DeltaEnabled:        param.DeltaEnabled,
		Prev:                param.Prev,
		Next:                param.Next,
		CreatedAt:           param.CreatedAt,
		UpdatedAt:           param.UpdatedAt,
	}
}

func NewHttpSearchParamService(queries *gen.Queries) HttpSearchParamService {
	return HttpSearchParamService{queries: queries}
}

func (hsps HttpSearchParamService) TX(tx *sql.Tx) HttpSearchParamService {
	return HttpSearchParamService{queries: hsps.queries.WithTx(tx)}
}

func NewHttpSearchParamServiceTX(ctx context.Context, tx *sql.Tx) (*HttpSearchParamService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := HttpSearchParamService{queries: queries}
	return &service, nil
}

func (hsps HttpSearchParamService) CreateBulk(ctx context.Context, params []mhttp.HTTPSearchParam) error {
	const sizeOfChunks = 10
	now := dbtime.DBNow().Unix()

	// Set timestamps for all params
	for i := range params {
		params[i].CreatedAt = now
		params[i].UpdatedAt = now
	}

	convertedItems := tgeneric.MassConvert(params, ConvertToDBHttpSearchParam)
	for paramChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(paramChunk) < sizeOfChunks {
			for _, param := range paramChunk {
				err := hsps.Create(ctx, param)
				if err != nil {
					return err
				}
			}
			continue
		}

		item1 := paramChunk[0]
		item2 := paramChunk[1]
		item3 := paramChunk[2]
		item4 := paramChunk[3]
		item5 := paramChunk[4]
		item6 := paramChunk[5]
		item7 := paramChunk[6]
		item8 := paramChunk[7]
		item9 := paramChunk[8]
		item10 := paramChunk[9]

		params := gen.CreateHTTPSearchParamsBulkParams{
			// 1
			ID:                  item1.ID,
			HttpID:              item1.HttpID,
			ParamKey:            item1.ParamKey,
			ParamValue:          item1.ParamValue,
			Description:         item1.Description,
			Enabled:             item1.Enabled,
			ParentSearchParamID: item1.ParentSearchParamID,
			IsDelta:             item1.IsDelta,
			DeltaParamKey:       item1.DeltaParamKey,
			DeltaParamValue:     item1.DeltaParamValue,
			DeltaDescription:    item1.DeltaDescription,
			DeltaEnabled:        item1.DeltaEnabled,
			Prev:                item1.Prev,
			Next:                item1.Next,
			CreatedAt:           item1.CreatedAt,
			UpdatedAt:           item1.UpdatedAt,
			// 2
			ID_2:                  item2.ID,
			HttpID_2:              item2.HttpID,
			ParamKey_2:            item2.ParamKey,
			ParamValue_2:          item2.ParamValue,
			Description_2:         item2.Description,
			Enabled_2:             item2.Enabled,
			ParentSearchParamID_2: item2.ParentSearchParamID,
			IsDelta_2:             item2.IsDelta,
			DeltaParamKey_2:       item2.DeltaParamKey,
			DeltaParamValue_2:     item2.DeltaParamValue,
			DeltaDescription_2:    item2.DeltaDescription,
			DeltaEnabled_2:        item2.DeltaEnabled,
			Prev_2:                item2.Prev,
			Next_2:                item2.Next,
			CreatedAt_2:           item2.CreatedAt,
			UpdatedAt_2:           item2.UpdatedAt,
			// 3
			ID_3:                  item3.ID,
			HttpID_3:              item3.HttpID,
			ParamKey_3:            item3.ParamKey,
			ParamValue_3:          item3.ParamValue,
			Description_3:         item3.Description,
			Enabled_3:             item3.Enabled,
			ParentSearchParamID_3: item3.ParentSearchParamID,
			IsDelta_3:             item3.IsDelta,
			DeltaParamKey_3:       item3.DeltaParamKey,
			DeltaParamValue_3:     item3.DeltaParamValue,
			DeltaDescription_3:    item3.DeltaDescription,
			DeltaEnabled_3:        item3.DeltaEnabled,
			Prev_3:                item3.Prev,
			Next_3:                item3.Next,
			CreatedAt_3:           item3.CreatedAt,
			UpdatedAt_3:           item3.UpdatedAt,
			// 4
			ID_4:                  item4.ID,
			HttpID_4:              item4.HttpID,
			ParamKey_4:            item4.ParamKey,
			ParamValue_4:          item4.ParamValue,
			Description_4:         item4.Description,
			Enabled_4:             item4.Enabled,
			ParentSearchParamID_4: item4.ParentSearchParamID,
			IsDelta_4:             item4.IsDelta,
			DeltaParamKey_4:       item4.DeltaParamKey,
			DeltaParamValue_4:     item4.DeltaParamValue,
			DeltaDescription_4:    item4.DeltaDescription,
			DeltaEnabled_4:        item4.DeltaEnabled,
			Prev_4:                item4.Prev,
			Next_4:                item4.Next,
			CreatedAt_4:           item4.CreatedAt,
			UpdatedAt_4:           item4.UpdatedAt,
			// 5
			ID_5:                  item5.ID,
			HttpID_5:              item5.HttpID,
			ParamKey_5:            item5.ParamKey,
			ParamValue_5:          item5.ParamValue,
			Description_5:         item5.Description,
			Enabled_5:             item5.Enabled,
			ParentSearchParamID_5: item5.ParentSearchParamID,
			IsDelta_5:             item5.IsDelta,
			DeltaParamKey_5:       item5.DeltaParamKey,
			DeltaParamValue_5:     item5.DeltaParamValue,
			DeltaDescription_5:    item5.DeltaDescription,
			DeltaEnabled_5:        item5.DeltaEnabled,
			Prev_5:                item5.Prev,
			Next_5:                item5.Next,
			CreatedAt_5:           item5.CreatedAt,
			UpdatedAt_5:           item5.UpdatedAt,
			// 6
			ID_6:                  item6.ID,
			HttpID_6:              item6.HttpID,
			ParamKey_6:            item6.ParamKey,
			ParamValue_6:          item6.ParamValue,
			Description_6:         item6.Description,
			Enabled_6:             item6.Enabled,
			ParentSearchParamID_6: item6.ParentSearchParamID,
			IsDelta_6:             item6.IsDelta,
			DeltaParamKey_6:       item6.DeltaParamKey,
			DeltaParamValue_6:     item6.DeltaParamValue,
			DeltaDescription_6:    item6.DeltaDescription,
			DeltaEnabled_6:        item6.DeltaEnabled,
			Prev_6:                item6.Prev,
			Next_6:                item6.Next,
			CreatedAt_6:           item6.CreatedAt,
			UpdatedAt_6:           item6.UpdatedAt,
			// 7
			ID_7:                  item7.ID,
			HttpID_7:              item7.HttpID,
			ParamKey_7:            item7.ParamKey,
			ParamValue_7:          item7.ParamValue,
			Description_7:         item7.Description,
			Enabled_7:             item7.Enabled,
			ParentSearchParamID_7: item7.ParentSearchParamID,
			IsDelta_7:             item7.IsDelta,
			DeltaParamKey_7:       item7.DeltaParamKey,
			DeltaParamValue_7:     item7.DeltaParamValue,
			DeltaDescription_7:    item7.DeltaDescription,
			DeltaEnabled_7:        item7.DeltaEnabled,
			Prev_7:                item7.Prev,
			Next_7:                item7.Next,
			CreatedAt_7:           item7.CreatedAt,
			UpdatedAt_7:           item7.UpdatedAt,
			// 8
			ID_8:                  item8.ID,
			HttpID_8:              item8.HttpID,
			ParamKey_8:            item8.ParamKey,
			ParamValue_8:          item8.ParamValue,
			Description_8:         item8.Description,
			Enabled_8:             item8.Enabled,
			ParentSearchParamID_8: item8.ParentSearchParamID,
			IsDelta_8:             item8.IsDelta,
			DeltaParamKey_8:       item8.DeltaParamKey,
			DeltaParamValue_8:     item8.DeltaParamValue,
			DeltaDescription_8:    item8.DeltaDescription,
			DeltaEnabled_8:        item8.DeltaEnabled,
			Prev_8:                item8.Prev,
			Next_8:                item8.Next,
			CreatedAt_8:           item8.CreatedAt,
			UpdatedAt_8:           item8.UpdatedAt,
			// 9
			ID_9:                  item9.ID,
			HttpID_9:              item9.HttpID,
			ParamKey_9:            item9.ParamKey,
			ParamValue_9:          item9.ParamValue,
			Description_9:         item9.Description,
			Enabled_9:             item9.Enabled,
			ParentSearchParamID_9: item9.ParentSearchParamID,
			IsDelta_9:             item9.IsDelta,
			DeltaParamKey_9:       item9.DeltaParamKey,
			DeltaParamValue_9:     item9.DeltaParamValue,
			DeltaDescription_9:    item9.DeltaDescription,
			DeltaEnabled_9:        item9.DeltaEnabled,
			Prev_9:                item9.Prev,
			Next_9:                item9.Next,
			CreatedAt_9:           item9.CreatedAt,
			UpdatedAt_9:           item9.UpdatedAt,
			// 10
			ID_10:                  item10.ID,
			HttpID_10:              item10.HttpID,
			ParamKey_10:            item10.ParamKey,
			ParamValue_10:          item10.ParamValue,
			Description_10:         item10.Description,
			Enabled_10:             item10.Enabled,
			ParentSearchParamID_10: item10.ParentSearchParamID,
			IsDelta_10:             item10.IsDelta,
			DeltaParamKey_10:       item10.DeltaParamKey,
			DeltaParamValue_10:     item10.DeltaParamValue,
			DeltaDescription_10:    item10.DeltaDescription,
			DeltaEnabled_10:        item10.DeltaEnabled,
			Prev_10:                item10.Prev,
			Next_10:                item10.Next,
			CreatedAt_10:           item10.CreatedAt,
			UpdatedAt_10:           item10.UpdatedAt,
		}
		if err := hsps.queries.CreateHTTPSearchParamsBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (hsps HttpSearchParamService) Create(ctx context.Context, param gen.HttpSearchParam) error {
	return hsps.queries.CreateHTTPSearchParam(ctx, gen.CreateHTTPSearchParamParams{
		ID:                  param.ID,
		HttpID:              param.HttpID,
		ParamKey:            param.ParamKey,
		ParamValue:          param.ParamValue,
		Description:         param.Description,
		Enabled:             param.Enabled,
		ParentSearchParamID: param.ParentSearchParamID,
		IsDelta:             param.IsDelta,
		DeltaParamKey:       param.DeltaParamKey,
		DeltaParamValue:     param.DeltaParamValue,
		DeltaDescription:    param.DeltaDescription,
		DeltaEnabled:        param.DeltaEnabled,
		Prev:                param.Prev,
		Next:                param.Next,
		CreatedAt:           param.CreatedAt,
		UpdatedAt:           param.UpdatedAt,
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
	return tgeneric.MassConvert(params, ConvertToModelHttpSearchParam), nil
}
