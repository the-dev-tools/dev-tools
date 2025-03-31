package sresultapi

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/result/mresultapi"
	"the-dev-tools/server/pkg/service/scollection"
	"the-dev-tools/server/pkg/service/sitemapi"
	"the-dev-tools/server/pkg/translate/tgeneric"
	"time"
)

type ResultApiService struct {
	queries *gen.Queries
}

func ConvertToDBResultApi(result mresultapi.MResultAPI) gen.ResultApi {
	return gen.ResultApi{
		ID:          result.ID,
		TriggerType: result.TriggerType,
		TriggerBy:   result.TriggerBy,
		Name:        result.Name,
		Time:        result.Time.Unix(),
		Duration:    result.Duration.Milliseconds(),
		HttpResp:    result.HttpResp,
	}
}

func ConvertToModelResultApi(result gen.ResultApi) *mresultapi.MResultAPI {
	return &mresultapi.MResultAPI{
		ID:          result.ID,
		TriggerType: result.TriggerType,
		TriggerBy:   result.TriggerBy,
		Name:        result.Name,
		Time:        time.Unix(result.Time, 0),
		Duration:    time.Duration(result.Duration),
		HttpResp:    result.HttpResp,
	}
}

func New(queries *gen.Queries) ResultApiService {
	return ResultApiService{queries: queries}
}

func (ras ResultApiService) TX(tx *sql.Tx) ResultApiService {
	return ResultApiService{queries: ras.queries.WithTx(tx)}
}

func (ras ResultApiService) CreateResultApi(ctx context.Context, result *mresultapi.MResultAPI) error {
	res := ConvertToDBResultApi(*result)

	return ras.queries.CreateResultApi(ctx, gen.CreateResultApiParams{
		ID:          res.ID,
		TriggerType: res.TriggerType,
		TriggerBy:   res.TriggerBy,
		Name:        res.Name,
		Time:        res.Time,
		Duration:    res.Duration,
		HttpResp:    res.HttpResp,
	})
}

func (ras ResultApiService) GetResultApi(id idwrap.IDWrap) (*mresultapi.MResultAPI, error) {
	result, err := ras.queries.GetResultApi(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return ConvertToModelResultApi(result), nil
}

func (ras ResultApiService) UpdateResultApi(ctx context.Context, result *mresultapi.MResultAPI) error {
	res := ConvertToDBResultApi(*result)

	return ras.queries.UpdateResultApi(ctx, gen.UpdateResultApiParams{
		ID:       res.ID,
		Name:     res.Name,
		Time:     res.Time,
		Duration: res.Duration,
		HttpResp: res.HttpResp,
	})
}

func (ras ResultApiService) DeleteResultApi(ctx context.Context, id idwrap.IDWrap) error {
	return ras.queries.DeleteResultApi(ctx, id)
}

func (ras ResultApiService) GetResultsApiWithTriggerBy(ctx context.Context, triggerBy idwrap.IDWrap, triggerType mresultapi.TriggerType) ([]mresultapi.MResultAPI, error) {
	resultsRaw, err := ras.queries.GetResultApiByTriggerBy(ctx, triggerBy)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvertPtr(resultsRaw, ConvertToModelResultApi), nil
}

func (ras ResultApiService) GetWorkspaceID(ctx context.Context, id idwrap.IDWrap, cs scollection.CollectionService, ias sitemapi.ItemApiService) (idwrap.IDWrap, error) {
	var ownerID idwrap.IDWrap
	result, err := ras.GetResultApi(id)
	if err != nil {
		return ownerID, err
	}
	switch result.TriggerType {
	case mresultapi.TRIGGER_TYPE_COLLECTION:
		collectionID, err := ias.GetOwnerID(ctx, result.TriggerBy)
		if err != nil {
			return ownerID, err
		}
		return cs.GetOwner(ctx, collectionID)
	default:
		return ownerID, errors.New("unsupported trigger type")
	}
}
