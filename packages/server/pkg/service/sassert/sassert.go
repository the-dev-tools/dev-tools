package sassert

import (
	"context"
	"database/sql"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

type AssertService struct {
	queries *gen.Queries
}

var ErrNoAssertFound = sql.ErrNoRows

func ConvertAssertDBToModel(assert gen.Assertion) massert.Assert {
	return massert.Assert{
		ID:            assert.ID,
		ExampleID:     assert.ExampleID,
		DeltaParentID: assert.DeltaParentID,
		Condition: mcondition.Condition{
			Comparisons: mcondition.Comparison{
				Expression: assert.Expression,
			},
		},
		Enable: assert.Enable,
		Prev:   assert.Prev,
		Next:   assert.Next,
	}
}

func ConvertAssertModelToDB(assert massert.Assert) gen.Assertion {
	return gen.Assertion{
		ID:            assert.ID,
		ExampleID:     assert.ExampleID,
		DeltaParentID: assert.DeltaParentID,
		Expression:    assert.Condition.Comparisons.Expression,
		Enable:        assert.Enable,
		Prev:          assert.Prev,
		Next:          assert.Next,
	}
}

func New(queries *gen.Queries) AssertService {
	return AssertService{queries: queries}
}

func (as AssertService) TX(tx *sql.Tx) AssertService {
	return AssertService{queries: as.queries.WithTx(tx)}
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
		ID:            arg.ID,
		Enable:        arg.Enable,
		Expression:    arg.Expression,
		DeltaParentID: arg.DeltaParentID,
	})
}

func (as AssertService) CreateAssert(ctx context.Context, assert massert.Assert) error {
	arg := ConvertAssertModelToDB(assert)
	return as.queries.CreateAssert(ctx, gen.CreateAssertParams{
		ID:            arg.ID,
		ExampleID:     arg.ExampleID,
		DeltaParentID: arg.DeltaParentID,
		Enable:        arg.Enable,
		Expression:    assert.Condition.Comparisons.Expression,
		Prev:          arg.Prev,
		Next:          arg.Next,
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

// CreateBulkAssert is an alias for CreateAssertBulk
func (as AssertService) CreateBulkAssert(ctx context.Context, asserts []massert.Assert) error {
	return as.CreateAssertBulk(ctx, asserts)
}

func (as AssertService) DeleteAssert(ctx context.Context, id idwrap.IDWrap) error {
	return as.queries.DeleteAssert(ctx, id)
}

func (as AssertService) ResetAssertDelta(ctx context.Context, id idwrap.IDWrap) error {
	assert, err := as.GetAssert(ctx, id)
	if err != nil {
		return err
	}

	assert.DeltaParentID = nil
	assert.Condition.Comparisons.Expression = ""
	assert.Enable = false

	return as.UpdateAssert(ctx, *assert)
}
