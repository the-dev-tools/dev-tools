package sbodyraw

import (
	"context"
	"database/sql"
	"dev-tools-backend/pkg/idwrap"
	"dev-tools-backend/pkg/model/mbodyraw"
	"dev-tools-backend/pkg/translate/tgeneric"
	"dev-tools-db/pkg/sqlc/gen"
	"slices"
)

type BodyRawService struct {
	queries *gen.Queries
}

var ErrNoBodyRawFound = sql.ErrNoRows

func ConvertModelToGen(body mbodyraw.ExampleBodyRaw) gen.ExampleBodyRaw {
	return gen.ExampleBodyRaw{
		ID:            body.ID,
		ExampleID:     body.ExampleID,
		VisualizeMode: int8(body.VisualizeMode),
		CompressType:  int8(body.CompressType),
		Data:          body.Data,
	}
}

func ConvertGenToModel(body gen.ExampleBodyRaw) mbodyraw.ExampleBodyRaw {
	return mbodyraw.ExampleBodyRaw{
		ID:            body.ID,
		ExampleID:     body.ExampleID,
		VisualizeMode: mbodyraw.VisualizeMode(body.VisualizeMode),
		CompressType:  mbodyraw.CompressType(body.CompressType),
		Data:          body.Data,
	}
}

func New(ctx context.Context, db *sql.DB) (BodyRawService, error) {
	queries, err := gen.Prepare(ctx, db)
	if err != nil {
		return BodyRawService{}, err
	}
	return BodyRawService{queries: queries}, nil
}

func NewTX(ctx context.Context, tx *sql.Tx) (*BodyRawService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := BodyRawService{queries: queries}
	return &service, nil
}

func (brs BodyRawService) GetBodyRaw(ctx context.Context, id idwrap.IDWrap) (*mbodyraw.ExampleBodyRaw, error) {
	bodyRaw, err := brs.queries.GetBodyRaw(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoBodyRawFound
		}
		return nil, err
	}
	body := ConvertGenToModel(bodyRaw)
	return &body, nil
}

func (brs BodyRawService) GetBodyRawByExampleID(ctx context.Context, exampleID idwrap.IDWrap) (*mbodyraw.ExampleBodyRaw, error) {
	bodyRaw, err := brs.queries.GetBodyRawsByExampleID(ctx, exampleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoBodyRawFound
		}
		return nil, err
	}
	body := ConvertGenToModel(bodyRaw)
	return &body, nil
}

func (brs BodyRawService) CreateBodyRaw(ctx context.Context, body mbodyraw.ExampleBodyRaw) error {
	bodyRaw := ConvertModelToGen(body)
	return brs.queries.CreateBodyRaw(ctx, gen.CreateBodyRawParams{
		ID:            bodyRaw.ID,
		ExampleID:     bodyRaw.ExampleID,
		VisualizeMode: bodyRaw.VisualizeMode,
		CompressType:  bodyRaw.CompressType,
		Data:          bodyRaw.Data,
	})
}

func (brs BodyRawService) CreateBodyRawGen(ctx context.Context, bodyGen gen.ExampleBodyRaw) error {
	return brs.queries.CreateBodyRaw(ctx, gen.CreateBodyRawParams{
		ID:            bodyGen.ID,
		ExampleID:     bodyGen.ExampleID,
		VisualizeMode: bodyGen.VisualizeMode,
		CompressType:  bodyGen.CompressType,
		Data:          bodyGen.Data,
	})
}

func (brs BodyRawService) CreateBulkBodyRaw(ctx context.Context, bodyRawArr []mbodyraw.ExampleBodyRaw) error {
	const sizeOfChunks = 5
	convertedItems := tgeneric.MassConvert(bodyRawArr, ConvertModelToGen)
	for bodyFormChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(bodyFormChunk) < sizeOfChunks {
			for _, bodyForm := range bodyFormChunk {
				err := brs.CreateBodyRawGen(ctx, bodyForm)
				if err != nil {
					return err
				}
			}
			continue
		}
		item1 := bodyFormChunk[0]
		item2 := bodyFormChunk[1]
		item3 := bodyFormChunk[2]
		item4 := bodyFormChunk[3]
		item5 := bodyFormChunk[4]

		params := gen.CreateBodyRawBulkParams{
			// 1
			ID:            item1.ID,
			ExampleID:     item1.ExampleID,
			VisualizeMode: item1.VisualizeMode,
			CompressType:  item1.CompressType,
			Data:          item1.Data,
			// 2
			ID_2:            item2.ID,
			ExampleID_2:     item2.ExampleID,
			VisualizeMode_2: item2.VisualizeMode,
			CompressType_2:  item2.CompressType,
			Data_2:          item2.Data,
			// 3
			ID_3:            item3.ID,
			ExampleID_3:     item3.ExampleID,
			VisualizeMode_3: item3.VisualizeMode,
			CompressType_3:  item3.CompressType,
			Data_3:          item3.Data,
			// 4
			ID_4:            item4.ID,
			ExampleID_4:     item4.ExampleID,
			VisualizeMode_4: item4.VisualizeMode,
			CompressType_4:  item4.CompressType,
			Data_4:          item4.Data,
			// 5
			ID_5:            item5.ID,
			ExampleID_5:     item5.ExampleID,
			VisualizeMode_5: item5.VisualizeMode,
			CompressType_5:  item5.CompressType,
			Data_5:          item5.Data,
		}
		if err := brs.queries.CreateBodyRawBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (brs BodyRawService) UpdateBodyRawBody(ctx context.Context, body mbodyraw.ExampleBodyRaw) error {
	bodyRaw := ConvertModelToGen(body)
	return brs.queries.UpdateBodyRawData(ctx, gen.UpdateBodyRawDataParams{
		ID:           bodyRaw.ID,
		CompressType: bodyRaw.CompressType,
		Data:         bodyRaw.Data,
	})
}

func (brs BodyRawService) UpdateBodyRawVisualize(ctx context.Context, body mbodyraw.ExampleBodyRaw) error {
	bodyRaw := ConvertModelToGen(body)
	return brs.queries.UpdateVisualizeMode(ctx, gen.UpdateVisualizeModeParams{
		ID:            bodyRaw.ID,
		VisualizeMode: bodyRaw.VisualizeMode,
	})
}

func (brs BodyRawService) DeleteBodyRaw(ctx context.Context, id idwrap.IDWrap) error {
	err := brs.queries.DeleteBodyRaw(ctx, id)
	if err != nil {
		return err
	}
	return nil
}
