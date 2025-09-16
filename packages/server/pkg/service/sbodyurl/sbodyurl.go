package sbodyurl

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mbodyurl"
	"the-dev-tools/server/pkg/movable"
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

// Repository returns a MovableRepository adapter for URL-encoded bodies
func (bues BodyURLEncodedService) Repository() *BodyUrlEncodedMovableRepository {
	return NewBodyUrlEncodedMovableRepository(bues.queries)
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
	r, err := bues.queries.GetBodyUrlEncoded(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNoBodyUrlEncodedFound
		}
		return nil, err
	}
	m := mbodyurl.BodyURLEncoded{
		ID: r.ID, ExampleID: r.ExampleID, BodyKey: r.BodyKey,
		Enable: r.Enable, Description: r.Description, Value: r.Value,
	}
	if r.DeltaParentID != nil {
		v := *r.DeltaParentID
		m.DeltaParentID = &v
	}
	return &m, nil
}

func (bues BodyURLEncodedService) GetBodyURLEncodedByExampleID(ctx context.Context, exampleID idwrap.IDWrap) ([]mbodyurl.BodyURLEncoded, error) {
	rows, err := bues.queries.GetBodyUrlEncodedsByExampleID(ctx, exampleID)
	if err != nil {
		return nil, err
	}
	out := make([]mbodyurl.BodyURLEncoded, 0, len(rows))
	for _, r := range rows {
		item := mbodyurl.BodyURLEncoded{
			ID:          r.ID,
			ExampleID:   r.ExampleID,
			BodyKey:     r.BodyKey,
			Enable:      r.Enable,
			Description: r.Description,
			Value:       r.Value,
		}
		if r.DeltaParentID != nil {
			v := *r.DeltaParentID
			item.DeltaParentID = &v
		}
		out = append(out, item)
	}
	return out, nil
}

func (bues BodyURLEncodedService) GetBodyURLEncodedByExampleIDs(ctx context.Context, exampleIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]mbodyurl.BodyURLEncoded, error) {
	result := make(map[idwrap.IDWrap][]mbodyurl.BodyURLEncoded, len(exampleIDs))
	if len(exampleIDs) == 0 {
		return result, nil
	}

	rows, err := bues.queries.GetBodyUrlEncodedsByExampleIDs(ctx, exampleIDs)
	if err != nil {
		if err == sql.ErrNoRows {
			return result, nil
		}
		return nil, err
	}

	for _, r := range rows {
		item := mbodyurl.BodyURLEncoded{
			ID:          r.ID,
			ExampleID:   r.ExampleID,
			BodyKey:     r.BodyKey,
			Enable:      r.Enable,
			Description: r.Description,
			Value:       r.Value,
		}
		if r.DeltaParentID != nil {
			v := *r.DeltaParentID
			item.DeltaParentID = &v
		}
		result[item.ExampleID] = append(result[item.ExampleID], item)
	}

	return result, nil
}

// GetBodyURLEncodedByExampleIDOrdered returns URL-encoded bodies ordered by linked list for the example
func (bues BodyURLEncodedService) GetBodyURLEncodedByExampleIDOrdered(ctx context.Context, exampleID idwrap.IDWrap) ([]mbodyurl.BodyURLEncoded, error) {
	// Safely reconstruct order in memory to avoid recursive CTE hangs
	// even if the linked list contains cycles or partial corruption.
	allRows, err := bues.queries.GetBodyUrlEncodedsByExampleID(ctx, exampleID)
	if err != nil {
		return nil, err
	}
	if len(allRows) == 0 {
		return []mbodyurl.BodyURLEncoded{}, nil
	}
	// Build a map id -> model for quick lookup
	byID := make(map[idwrap.IDWrap]mbodyurl.BodyURLEncoded, len(allRows))
	for _, r := range allRows {
		item := mbodyurl.BodyURLEncoded{
			ID:          r.ID,
			ExampleID:   r.ExampleID,
			BodyKey:     r.BodyKey,
			Enable:      r.Enable,
			Description: r.Description,
			Value:       r.Value,
		}
		if r.DeltaParentID != nil {
			v := *r.DeltaParentID
			item.DeltaParentID = &v
		}
		byID[r.ID] = item
	}
	// Use the Movable repo traversal to get the ordered IDs
	repo := NewBodyUrlEncodedMovableRepository(bues.queries)
	items, err := repo.GetItemsByParent(ctx, exampleID, movable.RequestListTypeBodyUrlEncoded)
	if err != nil {
		return nil, err
	}
	ordered := make([]mbodyurl.BodyURLEncoded, 0, len(items))
	for _, it := range items {
		if m, ok := byID[it.ID]; ok {
			ordered = append(ordered, m)
		}
	}
	return ordered, nil
}

func (bues BodyURLEncodedService) UpdateBodyURLEncodedOrder(ctx context.Context, id, exampleID idwrap.IDWrap, prev, next *idwrap.IDWrap) error {
	return bues.queries.UpdateBodyUrlEncodedOrder(ctx, gen.UpdateBodyUrlEncodedOrderParams{
		Prev:      btsID(prev),
		Next:      btsID(next),
		ID:        id,
		ExampleID: exampleID,
	})
}

func (bues BodyURLEncodedService) UpdateBodyURLEncodedPrev(ctx context.Context, id, exampleID idwrap.IDWrap, prev *idwrap.IDWrap) error {
	return bues.queries.UpdateBodyUrlEncodedPrev(ctx, gen.UpdateBodyUrlEncodedPrevParams{
		Prev:      btsID(prev),
		ID:        id,
		ExampleID: exampleID,
	})
}

func (bues BodyURLEncodedService) UpdateBodyURLEncodedNext(ctx context.Context, id, exampleID idwrap.IDWrap, next *idwrap.IDWrap) error {
	return bues.queries.UpdateBodyUrlEncodedNext(ctx, gen.UpdateBodyUrlEncodedNextParams{
		Next:      btsID(next),
		ID:        id,
		ExampleID: exampleID,
	})
}

func (bues BodyURLEncodedService) GetBodyURLEncodedTail(ctx context.Context, exampleID idwrap.IDWrap) (*mbodyurl.BodyURLEncoded, error) {
	row, err := bues.queries.GetBodyUrlEncodedTail(ctx, exampleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	m := DeserializeGenToModel(row)
	return &m, nil
}

func (bues BodyURLEncodedService) GetBodyURLEncodedLinks(ctx context.Context, id idwrap.IDWrap) (exampleID idwrap.IDWrap, prev, next *idwrap.IDWrap, err error) {
	row, err := bues.queries.GetBodyUrlEncodedLinks(ctx, id)
	if err != nil {
		return idwrap.IDWrap{}, nil, nil, err
	}
	exampleID = row.ExampleID
	if len(row.Prev) > 0 {
		v := idwrap.NewFromBytesMust(row.Prev)
		prev = &v
	}
	if len(row.Next) > 0 {
		v := idwrap.NewFromBytesMust(row.Next)
		next = &v
	}
	return
}

// AppendAtEnd links the given id as the new tail in example's linked list
func (bues BodyURLEncodedService) AppendAtEnd(ctx context.Context, exampleID, id idwrap.IDWrap) error {
	// Find a real tail that is not the new row
	tailRow, err := bues.queries.GetBodyUrlEncodedTailExcludingID(ctx, gen.GetBodyUrlEncodedTailExcludingIDParams{ExampleID: exampleID, ID: id})
	if err == sql.ErrNoRows {
		// First item in list: leave prev/next as NULL
		return nil
	}
	if err != nil {
		return err
	}
	tail := DeserializeGenToModel(tailRow)
	tailPtr := &tail
	// Set id.prev = tail, id.next = NULL
	if err := bues.UpdateBodyURLEncodedOrder(ctx, id, exampleID, ptrID(tailPtr), nil); err != nil {
		return err
	}
	// If tail exists, set tail.next = id
	if err := bues.UpdateBodyURLEncodedNext(ctx, tail.ID, exampleID, &id); err != nil {
		return err
	}
	return nil
}

// MoveUrlEncoded moves id before/after target within example linked list
func (bues BodyURLEncodedService) MoveUrlEncoded(ctx context.Context, id, exampleID idwrap.IDWrap, afterID, beforeID *idwrap.IDWrap) error {
	if (afterID == nil && beforeID == nil) || (afterID != nil && beforeID != nil) {
		return errors.New("exactly one of afterID or beforeID must be specified")
	}
	exID, origPrev, origNext, err := bues.GetBodyURLEncodedLinks(ctx, id)
	if err != nil {
		return err
	}
	if exID.Compare(exampleID) != 0 {
		return errors.New("item does not belong to specified example")
	}

	var target idwrap.IDWrap
	if afterID != nil {
		target = *afterID
	} else {
		target = *beforeID
	}
	targetEx, targetPrev, targetNext, err := bues.GetBodyURLEncodedLinks(ctx, target)
	if err != nil {
		return err
	}
	if targetEx.Compare(exampleID) != 0 {
		return errors.New("target does not belong to specified example")
	}

	// No-op checks
	if afterID != nil && targetPrev != nil && targetPrev.Compare(id) == 0 {
		return nil
	}
	if beforeID != nil && targetNext != nil && targetNext.Compare(id) == 0 {
		return nil
	}

	// Unlink id
	if origPrev != nil {
		if err := bues.UpdateBodyURLEncodedNext(ctx, *origPrev, exampleID, origNext); err != nil {
			return err
		}
	}
	if origNext != nil {
		if err := bues.UpdateBodyURLEncodedPrev(ctx, *origNext, exampleID, origPrev); err != nil {
			return err
		}
	}

	// Insert relative to target
	var newPrev, newNext *idwrap.IDWrap
	if afterID != nil {
		newPrev = &target
		newNext = targetNext
		if err := bues.UpdateBodyURLEncodedNext(ctx, target, exampleID, &id); err != nil {
			return err
		}
		if targetNext != nil {
			if err := bues.UpdateBodyURLEncodedPrev(ctx, *targetNext, exampleID, &id); err != nil {
				return err
			}
		}
	} else { // before
		newPrev = targetPrev
		newNext = &target
		if targetPrev != nil {
			if err := bues.UpdateBodyURLEncodedNext(ctx, *targetPrev, exampleID, &id); err != nil {
				return err
			}
		}
		if err := bues.UpdateBodyURLEncodedPrev(ctx, target, exampleID, &id); err != nil {
			return err
		}
	}
	// Update id
	if err := bues.UpdateBodyURLEncodedOrder(ctx, id, exampleID, newPrev, newNext); err != nil {
		return err
	}
	return nil
}

// EnsureDeltaProxy ensures an id exists in the given delta example; if id belongs to origin, creates a proxy and appends it.
func (bues BodyURLEncodedService) EnsureDeltaProxy(ctx context.Context, deltaExampleID, id idwrap.IDWrap) (idwrap.IDWrap, error) {
	ex, _, _, err := bues.GetBodyURLEncodedLinks(ctx, id)
	if err != nil {
		return idwrap.IDWrap{}, err
	}
	if ex.Compare(deltaExampleID) == 0 {
		return id, nil
	}
	// If a delta row already exists for this origin in the target example, reuse it
	if rows, err2 := bues.queries.GetBodyUrlEncodedsByDeltaParentID(ctx, &id); err2 == nil {
		for _, r := range rows {
			if r.ExampleID.Compare(deltaExampleID) == 0 {
				return r.ID, nil
			}
		}
	}
	// load origin data
	origin, err := bues.GetBodyURLEncoded(ctx, id)
	if err != nil {
		return idwrap.IDWrap{}, err
	}
	newID := idwrap.NewNow()
	m := &mbodyurl.BodyURLEncoded{
		ID:            newID,
		ExampleID:     deltaExampleID,
		DeltaParentID: &origin.ID,
		BodyKey:       origin.BodyKey,
		Enable:        origin.Enable,
		Description:   origin.Description,
		Value:         origin.Value,
	}
	if err := bues.CreateBodyURLEncoded(ctx, m); err != nil {
		return idwrap.IDWrap{}, err
	}
	if err := bues.AppendAtEnd(ctx, deltaExampleID, newID); err != nil {
		return idwrap.IDWrap{}, err
	}
	return newID, nil
}

func ptrID(m *mbodyurl.BodyURLEncoded) *idwrap.IDWrap {
	if m == nil {
		return nil
	}
	v := m.ID
	return &v
}

// btsID converts an optional idwrap to []byte for sqlc params
func btsID(id *idwrap.IDWrap) []byte {
	if id == nil {
		return nil
	}
	return id.Bytes()
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

	// The bulk insert SQL expects exactly 7 items per batch
	const batchSize = 7
	for i := 0; i < len(bodyForms); i += batchSize {
		end := i + batchSize
		if end > len(bodyForms) {
			end = len(bodyForms)
		}

		batch := bodyForms[i:end]

		// For batches with fewer than 7 items, use individual inserts
		if len(batch) < batchSize {
			for _, body := range batch {
				err := bues.CreateBodyURLEncoded(ctx, &body)
				if err != nil {
					return err
				}
			}
			continue
		}

		params := gen.CreateBodyUrlEncodedBulkParams{}

		// Set all 7 batch parameters
		params.ID = batch[0].ID
		params.ExampleID = batch[0].ExampleID
		params.DeltaParentID = batch[0].DeltaParentID
		params.BodyKey = batch[0].BodyKey
		params.Enable = batch[0].Enable
		params.Description = batch[0].Description
		params.Value = batch[0].Value

		params.ID_2 = batch[1].ID
		params.ExampleID_2 = batch[1].ExampleID
		params.DeltaParentID_2 = batch[1].DeltaParentID
		params.BodyKey_2 = batch[1].BodyKey
		params.Enable_2 = batch[1].Enable
		params.Description_2 = batch[1].Description
		params.Value_2 = batch[1].Value

		params.ID_3 = batch[2].ID
		params.ExampleID_3 = batch[2].ExampleID
		params.DeltaParentID_3 = batch[2].DeltaParentID
		params.BodyKey_3 = batch[2].BodyKey
		params.Enable_3 = batch[2].Enable
		params.Description_3 = batch[2].Description
		params.Value_3 = batch[2].Value

		params.ID_4 = batch[3].ID
		params.ExampleID_4 = batch[3].ExampleID
		params.DeltaParentID_4 = batch[3].DeltaParentID
		params.BodyKey_4 = batch[3].BodyKey
		params.Enable_4 = batch[3].Enable
		params.Description_4 = batch[3].Description
		params.Value_4 = batch[3].Value

		params.ID_5 = batch[4].ID
		params.ExampleID_5 = batch[4].ExampleID
		params.DeltaParentID_5 = batch[4].DeltaParentID
		params.BodyKey_5 = batch[4].BodyKey
		params.Enable_5 = batch[4].Enable
		params.Description_5 = batch[4].Description
		params.Value_5 = batch[4].Value

		params.ID_6 = batch[5].ID
		params.ExampleID_6 = batch[5].ExampleID
		params.DeltaParentID_6 = batch[5].DeltaParentID
		params.BodyKey_6 = batch[5].BodyKey
		params.Enable_6 = batch[5].Enable
		params.Description_6 = batch[5].Description
		params.Value_6 = batch[5].Value

		params.ID_7 = batch[6].ID
		params.ExampleID_7 = batch[6].ExampleID
		params.DeltaParentID_7 = batch[6].DeltaParentID
		params.BodyKey_7 = batch[6].BodyKey
		params.Enable_7 = batch[6].Enable
		params.Description_7 = batch[6].Description
		params.Value_7 = batch[6].Value

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
	// Load the current (delta or mixed) row
	bodyURLEncoded, err := bues.GetBodyURLEncoded(ctx, id)
	if err != nil {
		return err
	}

	// If this item has a delta parent, restore fields from the parent
	if bodyURLEncoded.DeltaParentID != nil {
		parent, err := bues.GetBodyURLEncoded(ctx, *bodyURLEncoded.DeltaParentID)
		if err != nil {
			return err
		}

		bodyURLEncoded.BodyKey = parent.BodyKey
		bodyURLEncoded.Enable = parent.Enable
		bodyURLEncoded.Description = parent.Description
		bodyURLEncoded.Value = parent.Value

		// IMPORTANT: keep DeltaParentID to preserve delta linkage
		return bues.UpdateBodyURLEncoded(ctx, bodyURLEncoded)
	}

	// No parent: clear values but keep identity as-is
	bodyURLEncoded.BodyKey = ""
	bodyURLEncoded.Enable = false
	bodyURLEncoded.Description = ""
	bodyURLEncoded.Value = ""
	return bues.UpdateBodyURLEncoded(ctx, bodyURLEncoded)
}
