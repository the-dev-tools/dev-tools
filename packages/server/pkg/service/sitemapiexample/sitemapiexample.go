package sitemapiexample

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mitemapiexample"
)

type ItemApiExampleService struct {
	Queries *gen.Queries
}

var ErrNoItemApiExampleFound = errors.New("no example found")

func New(queries *gen.Queries) ItemApiExampleService {
	return ItemApiExampleService{Queries: queries}
}

func (ias ItemApiExampleService) TX(tx *sql.Tx) ItemApiExampleService {
	return ItemApiExampleService{Queries: ias.Queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*ItemApiExampleService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	return &ItemApiExampleService{
		Queries: queries,
	}, nil
}

func MassConvert[T any, O any](item []T, convFunc func(T) *O) []O {
	arr := make([]O, len(item))
	for i, v := range item {
		arr[i] = *convFunc(v)
	}
	return arr
}

func ConvertToDBItem(item mitemapiexample.ItemApiExample) gen.ItemApiExample {
	// TODO: add headers and query
	return gen.ItemApiExample{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		CollectionID: item.CollectionID,
		IsDefault:    item.IsDefault,
		BodyType:     int8(item.BodyType),
		Name:         item.Name,

		VersionParentID: item.VersionParentID,
		Prev:            item.Prev,
		Next:            item.Next,
	}
}

func ConvertToModelItem(item gen.ItemApiExample) *mitemapiexample.ItemApiExample {
	return &mitemapiexample.ItemApiExample{
		ID:           item.ID,
		ItemApiID:    item.ItemApiID,
		CollectionID: item.CollectionID,
		IsDefault:    item.IsDefault,
		BodyType:     mitemapiexample.BodyType(item.BodyType),
		Name:         item.Name,

		VersionParentID: item.VersionParentID,
		Prev:            item.Prev,
		Next:            item.Next,
	}
}

func (iaes ItemApiExampleService) GetApiExamples(ctx context.Context, apiUlid idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExamples(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) GetApiExamplesWithDefaults(ctx context.Context, endpointID idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExamplesWithDefaults(ctx, endpointID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) GetDefaultApiExample(ctx context.Context, apiUlid idwrap.IDWrap) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExampleDefault(ctx, apiUlid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}

	return ConvertToModelItem(itemApiExample), nil
}

func (iaes ItemApiExampleService) GetApiExample(ctx context.Context, id idwrap.IDWrap) (*mitemapiexample.ItemApiExample, error) {
	itemApiExample, err := iaes.Queries.GetItemApiExample(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return ConvertToModelItem(itemApiExample), nil
}

func (iaes ItemApiExampleService) GetExampleAllParentsNames(ctx context.Context, id idwrap.IDWrap) (*mitemapiexample.ExampleBreadcrumbs, error) {
	arg := gen.GetExampleAllParentsNamesParams{
		ID:   id,
		ID_2: id,
	}

	names, err := iaes.Queries.GetExampleAllParentsNames(ctx, arg)
	if err != nil {
		return nil, err
	}
	var folderPathPtr *string
	if names.FolderPath != nil {
		path, ok := names.FolderPath.(string)
		if !ok {
			return nil, errors.New("folderPath type is not string")
		}
		folderPathPtr = &path
	}

	breadcrumbs := mitemapiexample.ExampleBreadcrumbs{
		CollectionName: names.CollectionName,
		ApiName:        names.ApiName,
		ExampleName:    names.ExampleName,
		FolderPath:     folderPathPtr,
	}
	return &breadcrumbs, nil
}

func (iaes ItemApiExampleService) GetApiExampleByCollection(ctx context.Context, collectionID idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExampleByCollectionID(ctx, collectionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) GetApiExampleByVersionParentID(ctx context.Context, versionID idwrap.IDWrap) ([]mitemapiexample.ItemApiExample, error) {
	itemApiExamples, err := iaes.Queries.GetItemApiExampleByVersionParentID(ctx, &versionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mitemapiexample.ItemApiExample{}, ErrNoItemApiExampleFound
		}
		return nil, err
	}
	return MassConvert(itemApiExamples, ConvertToModelItem), nil
}

func (iaes ItemApiExampleService) CreateApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	arg := ConvertToDBItem(*item)
	return iaes.Queries.CreateItemApiExample(ctx, gen.CreateItemApiExampleParams{
		ID:              arg.ID,
		ItemApiID:       arg.ItemApiID,
		CollectionID:    arg.CollectionID,
		IsDefault:       arg.IsDefault,
		BodyType:        arg.BodyType,
		Name:            arg.Name,
		VersionParentID: arg.VersionParentID,
		Prev:            arg.Prev,
		Next:            arg.Next,
	})
}

func (iaes ItemApiExampleService) CreateApiExampleBulk(ctx context.Context, items []mitemapiexample.ItemApiExample) error {
	sizeOfChunks := 10

	for chunk := range slices.Chunk(items, sizeOfChunks) {
		if len(chunk) < sizeOfChunks {
			for _, item := range chunk {
				err := iaes.CreateApiExample(ctx, &item)
				if err != nil {
					return err
				}
			}
			continue
		}

		// Convert all items in the chunk using ConvertToDBItem
		dbItems := make([]gen.ItemApiExample, len(chunk))
		for i, item := range chunk {
			dbItems[i] = ConvertToDBItem(item)
		}

		params := gen.CreateItemApiExampleBulkParams{
			ID:              dbItems[0].ID,
			ItemApiID:       dbItems[0].ItemApiID,
			CollectionID:    dbItems[0].CollectionID,
			IsDefault:       dbItems[0].IsDefault,
			BodyType:        dbItems[0].BodyType,
			Name:            dbItems[0].Name,
			VersionParentID: dbItems[0].VersionParentID,
			Prev:            dbItems[0].Prev,
			Next:            dbItems[0].Next,

			ID_2:              dbItems[1].ID,
			ItemApiID_2:       dbItems[1].ItemApiID,
			CollectionID_2:    dbItems[1].CollectionID,
			IsDefault_2:       dbItems[1].IsDefault,
			BodyType_2:        dbItems[1].BodyType,
			Name_2:            dbItems[1].Name,
			VersionParentID_2: dbItems[1].VersionParentID,
			Prev_2:            dbItems[1].Prev,
			Next_2:            dbItems[1].Next,

			ID_3:              dbItems[2].ID,
			ItemApiID_3:       dbItems[2].ItemApiID,
			CollectionID_3:    dbItems[2].CollectionID,
			IsDefault_3:       dbItems[2].IsDefault,
			BodyType_3:        dbItems[2].BodyType,
			Name_3:            dbItems[2].Name,
			VersionParentID_3: dbItems[2].VersionParentID,
			Prev_3:            dbItems[2].Prev,
			Next_3:            dbItems[2].Next,

			ID_4:              dbItems[3].ID,
			ItemApiID_4:       dbItems[3].ItemApiID,
			CollectionID_4:    dbItems[3].CollectionID,
			IsDefault_4:       dbItems[3].IsDefault,
			BodyType_4:        dbItems[3].BodyType,
			Name_4:            dbItems[3].Name,
			VersionParentID_4: dbItems[3].VersionParentID,
			Prev_4:            dbItems[3].Prev,
			Next_4:            dbItems[3].Next,

			ID_5:              dbItems[4].ID,
			ItemApiID_5:       dbItems[4].ItemApiID,
			CollectionID_5:    dbItems[4].CollectionID,
			IsDefault_5:       dbItems[4].IsDefault,
			BodyType_5:        dbItems[4].BodyType,
			Name_5:            dbItems[4].Name,
			VersionParentID_5: dbItems[4].VersionParentID,
			Prev_5:            dbItems[4].Prev,
			Next_5:            dbItems[4].Next,

			ID_6:              dbItems[5].ID,
			ItemApiID_6:       dbItems[5].ItemApiID,
			CollectionID_6:    dbItems[5].CollectionID,
			IsDefault_6:       dbItems[5].IsDefault,
			BodyType_6:        dbItems[5].BodyType,
			Name_6:            dbItems[5].Name,
			VersionParentID_6: dbItems[5].VersionParentID,
			Prev_6:            dbItems[5].Prev,
			Next_6:            dbItems[5].Next,

			ID_7:              dbItems[6].ID,
			ItemApiID_7:       dbItems[6].ItemApiID,
			CollectionID_7:    dbItems[6].CollectionID,
			IsDefault_7:       dbItems[6].IsDefault,
			BodyType_7:        dbItems[6].BodyType,
			Name_7:            dbItems[6].Name,
			VersionParentID_7: dbItems[6].VersionParentID,
			Prev_7:            dbItems[6].Prev,
			Next_7:            dbItems[6].Next,

			ID_8:              dbItems[7].ID,
			ItemApiID_8:       dbItems[7].ItemApiID,
			CollectionID_8:    dbItems[7].CollectionID,
			IsDefault_8:       dbItems[7].IsDefault,
			BodyType_8:        dbItems[7].BodyType,
			Name_8:            dbItems[7].Name,
			VersionParentID_8: dbItems[7].VersionParentID,
			Prev_8:            dbItems[7].Prev,
			Next_8:            dbItems[7].Next,

			ID_9:              dbItems[8].ID,
			ItemApiID_9:       dbItems[8].ItemApiID,
			CollectionID_9:    dbItems[8].CollectionID,
			IsDefault_9:       dbItems[8].IsDefault,
			BodyType_9:        dbItems[8].BodyType,
			Name_9:            dbItems[8].Name,
			VersionParentID_9: dbItems[8].VersionParentID,
			Prev_9:            dbItems[8].Prev,
			Next_9:            dbItems[8].Next,

			ID_10:              dbItems[9].ID,
			ItemApiID_10:       dbItems[9].ItemApiID,
			CollectionID_10:    dbItems[9].CollectionID,
			IsDefault_10:       dbItems[9].IsDefault,
			BodyType_10:        dbItems[9].BodyType,
			Name_10:            dbItems[9].Name,
			VersionParentID_10: dbItems[9].VersionParentID,
			Prev_10:            dbItems[9].Prev,
			Next_10:            dbItems[9].Next,
		}

		if err := iaes.Queries.CreateItemApiExampleBulk(ctx, params); err != nil {
			return fmt.Errorf("failed to create bulk examples: %w", err)
		}
	}

	return nil
}

func (iaes ItemApiExampleService) UpdateItemApiExample(ctx context.Context, item *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.UpdateItemApiExample(ctx, gen.UpdateItemApiExampleParams{
		ID:       item.ID,
		Name:     item.Name,
		BodyType: int8(item.BodyType),
	})
}

func (iaes ItemApiExampleService) UpdateItemApiExampleOrder(ctx context.Context, example *mitemapiexample.ItemApiExample) error {
	return iaes.Queries.UpdateItemApiExampleOrder(ctx, gen.UpdateItemApiExampleOrderParams{
		ID:   example.ID,
		Next: example.Next,
		Prev: example.Prev,
	})
}

func (iaes ItemApiExampleService) DeleteApiExample(ctx context.Context, id idwrap.IDWrap) error {
	return iaes.Queries.DeleteItemApiExample(ctx, id)
}
