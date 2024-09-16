package sbodyform

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyform"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
)

type BodyFormService struct {
	queries *gen.Queries
}

var ErrNoBodyFormFound = sql.ErrNoRows

func SeralizeModeltoGen(body mbodyform.BodyForm) gen.ExampleBodyForm {
	return gen.ExampleBodyForm{
		ID:          body.ID,
		ExampleID:   body.ExampleID,
		BodyKey:     body.BodyKey,
		Description: body.Description,
		Enable:      body.Enable,
		Value:       body.Value,
	}
}

func DeserializeGenToModel(body gen.ExampleBodyForm) mbodyform.BodyForm {
	return mbodyform.BodyForm{
		ID:          body.ID,
		ExampleID:   body.ExampleID,
		BodyKey:     body.BodyKey,
		Description: body.Description,
		Enable:      body.Enable,
		Value:       body.Value,
	}
}

func New(ctx context.Context, db *sql.DB) (*BodyFormService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return nil, err
	}
	service := BodyFormService{queries: queries}
	return &service, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*BodyFormService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := BodyFormService{queries: queries}
	return &service, nil
}

func (bfs BodyFormService) GetBodyForm(ctx context.Context, id idwrap.IDWrap) (*mbodyform.BodyForm, error) {
	bodyForm, err := bfs.queries.GetBodyForm(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoBodyFormFound
		}
		return nil, err
	}
	body := DeserializeGenToModel(bodyForm)
	return &body, nil
}

func (bfs BodyFormService) GetBodyFormsByExampleID(ctx context.Context, exampleID idwrap.IDWrap) ([]mbodyform.BodyForm, error) {
	bodyForms, err := bfs.queries.GetBodyFormsByExampleID(ctx, exampleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoBodyFormFound
		}
		return nil, err
	}
	return tgeneric.MassConvert(bodyForms, DeserializeGenToModel), nil
}

func (bfs BodyFormService) CreateBodyForm(ctx context.Context, body *mbodyform.BodyForm) error {
	bf := SeralizeModeltoGen(*body)
	return bfs.queries.CreateBodyForm(ctx, gen.CreateBodyFormParams{
		ID:          bf.ID,
		ExampleID:   bf.ExampleID,
		BodyKey:     bf.BodyKey,
		Description: bf.Description,
		Enable:      bf.Enable,
		Value:       bf.Value,
	})
}

func (bfs BodyFormService) UpdateBodyForm(ctx context.Context, body *mbodyform.BodyForm) error {
	bf := SeralizeModeltoGen(*body)
	return bfs.queries.UpdateBodyForm(ctx, gen.UpdateBodyFormParams{
		ID:          bf.ID,
		BodyKey:     bf.BodyKey,
		Description: bf.Description,
		Enable:      bf.Enable,
		Value:       bf.Value,
	})
}

func (bfs BodyFormService) DeleteBodyForm(ctx context.Context, id idwrap.IDWrap) error {
	return bfs.queries.DeleteBodyForm(ctx, id)
}
