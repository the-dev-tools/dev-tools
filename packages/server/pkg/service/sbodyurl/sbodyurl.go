package sbodyurl

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyurl"
)

var (
	ErrNoBodyUrlEncodedFound = errors.New("no url encoded body found")
)

type BodyURLEncodedService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) BodyURLEncodedService {
	return BodyURLEncodedService{queries: queries}
}

func (bues BodyURLEncodedService) TX(tx *sql.Tx) BodyURLEncodedService {
	return BodyURLEncodedService{queries: bues.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*BodyURLEncodedService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := BodyURLEncodedService{queries: queries}
	return &service, nil
}

// ----- Serializers -----

func SeralizeModeltoGen(body mbodyurl.BodyURLEncoded) gen.ExampleBodyUrlencoded {
	var deltaParentID *idwrap.IDWrap
	if body.DeltaParentID != nil {
		deltaParentID = body.DeltaParentID
	}

	return gen.ExampleBodyUrlencoded{
		ID:            body.ID,
		ExampleID:     body.ExampleID,
		DeltaParentID: deltaParentID,
		BodyKey:       body.BodyKey,
		Enable:        body.Enable,
		Description:   body.Description,
		Value:         body.Value,
	}
}

func DeserializeGenToModel(body gen.ExampleBodyUrlencoded) mbodyurl.BodyURLEncoded {
	return mbodyurl.BodyURLEncoded{
		ID:            body.ID,
		ExampleID:     body.ExampleID,
		DeltaParentID: body.DeltaParentID,
		BodyKey:       body.BodyKey,
		Enable:        body.Enable,
		Description:   body.Description,
		Value:         body.Value,
	}
}

func (bues BodyURLEncodedService) GetBodyURLEncoded(ctx context.Context, id idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	body, err := bues.queries.GetBodyUrlEncoded(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoBodyUrlEncodedFound
		}
		return nil, err
	}
	urlEncoded := DeserializeGenToModel(body)
	return &urlEncoded, nil
}

func (bues BodyURLEncodedService) GetBodyURLEncodedByExampleID(ctx context.Context, exampleID idwrap.IDWrap) ([]mbodyurl.BodyURLEncoded, error) {
	bodys, err := bues.queries.GetBodyUrlEncodedsByExampleID(ctx, exampleID)
	if err != nil {
		return nil, err
	}
	var bodyURLEncodeds []mbodyurl.BodyURLEncoded
	for _, body := range bodys {
		bodyURLEncodeds = append(bodyURLEncodeds, DeserializeGenToModel(body))
	}
	return bodyURLEncodeds, nil
}

// TODO: Re-enable after code regeneration
// func (bues BodyURLEncodedService) GetBodyURLEncodedByDeltaParentID(ctx context.Context, deltaParentID idwrap.IDWrap) ([]mbodyurl.BodyURLEncoded, error) {
// 	bodys, err := bues.queries.GetBodyUrlEncodedsByDeltaParentID(ctx, &deltaParentID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var bodyURLEncodeds []mbodyurl.BodyURLEncoded
// 	for _, body := range bodys {
// 		bodyURLEncodeds = append(bodyURLEncodeds, DeserializeGenToModel(body))
// 	}
// 	return bodyURLEncodeds, nil
// }

func (bues BodyURLEncodedService) CreateBodyURLEncoded(ctx context.Context, body *mbodyurl.BodyURLEncoded) error {
	err := bues.queries.CreateBodyUrlEncoded(ctx, gen.CreateBodyUrlEncodedParams{
		ID:            body.ID,
		ExampleID:     body.ExampleID,
		DeltaParentID: body.DeltaParentID,
		BodyKey:       body.BodyKey,
		Enable:        body.Enable,
		Description:   body.Description,
		Value:         body.Value,
	})
	return err
}

func (bues BodyURLEncodedService) CreateBodyFormRaw(ctx context.Context, bodyForm gen.ExampleBodyUrlencoded) error {
	err := bues.queries.CreateBodyUrlEncoded(ctx, gen.CreateBodyUrlEncodedParams{
		ID:            bodyForm.ID,
		ExampleID:     bodyForm.ExampleID,
		DeltaParentID: bodyForm.DeltaParentID,
		BodyKey:       bodyForm.BodyKey,
		Enable:        bodyForm.Enable,
		Description:   bodyForm.Description,
		Value:         bodyForm.Value,
	})
	return err
}

func (bues BodyURLEncodedService) CreateBulkBodyURLEncoded(ctx context.Context, bodyForms []mbodyurl.BodyURLEncoded) error {
	if len(bodyForms) == 0 {
		return nil
	}

	const batchSize = 10
	for i := 0; i < len(bodyForms); i += batchSize {
		end := i + batchSize
		if end > len(bodyForms) {
			end = len(bodyForms)
		}

		batch := bodyForms[i:end]
		params := gen.CreateBodyUrlEncodedBulkParams{}

		// Set the bulk parameters based on batch size
		if len(batch) > 0 {
			params.ID = batch[0].ID
			params.ExampleID = batch[0].ExampleID
			params.DeltaParentID = batch[0].DeltaParentID
			params.BodyKey = batch[0].BodyKey
			params.Enable = batch[0].Enable
			params.Description = batch[0].Description
			params.Value = batch[0].Value
		}

		if len(batch) > 1 {
			params.ID_2 = batch[1].ID
			params.ExampleID_2 = batch[1].ExampleID
			params.DeltaParentID_2 = batch[1].DeltaParentID
			params.BodyKey_2 = batch[1].BodyKey
			params.Enable_2 = batch[1].Enable
			params.Description_2 = batch[1].Description
			params.Value_2 = batch[1].Value
		}

		// Continue for batch[2] through batch[9] if they exist...
		if len(batch) > 2 {
			params.ID_3 = batch[2].ID
			params.ExampleID_3 = batch[2].ExampleID
			params.DeltaParentID_3 = batch[2].DeltaParentID
			params.BodyKey_3 = batch[2].BodyKey
			params.Enable_3 = batch[2].Enable
			params.Description_3 = batch[2].Description
			params.Value_3 = batch[2].Value
		}

		err := bues.queries.CreateBodyUrlEncodedBulk(ctx, params)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bues BodyURLEncodedService) UpdateBodyURLEncoded(ctx context.Context, body *mbodyurl.BodyURLEncoded) error {
	err := bues.queries.UpdateBodyUrlEncoded(ctx, gen.UpdateBodyUrlEncodedParams{
		BodyKey:     body.BodyKey,
		Enable:      body.Enable,
		Description: body.Description,
		Value:       body.Value,
		ID:          body.ID,
	})
	return err
}

func (bues BodyURLEncodedService) DeleteBodyURLEncoded(ctx context.Context, id idwrap.IDWrap) error {
	err := bues.queries.DeleteBodyURLEncoded(ctx, id)
	return err
}

func (bues BodyURLEncodedService) ResetBodyURLEncodedDelta(ctx context.Context, id idwrap.IDWrap) error {
	bodyURLEncoded, err := bues.GetBodyURLEncoded(ctx, id)
	if err != nil {
		return err
	}

	bodyURLEncoded.DeltaParentID = nil
	bodyURLEncoded.BodyKey = ""
	bodyURLEncoded.Enable = false
	bodyURLEncoded.Description = ""
	bodyURLEncoded.Value = ""

	return bues.UpdateBodyURLEncoded(ctx, bodyURLEncoded)
}
