package sassert

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/massert"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
)

type AssertService struct {
	queries *gen.Queries
}

var ErrNoAssertFound = sql.ErrNoRows

func ConvertAssertDBToModel(assert gen.Assertion) massert.Assert {
	return massert.Assert{
		ID:        assert.ID,
		ExampleID: assert.ExampleID,
		Name:      assert.Name,
		Value:     assert.Value,
		Type:      massert.AssertType(assert.Type),
		Target:    massert.AssertTarget(assert.TargetType),
	}
}

func ConvertAssertModelToDB(assert massert.Assert) gen.Assertion {
	return gen.Assertion{
		ID:         assert.ID,
		ExampleID:  assert.ExampleID,
		Name:       assert.Name,
		Value:      assert.Value,
		Type:       int8(assert.Type),
		TargetType: int8(assert.Target),
	}
}

func New(ctx context.Context, db *sql.DB) (AssertService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return AssertService{}, err
	}
	return AssertService{queries: queries}, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*AssertService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := AssertService{queries: queries}
	return &service, nil
}

func (as AssertService) GetAssert(ctx context.Context, id idwrap.IDWrap) (*massert.Assert, error) {
	assert, err := as.queries.GetAssert(ctx, id)
	if err != nil {
		return nil, err
	}
	a := ConvertAssertDBToModel(assert)
	return &a, nil
}

func (as AssertService) GetAssertByExampleID(ctx context.Context, id idwrap.IDWrap) ([]massert.Assert, error) {
	asserts, err := as.queries.GetAssertsByExampleID(ctx, id)
	if err != nil {
		return nil, err
	}
	return tgeneric.MassConvert(asserts, ConvertAssertDBToModel), nil
}

func (as AssertService) UpdateAssert(ctx context.Context, assert massert.Assert) error {
	arg := ConvertAssertModelToDB(assert)
	return as.queries.UpdateAssert(ctx, gen.UpdateAssertParams{
		ID:          arg.ID,
		Name:        arg.Name,
		Description: arg.Description,
		Enable:      arg.Enable,
		Value:       arg.Value,
		Type:        arg.Type,
		TargetType:  arg.TargetType,
	})
}

func (as AssertService) CreateAssert(ctx context.Context, assert massert.Assert) error {
	arg := ConvertAssertModelToDB(assert)
	return as.queries.CreateAssert(ctx, gen.CreateAssertParams{
		ID:         arg.ID,
		ExampleID:  arg.ExampleID,
		Name:       arg.Name,
		Value:      arg.Value,
		Type:       int8(arg.Type),
		TargetType: int8(arg.TargetType),
	})
}

func (as AssertService) DeleteAssert(ctx context.Context, id idwrap.IDWrap) error {
	return as.queries.DeleteAssert(ctx, id)
}
