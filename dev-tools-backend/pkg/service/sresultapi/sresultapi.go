package sresultapi

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/model/result/mresultapi"
	"dev-tools-backend/pkg/service/scollection"
	"dev-tools-backend/pkg/service/sitemapi"
	"dev-tools-db/pkg/sqlc/gen"
	"errors"
	"time"

	"github.com/oklog/ulid/v2"
)

type ResultApiService struct {
	db      *sql.DB
	queries *gen.Queries
}

func MassConvert[T any, O any](item []T, convFunc func(T) *O) []O {
	arr := make([]O, len(item))
	for i, v := range item {
		arr[i] = *convFunc(v)
	}
	return arr
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

func New(ctx context.Context, db *sql.DB) (*ResultApiService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	resultApiService := ResultApiService{db, queries}
	return &resultApiService, nil
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

func (ras ResultApiService) GetResultApi(id ulid.ULID) (*mresultapi.MResultAPI, error) {
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

func (ras ResultApiService) DeleteResultApi(ctx context.Context, id ulid.ULID) error {
	return ras.queries.DeleteResultApi(ctx, id)
}

func (ras ResultApiService) GetResultsApiWithTriggerBy(ctx context.Context, triggerBy ulid.ULID, triggerType mresultapi.TriggerType) ([]mresultapi.MResultAPI, error) {
	resultsRaw, err := ras.queries.GetResultApiByTriggerBy(ctx, triggerBy)
	if err != nil {
		return nil, err
	}
	return MassConvert(resultsRaw, ConvertToModelResultApi), nil
}

func (ras ResultApiService) GetWorkspaceID(ctx context.Context, id ulid.ULID, cs scollection.CollectionService, ias sitemapi.ItemApiService) (ulid.ULID, error) {
	var ownerID ulid.ULID
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
