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
		Prev:          query.Prev,
		Next:          query.Next,
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
		Prev:          query.Prev,
		Next:          query.Next,
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

func convertGetQueriesByExampleIDRowToQuery(row gen.GetQueriesByExampleIDRow) mexamplequery.Query {
	return mexamplequery.Query{
		ID:            row.ID,
		ExampleID:     row.ExampleID,
		DeltaParentID: row.DeltaParentID,
		QueryKey:      row.QueryKey,
		Enable:        row.Enable,
		Description:   row.Description,
		Value:         row.Value,
		Prev:          nil, // Row type doesn't include prev/next
		Next:          nil,
	}
}

func (h ExampleQueryService) GetExampleQueriesByExampleID(ctx context.Context, exampleID idwrap.IDWrap) ([]mexamplequery.Query, error) {
	queryRows, err := h.queries.GetQueriesByExampleID(ctx, exampleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []mexamplequery.Query{}, ErrNoQueryFound
		}
		return nil, err
	}
	sort.Slice(queryRows, func(i, j int) bool {
		return queryRows[i].ID.Compare(queryRows[j].ID) < 0
	})
	return tgeneric.MassConvert(queryRows, convertGetQueriesByExampleIDRowToQuery), nil
}

func convertGetQueryByDeltaParentIDRowToQuery(row gen.GetQueryByDeltaParentIDRow) mexamplequery.Query {
	return mexamplequery.Query{
		ID:            row.ID,
		ExampleID:     row.ExampleID,
		DeltaParentID: row.DeltaParentID,
		QueryKey:      row.QueryKey,
		Enable:        row.Enable,
		Description:   row.Description,
		Value:         row.Value,
		Prev:          nil, // Row type doesn't include prev/next
		Next:          nil,
	}
}

func (h ExampleQueryService) GetExampleQueryByDeltaParentID(ctx context.Context, deltaParentID *idwrap.IDWrap) (mexamplequery.Query, error) {
	query, err := h.queries.GetQueryByDeltaParentID(ctx, deltaParentID)
	if err != nil {
		return mexamplequery.Query{}, err
	}
	return convertGetQueryByDeltaParentIDRowToQuery(query), nil
}

func (h ExampleQueryService) CreateExampleQuery(ctx context.Context, query mexamplequery.Query) error {
	// Find the last query in the list to maintain order
	queries, err := h.queries.GetQueriesByExampleIDOrdered(ctx, query.ExampleID)
	if err != nil {
		return err
	}
	
	var prevID, nextID *idwrap.IDWrap
	
	// If query already has prev/next set, use those (for manual control)
	if query.Prev != nil || query.Next != nil {
		prevID = query.Prev
		nextID = query.Next
	} else if len(queries) > 0 {
		// Set this query to be last in the list
		lastQuery := queries[len(queries)-1]
		prevID = &lastQuery.ID
		nextID = nil
		
		// Update the previous last query to point to this new query
		err = h.queries.UpdateQueryNext(ctx, gen.UpdateQueryNextParams{
			ID:   lastQuery.ID,
			Next: &query.ID,
		})
		if err != nil {
			return err
		}
	}
	
	return h.queries.CreateQuery(ctx, gen.CreateQueryParams{
		ID:            query.ID,
		ExampleID:     query.ExampleID,
		QueryKey:      query.QueryKey,
		Enable:        query.Enable,
		Description:   query.Description,
		Value:         query.Value,
		DeltaParentID: query.DeltaParentID,
		Prev:          prevID,
		Next:          nextID,
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
		Prev:          query.Prev,
		Next:          query.Next,
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

// MoveQuery moves a query to a new position relative to a target
func (h ExampleQueryService) MoveQuery(ctx context.Context, queryID, targetID idwrap.IDWrap, position string) error {
	// This implementation assumes the transaction will be managed at a higher level
	// For now, we'll implement it without explicit transaction management
	queries := h.queries
	
	// 1. Get the query to move
	query, err := queries.GetQuery(ctx, queryID)
	if err != nil {
		return err
	}
	
	// 2. Remove query from current position
	if query.Prev != nil {
		err = queries.UpdateQueryNext(ctx, gen.UpdateQueryNextParams{
			ID:   *query.Prev,
			Next: query.Next,
		})
		if err != nil {
			return err
		}
	}
	
	if query.Next != nil {
		err = queries.UpdateQueryPrev(ctx, gen.UpdateQueryPrevParams{
			ID:   *query.Next,
			Prev: query.Prev,
		})
		if err != nil {
			return err
		}
	}
	
	// 3. Insert at new position
	if position == "before" {
		target, err := queries.GetQuery(ctx, targetID)
		if err != nil {
			return err
		}
		
		// Update query's pointers
		err = queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   queryID,
			Prev: target.Prev,
			Next: &targetID,
		})
		if err != nil {
			return err
		}
		
		// Update target's prev
		err = queries.UpdateQueryPrev(ctx, gen.UpdateQueryPrevParams{
			ID:   targetID,
			Prev: &queryID,
		})
		if err != nil {
			return err
		}
		
		// Update previous item's next if exists
		if target.Prev != nil {
			err = queries.UpdateQueryNext(ctx, gen.UpdateQueryNextParams{
				ID:   *target.Prev,
				Next: &queryID,
			})
			if err != nil {
				return err
			}
		}
	} else if position == "after" {
		target, err := queries.GetQuery(ctx, targetID)
		if err != nil {
			return err
		}
		
		// Update query's pointers
		err = queries.UpdateQueryOrder(ctx, gen.UpdateQueryOrderParams{
			ID:   queryID,
			Prev: &targetID,
			Next: target.Next,
		})
		if err != nil {
			return err
		}
		
		// Update target's next
		err = queries.UpdateQueryNext(ctx, gen.UpdateQueryNextParams{
			ID:   targetID,
			Next: &queryID,
		})
		if err != nil {
			return err
		}
		
		// Update next item's prev if exists
		if target.Next != nil {
			err = queries.UpdateQueryPrev(ctx, gen.UpdateQueryPrevParams{
				ID:   *target.Next,
				Prev: &queryID,
			})
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

// GetQueriesByExampleIDOrdered returns queries in their linked list order
func (h ExampleQueryService) GetQueriesByExampleIDOrdered(ctx context.Context, exampleID idwrap.IDWrap) ([]mexamplequery.Query, error) {
	dbQueries, err := h.queries.GetQueriesByExampleIDOrdered(ctx, exampleID)
	if err != nil {
		return nil, err
	}
	
	return tgeneric.MassConvert(dbQueries, SerializeQueryDBToModel), nil
}
