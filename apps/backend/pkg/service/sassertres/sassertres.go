package sassertres

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massertres"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type AssertResultService struct {
	queries *gen.Queries
}

func ConvertAssertResultDBToModel(assertResponse gen.AssertionResult) massertres.AssertResult {
	return massertres.AssertResult{
		ID:         assertResponse.ID,
		ResponseID: assertResponse.ResponseID,
		AssertID:   assertResponse.AssertionID,
		Result:     assertResponse.Result,
	}
}

func ConvertAssertResultModelToDB(assertResponse massertres.AssertResult) gen.AssertionResult {
	return gen.AssertionResult{
		ID:          assertResponse.ID,
		ResponseID:  assertResponse.ResponseID,
		AssertionID: assertResponse.AssertID,
		Result:      assertResponse.Result,
	}
}

func New(queries *gen.Queries) AssertResultService {
	return AssertResultService{queries: queries}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*AssertResultService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := AssertResultService{queries: queries}
	return &service, nil
}

func (ars AssertResultService) GetAssertResult(ctx context.Context, id idwrap.IDWrap) (*massertres.AssertResult, error) {
	assertResult, err := ars.queries.GetAssertResult(ctx, id)
	if err != nil {
		return nil, err
	}
	a := ConvertAssertResultDBToModel(assertResult)
	return &a, nil
}

func (ars AssertResultService) GetAssertResultsByResponseID(ctx context.Context, responseID idwrap.IDWrap) ([]massertres.AssertResult, error) {
	assertResaultsRaw, err := ars.queries.GetAssertResultsByResponseID(ctx, responseID)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(assertResaultsRaw, ConvertAssertResultDBToModel), nil
}

func (ars AssertResultService) CreateAssertResult(ctx context.Context, assertResult massertres.AssertResult) error {
	assertResultDB := ConvertAssertResultModelToDB(assertResult)
	return ars.queries.CreateAssertResult(ctx, gen.CreateAssertResultParams{
		ID:          assertResultDB.ID,
		ResponseID:  assertResultDB.ResponseID,
		AssertionID: assertResultDB.AssertionID,
		Result:      assertResultDB.Result,
	})
}

func (ars AssertResultService) UpdateAssertResult(ctx context.Context, assertResult massertres.AssertResult) error {
	assertResultDB := ConvertAssertResultModelToDB(assertResult)
	return ars.queries.UpdateAssertResult(ctx, gen.UpdateAssertResultParams{
		ID:     assertResultDB.ID,
		Result: assertResultDB.Result,
	})
}

func (ars AssertResultService) DeleteAssertResult(ctx context.Context, id idwrap.IDWrap) error {
	return ars.queries.DeleteAssertResult(ctx, id)
}
