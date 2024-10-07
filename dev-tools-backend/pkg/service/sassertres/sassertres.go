package sassertres

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/massertres"
	"dev-tools-db/pkg/sqlc/gen"
)

type AssertResultService struct {
	queries *gen.Queries
}

func ConvertAssertResultDBToModel(assertResponse gen.AssertionResult) massertres.AssertResult {
	return massertres.AssertResult{
		ID:       assertResponse.ID,
		AssertID: assertResponse.AssertionID,
		Result:   assertResponse.Result,
		Value:    assertResponse.AssertedValue,
	}
}

func ConvertAssertResultModelToDB(assertResponse massertres.AssertResult) gen.AssertionResult {
	return gen.AssertionResult{
		ID:            assertResponse.ID,
		AssertionID:   assertResponse.AssertID,
		Result:        assertResponse.Result,
		AssertedValue: assertResponse.Value,
	}
}

func New(ctx context.Context, db *sql.DB) (*AssertResultService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	service := AssertResultService{queries: queries}
	return &service, nil
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

func (ars AssertResultService) CreateAssertResult(ctx context.Context, assertResult massertres.AssertResult) error {
	assertResultDB := ConvertAssertResultModelToDB(assertResult)
	return ars.queries.CreateAssertResult(ctx, gen.CreateAssertResultParams{
		ID:            assertResultDB.ID,
		AssertionID:   assertResultDB.AssertionID,
		Result:        assertResultDB.Result,
		AssertedValue: assertResultDB.AssertedValue,
	})
}

func (ars AssertResultService) UpdateAssertResult(ctx context.Context, assertResult massertres.AssertResult) error {
	assertResultDB := ConvertAssertResultModelToDB(assertResult)
	return ars.queries.UpdateAssertResult(ctx, gen.UpdateAssertResultParams{
		ID:            assertResultDB.ID,
		Result:        assertResultDB.Result,
		AssertedValue: assertResultDB.AssertedValue,
	})
}

func (ars AssertResultService) DeleteAssertResult(ctx context.Context, id idwrap.IDWrap) error {
	return ars.queries.DeleteAssertResult(ctx, id)
}
