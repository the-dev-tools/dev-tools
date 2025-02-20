package sassert

import (
	"context"
	"database/sql"
	"the-dev-tools/backend/pkg/idwrap"
	"the-dev-tools/backend/pkg/model/massert"
	"the-dev-tools/backend/pkg/translate/tgeneric"
	"the-dev-tools/db/pkg/sqlc/gen"
)

type AssertService struct {
	queries *gen.Queries
}

var ErrNoAssertFound = sql.ErrNoRows

func ConvertAssertDBToModel(assert gen.Assertion) massert.Assert {
	return massert.Assert{
		ID:        assert.ID,
		ExampleID: assert.ExampleID,
		Path:      assert.Path,
		Value:     assert.Value,
		Enable:    assert.Enable,
		Type:      massert.AssertType(assert.Type),
	}
}

func ConvertAssertModelToDB(assert massert.Assert) gen.Assertion {
	return gen.Assertion{
		ID:        assert.ID,
		ExampleID: assert.ExampleID,
		Path:      assert.Path,
		Value:     assert.Value,
		Type:      int8(assert.Type),
		Enable:    assert.Enable,
	}
}

func New(queries *gen.Queries) AssertService {
	return AssertService{queries: queries}
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
		if err == sql.ErrNoRows {
			return nil, ErrNoAssertFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(asserts, ConvertAssertDBToModel), nil
}

func (as AssertService) UpdateAssert(ctx context.Context, assert massert.Assert) error {
	arg := ConvertAssertModelToDB(assert)
	return as.queries.UpdateAssert(ctx, gen.UpdateAssertParams{
		ID:     arg.ID,
		Enable: arg.Enable,
		Path:   arg.Path,
		Value:  arg.Value,
		Type:   arg.Type,
	})
}

func (as AssertService) CreateAssert(ctx context.Context, assert massert.Assert) error {
	arg := ConvertAssertModelToDB(assert)
	return as.queries.CreateAssert(ctx, gen.CreateAssertParams{
		ID:        arg.ID,
		ExampleID: arg.ExampleID,
		Enable:    arg.Enable,
		Value:     arg.Value,
		Path:      arg.Path,
		Type:      int8(arg.Type),
	})
}

// TODO: create bulk query
func (as AssertService) CreateAssertBulk(ctx context.Context, asserts []massert.Assert) error {
	var err error
	for _, a := range asserts {
		err = as.CreateAssert(ctx, a)
		if err != nil {
			return err
		}
	}
	return nil
}

func (as AssertService) DeleteAssert(ctx context.Context, id idwrap.IDWrap) error {
	return as.queries.DeleteAssert(ctx, id)
}
