package sexamplequery

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"sort"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexamplequery"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoQueryFound = errors.New("no error query found")

func SerializeQueryModelToDB(query mexamplequery.Query) gen.ExampleQuery {
	return gen.ExampleQuery{
		ID:            query.ID,
		ExampleID:     query.ExampleID,
		DeltaParentID: query.DeltaParentID,
		QueryKey:      query.QueryKey,
		Enable:        query.Enable,
		Description:   query.Description,
		Value:         query.Value,
	}
}

func SerializeQueryDBToModel(query gen.ExampleQuery) mexamplequery.Query {
	return mexamplequery.Query{
		ID:            query.ID,
		ExampleID:     query.ExampleID,
		DeltaParentID: query.DeltaParentID,
		QueryKey:      query.QueryKey,
		Enable:        query.Enable,
		Description:   query.Description,
		Value:         query.Value,
	}
}

type ExampleQueryService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) ExampleQueryService {
	return ExampleQueryService{queries: queries}
}

func (h ExampleQueryService) TX(tx *sql.Tx) ExampleQueryService {
	return ExampleQueryService{queries: h.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*ExampleQueryService, error) {
	queries, err := gen.Prepare(ctx, tx)
	if err != nil {
		return nil, err
	}
	service := ExampleQueryService{queries: queries}
	return &service, nil
}

func (h ExampleQueryService) GetExampleQuery(ctx context.Context, id idwrap.IDWrap) (mexamplequery.Query, error) {
	query, err := h.queries.GetQuery(ctx, id)
	if err != nil {
		return mexamplequery.Query{}, err
	}
	return SerializeQueryDBToModel(query), nil
}

func (h ExampleQueryService) GetExampleQueriesByExampleID(ctx context.Context, exampleID idwrap.IDWrap) ([]mexamplequery.Query, error) {
	queries, err := h.queries.GetQueriesByExampleID(ctx, exampleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mexamplequery.Query{}, ErrNoQueryFound
		}
		return nil, err
	}
	sort.Slice(queries, func(i, j int) bool {
		return queries[i].ID.Compare(queries[j].ID) < 0
	})
	return tgeneric.MassConvert(queries, SerializeQueryDBToModel), nil
}

func (h ExampleQueryService) GetExampleQueriesByExampleIDs(ctx context.Context, exampleIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mexamplequery.Query, error) {
	result := make(map[idwrap.IDWrap][]mexamplequery.Query, len(exampleIDs))
	if len(exampleIDs) == 0 {
		return result, nil
	}

	rows, err := h.queries.GetQueriesByExampleIDs(ctx, exampleIDs)
	if err != nil {
		if err == sql.ErrNoRows {
			return result, nil
		}
		return nil, err
	}

	for _, row := range rows {
		model := SerializeQueryDBToModel(row)
		exampleID := model.ExampleID
		result[exampleID] = append(result[exampleID], model)
	}

	for exampleID, queries := range result {
		sort.Slice(queries, func(i, j int) bool {
			return queries[i].ID.Compare(queries[j].ID) < 0
		})
		result[exampleID] = queries
	}

	return result, nil
}

func (h ExampleQueryService) GetExampleQueryByDeltaParentID(ctx context.Context, deltaParentID *idwrap.IDWrap) (mexamplequery.Query, error) {
	query, err := h.queries.GetQueryByDeltaParentID(ctx, deltaParentID)
	if err != nil {
		return mexamplequery.Query{}, err
	}
	return SerializeQueryDBToModel(query), nil
}

func (h ExampleQueryService) CreateExampleQuery(ctx context.Context, query mexamplequery.Query) error {
	return h.queries.CreateQuery(ctx, gen.CreateQueryParams{
		ID:            query.ID,
		ExampleID:     query.ExampleID,
		QueryKey:      query.QueryKey,
		Enable:        query.Enable,
		Description:   query.Description,
		Value:         query.Value,
		DeltaParentID: query.DeltaParentID,
	})
}

func (h ExampleQueryService) CreateExampleQueryDB(ctx context.Context, query gen.ExampleQuery) error {
	return h.queries.CreateQuery(ctx, gen.CreateQueryParams{
		ID:            query.ID,
		ExampleID:     query.ExampleID,
		QueryKey:      query.QueryKey,
		Enable:        query.Enable,
		Description:   query.Description,
		Value:         query.Value,
		DeltaParentID: query.DeltaParentID,
	})
}

func (h ExampleQueryService) CreateBulkQuery(ctx context.Context, queries []mexamplequery.Query) error {
	const sizeOfChunks = 10
	convertedItems := tgeneric.MassConvert(queries, SerializeQueryModelToDB)
	for headerChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(headerChunk) < sizeOfChunks {
			for _, header := range headerChunk {
				err := h.CreateExampleQueryDB(ctx, header)
				if err != nil {
					return err
				}
			}
			continue
		}
		item1 := headerChunk[0]
		item2 := headerChunk[1]
		item3 := headerChunk[2]
		item4 := headerChunk[3]
		item5 := headerChunk[4]
		item6 := headerChunk[5]
		item7 := headerChunk[6]
		item8 := headerChunk[7]
		item9 := headerChunk[8]
		item10 := headerChunk[9]

		params := gen.CreateQueryBulkParams{
			// 1
			ID:            item1.ID,
			ExampleID:     item1.ExampleID,
			DeltaParentID: item1.DeltaParentID,
			QueryKey:      item1.QueryKey,
			Enable:        item1.Enable,
			Description:   item1.Description,
			Value:         item1.Value,
			// 2
			ID_2:            item2.ID,
			ExampleID_2:     item2.ExampleID,
			DeltaParentID_2: item2.DeltaParentID,
			QueryKey_2:      item2.QueryKey,
			Enable_2:        item2.Enable,
			Description_2:   item2.Description,
			Value_2:         item2.Value,
			// 3
			ID_3:            item3.ID,
			ExampleID_3:     item3.ExampleID,
			DeltaParentID_3: item3.DeltaParentID,
			QueryKey_3:      item3.QueryKey,
			Enable_3:        item3.Enable,
			Description_3:   item3.Description,
			Value_3:         item3.Value,
			// 4
			ID_4:            item4.ID,
			ExampleID_4:     item4.ExampleID,
			DeltaParentID_4: item4.DeltaParentID,
			QueryKey_4:      item4.QueryKey,
			Enable_4:        item4.Enable,
			Description_4:   item4.Description,
			Value_4:         item4.Value,
			// 5
			ID_5:            item5.ID,
			ExampleID_5:     item5.ExampleID,
			DeltaParentID_5: item5.DeltaParentID,
			QueryKey_5:      item5.QueryKey,
			Enable_5:        item5.Enable,
			Description_5:   item5.Description,
			Value_5:         item5.Value,
			// 6
			ID_6:            item6.ID,
			ExampleID_6:     item6.ExampleID,
			DeltaParentID_6: item6.DeltaParentID,
			QueryKey_6:      item6.QueryKey,
			Enable_6:        item6.Enable,
			Description_6:   item6.Description,
			Value_6:         item6.Value,
			// 7
			ID_7:            item7.ID,
			ExampleID_7:     item7.ExampleID,
			DeltaParentID_7: item7.DeltaParentID,
			QueryKey_7:      item7.QueryKey,
			Enable_7:        item7.Enable,
			Description_7:   item7.Description,
			Value_7:         item7.Value,
			// 8
			ID_8:            item8.ID,
			ExampleID_8:     item8.ExampleID,
			DeltaParentID_8: item8.DeltaParentID,
			QueryKey_8:      item8.QueryKey,
			Enable_8:        item8.Enable,
			Description_8:   item8.Description,
			Value_8:         item8.Value,
			// 9
			ID_9:            item9.ID,
			ExampleID_9:     item9.ExampleID,
			DeltaParentID_9: item9.DeltaParentID,
			QueryKey_9:      item9.QueryKey,
			Enable_9:        item9.Enable,
			Description_9:   item9.Description,
			Value_9:         item9.Value,
			// 10
			ID_10:            item10.ID,
			ExampleID_10:     item10.ExampleID,
			DeltaParentID_10: item10.DeltaParentID,
			QueryKey_10:      item10.QueryKey,
			Enable_10:        item10.Enable,
			Description_10:   item10.Description,
			Value_10:         item10.Value,
		}
		if err := h.queries.CreateQueryBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (h ExampleQueryService) UpdateExampleQuery(ctx context.Context, query mexamplequery.Query) error {
	return h.queries.UpdateQuery(ctx, gen.UpdateQueryParams{
		ID:          query.ID,
		QueryKey:    query.QueryKey,
		Enable:      query.Enable,
		Description: query.Description,
		Value:       query.Value,
	})
}

func (h ExampleQueryService) DeleteExampleQuery(ctx context.Context, id idwrap.IDWrap) error {
	return h.queries.DeleteQuery(ctx, id)
}

func (h ExampleQueryService) ResetExampleQueryDelta(ctx context.Context, id idwrap.IDWrap) error {
	query, err := h.GetExampleQuery(ctx, id)
	if err != nil {
		return err
	}

	query.DeltaParentID = nil
	query.QueryKey = ""
	query.Enable = false
	query.Description = ""
	query.Value = ""

	return h.UpdateExampleQuery(ctx, query)
}
