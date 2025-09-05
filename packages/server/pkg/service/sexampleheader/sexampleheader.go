package sexampleheader

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/mexampleheader"
	"the-dev-tools/server/pkg/movable"
	"the-dev-tools/server/pkg/translate/tgeneric"
)

var ErrNoHeaderFound = errors.New("not header found")

type HeaderService struct {
	queries *gen.Queries
}

func New(queries *gen.Queries) HeaderService {
	return HeaderService{queries: queries}
}

func (h HeaderService) TX(tx *sql.Tx) HeaderService {
	return HeaderService{queries: h.queries.WithTx(tx)}
}

func NewTX(ctx context.Context, tx *sql.Tx) (*HeaderService, error) {
	queries := gen.New(tx)
	headerService := HeaderService{
		queries: queries,
	}

	return &headerService, nil
}

func SerializeHeaderModelToDB(header gen.ExampleHeader) mexampleheader.Header {
	return mexampleheader.Header{
		ID:            header.ID,
		ExampleID:     header.ExampleID,
		DeltaParentID: header.DeltaParentID,
		HeaderKey:     header.HeaderKey,
		Enable:        header.Enable,
		Description:   header.Description,
		Value:         header.Value,
		Prev:          header.Prev,
		Next:          header.Next,
	}
}

func SerializeHeaderDBToModel(header mexampleheader.Header) gen.ExampleHeader {
	return gen.ExampleHeader{
		ID:            header.ID,
		ExampleID:     header.ExampleID,
		DeltaParentID: header.DeltaParentID,
		HeaderKey:     header.HeaderKey,
		Enable:        header.Enable,
		Description:   header.Description,
		Value:         header.Value,
		Prev:          header.Prev,
		Next:          header.Next,
	}
}

func (h HeaderService) GetHeaderByExampleID(ctx context.Context, exampleID idwrap.IDWrap) ([]mexampleheader.Header, error) {
	dbHeaders, err := h.queries.GetHeadersByExampleID(ctx, exampleID)
	if err != nil {
		return nil, err
	}

	var headers []mexampleheader.Header
	for _, dbHeader := range dbHeaders {
		header := SerializeHeaderModelToDB(dbHeader)
		headers = append(headers, header)
	}

	return headers, nil
}

func (h HeaderService) GetHeaderByExampleIDOrdered(ctx context.Context, exampleID idwrap.IDWrap) ([]mexampleheader.Header, error) {
	// Use the ordered query that traverses the linked list
	dbHeaders, err := h.queries.GetHeadersByExampleIDOrdered(ctx, gen.GetHeadersByExampleIDOrderedParams{
		ExampleID:   exampleID,
		ExampleID_2: exampleID,
	})
	if err != nil {
		return nil, err
	}

	var headers []mexampleheader.Header
	for _, dbHeader := range dbHeaders {
		// Convert the query row to model Header
		var deltaParentID *idwrap.IDWrap
		if dbHeader.DeltaParentID != nil {
			id := idwrap.NewFromBytesMust(dbHeader.DeltaParentID)
			deltaParentID = &id
		}

		var prev *idwrap.IDWrap
		if dbHeader.Prev != nil {
			id := idwrap.NewFromBytesMust(dbHeader.Prev)
			prev = &id
		}

		var next *idwrap.IDWrap
		if dbHeader.Next != nil {
			id := idwrap.NewFromBytesMust(dbHeader.Next)
			next = &id
		}

		header := mexampleheader.Header{
			ID:            idwrap.NewFromBytesMust(dbHeader.ID),
			ExampleID:     idwrap.NewFromBytesMust(dbHeader.ExampleID),
			DeltaParentID: deltaParentID,
			HeaderKey:     dbHeader.HeaderKey,
			Enable:        dbHeader.Enable,
			Description:   dbHeader.Description,
			Value:         dbHeader.Value,
			Prev:          prev,
			Next:          next,
		}
		headers = append(headers, header)
	}

	return headers, nil
}

func (h HeaderService) GetHeaderByDeltaParentID(ctx context.Context, deltaParentID idwrap.IDWrap) ([]mexampleheader.Header, error) {
	dbHeader, err := h.queries.GetHeaderByDeltaParentID(ctx, &deltaParentID)
	if err != nil {
		return nil, err
	}

	header := SerializeHeaderModelToDB(dbHeader)
	return []mexampleheader.Header{header}, nil
}

func (h HeaderService) GetHeaderByID(ctx context.Context, headerID idwrap.IDWrap) (mexampleheader.Header, error) {
	dbHeader, err := h.queries.GetHeader(ctx, headerID)
	if err != nil {
		return mexampleheader.Header{}, err
	}

	header := SerializeHeaderModelToDB(dbHeader)
	return header, nil
}

func (h HeaderService) CreateHeader(ctx context.Context, header mexampleheader.Header) error {
	return h.queries.CreateHeader(ctx, gen.CreateHeaderParams{
		ID:            header.ID,
		ExampleID:     header.ExampleID,
		DeltaParentID: header.DeltaParentID,
		HeaderKey:     header.HeaderKey,
		Enable:        header.Enable,
		Description:   header.Description,
		Value:         header.Value,
		Prev:          header.Prev,
		Next:          header.Next,
	})
}

func (h HeaderService) CreateHeaderModel(ctx context.Context, header gen.ExampleHeader) error {
	return h.queries.CreateHeader(ctx, gen.CreateHeaderParams{
		ID:            header.ID,
		ExampleID:     header.ExampleID,
		DeltaParentID: header.DeltaParentID,
		HeaderKey:     header.HeaderKey,
		Enable:        header.Enable,
		Description:   header.Description,
		Value:         header.Value,
		Prev:          header.Prev,
		Next:          header.Next,
	})
}

func (h HeaderService) CreateBulkHeader(ctx context.Context, headers []mexampleheader.Header) error {
	const sizeOfChunks = 15
	convertedItems := tgeneric.MassConvert(headers, SerializeHeaderDBToModel)
	for headerChunk := range slices.Chunk(convertedItems, sizeOfChunks) {
		if len(headerChunk) < sizeOfChunks {
			for _, header := range headerChunk {
				err := h.CreateHeaderModel(ctx, header)
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
		item11 := headerChunk[10]
		item12 := headerChunk[11]
		item13 := headerChunk[12]
		item14 := headerChunk[13]
		item15 := headerChunk[14]

		params := gen.CreateHeaderBulkParams{
			// 1
			ID:            item1.ID,
			ExampleID:     item1.ExampleID,
			DeltaParentID: item1.DeltaParentID,
			HeaderKey:     item1.HeaderKey,
			Enable:        item1.Enable,
			Description:   item1.Description,
			Value:         item1.Value,
			Prev:          item1.Prev,
			Next:          item1.Next,
			// 2
			ID_2:            item2.ID,
			ExampleID_2:     item2.ExampleID,
			DeltaParentID_2: item2.DeltaParentID,
			HeaderKey_2:     item2.HeaderKey,
			Enable_2:        item2.Enable,
			Description_2:   item2.Description,
			Value_2:         item2.Value,
			Prev_2:          item2.Prev,
			Next_2:          item2.Next,
			// 3
			ID_3:            item3.ID,
			ExampleID_3:     item3.ExampleID,
			DeltaParentID_3: item3.DeltaParentID,
			HeaderKey_3:     item3.HeaderKey,
			Enable_3:        item3.Enable,
			Description_3:   item3.Description,
			Value_3:         item3.Value,
			Prev_3:          item3.Prev,
			Next_3:          item3.Next,
			// 4
			ID_4:            item4.ID,
			ExampleID_4:     item4.ExampleID,
			DeltaParentID_4: item4.DeltaParentID,
			HeaderKey_4:     item4.HeaderKey,
			Enable_4:        item4.Enable,
			Description_4:   item4.Description,
			Value_4:         item4.Value,
			Prev_4:          item4.Prev,
			Next_4:          item4.Next,
			// 5
			ID_5:            item5.ID,
			ExampleID_5:     item5.ExampleID,
			DeltaParentID_5: item5.DeltaParentID,
			HeaderKey_5:     item5.HeaderKey,
			Enable_5:        item5.Enable,
			Description_5:   item5.Description,
			Value_5:         item5.Value,
			Prev_5:          item5.Prev,
			Next_5:          item5.Next,
			// 6
			ID_6:            item6.ID,
			ExampleID_6:     item6.ExampleID,
			DeltaParentID_6: item6.DeltaParentID,
			HeaderKey_6:     item6.HeaderKey,
			Enable_6:        item6.Enable,
			Description_6:   item6.Description,
			Value_6:         item6.Value,
			Prev_6:          item6.Prev,
			Next_6:          item6.Next,
			// 7
			ID_7:            item7.ID,
			ExampleID_7:     item7.ExampleID,
			DeltaParentID_7: item7.DeltaParentID,
			HeaderKey_7:     item7.HeaderKey,
			Enable_7:        item7.Enable,
			Description_7:   item7.Description,
			Value_7:         item7.Value,
			Prev_7:          item7.Prev,
			Next_7:          item7.Next,
			// 8
			ID_8:            item8.ID,
			ExampleID_8:     item8.ExampleID,
			DeltaParentID_8: item8.DeltaParentID,
			HeaderKey_8:     item8.HeaderKey,
			Enable_8:        item8.Enable,
			Description_8:   item8.Description,
			Value_8:         item8.Value,
			Prev_8:          item8.Prev,
			Next_8:          item8.Next,
			// 9
			ID_9:            item9.ID,
			ExampleID_9:     item9.ExampleID,
			DeltaParentID_9: item9.DeltaParentID,
			HeaderKey_9:     item9.HeaderKey,
			Enable_9:        item9.Enable,
			Description_9:   item9.Description,
			Value_9:         item9.Value,
			Prev_9:          item9.Prev,
			Next_9:          item9.Next,
			// 10
			ID_10:            item10.ID,
			ExampleID_10:     item10.ExampleID,
			DeltaParentID_10: item10.DeltaParentID,
			HeaderKey_10:     item10.HeaderKey,
			Enable_10:        item10.Enable,
			Description_10:   item10.Description,
			Value_10:         item10.Value,
			Prev_10:          item10.Prev,
			Next_10:          item10.Next,
			// 11
			ID_11:            item11.ID,
			ExampleID_11:     item11.ExampleID,
			DeltaParentID_11: item11.DeltaParentID,
			HeaderKey_11:     item11.HeaderKey,
			Enable_11:        item11.Enable,
			Description_11:   item11.Description,
			Value_11:         item11.Value,
			Prev_11:          item11.Prev,
			Next_11:          item11.Next,
			// 12
			ID_12:            item12.ID,
			ExampleID_12:     item12.ExampleID,
			DeltaParentID_12: item12.DeltaParentID,
			HeaderKey_12:     item12.HeaderKey,
			Enable_12:        item12.Enable,
			Description_12:   item12.Description,
			Value_12:         item12.Value,
			Prev_12:          item12.Prev,
			Next_12:          item12.Next,
			// 13
			ID_13:            item13.ID,
			ExampleID_13:     item13.ExampleID,
			DeltaParentID_13: item13.DeltaParentID,
			HeaderKey_13:     item13.HeaderKey,
			Enable_13:        item13.Enable,
			Description_13:   item13.Description,
			Value_13:         item13.Value,
			Prev_13:          item13.Prev,
			Next_13:          item13.Next,
			// 14
			ID_14:            item14.ID,
			ExampleID_14:     item14.ExampleID,
			DeltaParentID_14: item14.DeltaParentID,
			HeaderKey_14:     item14.HeaderKey,
			Enable_14:        item14.Enable,
			Description_14:   item14.Description,
			Value_14:         item14.Value,
			Prev_14:          item14.Prev,
			Next_14:          item14.Next,
			// 15
			ID_15:            item15.ID,
			ExampleID_15:     item15.ExampleID,
			DeltaParentID_15: item15.DeltaParentID,
			HeaderKey_15:     item15.HeaderKey,
			Enable_15:        item15.Enable,
			Description_15:   item15.Description,
			Value_15:         item15.Value,
			Prev_15:          item15.Prev,
			Next_15:          item15.Next,
		}
		if err := h.queries.CreateHeaderBulk(ctx, params); err != nil {
			return err
		}
	}

	return nil
}

func (h HeaderService) UpdateHeader(ctx context.Context, header mexampleheader.Header) error {
	return h.queries.UpdateHeader(ctx, gen.UpdateHeaderParams{
		ID:          header.ID,
		HeaderKey:   header.HeaderKey,
		Enable:      header.Enable,
		Description: header.Description,
		Value:       header.Value,
	})
}

func (h HeaderService) DeleteHeader(ctx context.Context, headerID idwrap.IDWrap) error {
	return h.queries.DeleteHeader(ctx, headerID)
}

func (h HeaderService) ResetHeaderDelta(ctx context.Context, id idwrap.IDWrap) error {
	header, err := h.GetHeaderByID(ctx, id)
	if err != nil {
		return err
	}

	header.DeltaParentID = nil
	header.HeaderKey = ""
	header.Enable = false
	header.Description = ""
	header.Value = ""

	return h.UpdateHeader(ctx, header)
}

// GetHeadersOrdered walks the linked list from head to tail for an example
func (h HeaderService) GetHeadersOrdered(ctx context.Context, exampleID idwrap.IDWrap) ([]mexampleheader.Header, error) {
	dbHeaders, err := h.queries.GetHeadersByExampleIDOrdered(ctx, gen.GetHeadersByExampleIDOrderedParams{
		ExampleID:   exampleID,
		ExampleID_2: exampleID,
	})
	if err != nil {
		return nil, err
	}

	var headers []mexampleheader.Header
	for _, dbHeader := range dbHeaders {
		// Convert the query row to model Header
		var deltaParentID *idwrap.IDWrap
		if dbHeader.DeltaParentID != nil {
			id := idwrap.NewFromBytesMust(dbHeader.DeltaParentID)
			deltaParentID = &id
		}

		var prev *idwrap.IDWrap
		if dbHeader.Prev != nil {
			id := idwrap.NewFromBytesMust(dbHeader.Prev)
			prev = &id
		}

		var next *idwrap.IDWrap
		if dbHeader.Next != nil {
			id := idwrap.NewFromBytesMust(dbHeader.Next)
			next = &id
		}

		header := mexampleheader.Header{
			ID:            idwrap.NewFromBytesMust(dbHeader.ID),
			ExampleID:     idwrap.NewFromBytesMust(dbHeader.ExampleID),
			DeltaParentID: deltaParentID,
			HeaderKey:     dbHeader.HeaderKey,
			Enable:        dbHeader.Enable,
			Description:   dbHeader.Description,
			Value:         dbHeader.Value,
			Prev:          prev,
			Next:          next,
		}
		headers = append(headers, header)
	}

	return headers, nil
}

// UpdateHeaderLinks updates the prev/next pointers for a header
func (h HeaderService) UpdateHeaderLinks(ctx context.Context, headerID idwrap.IDWrap, prev, next *idwrap.IDWrap) error {
	// Get the header to extract its example ID for validation
	header, err := h.GetHeaderByID(ctx, headerID)
	if err != nil {
		return err
	}

	return h.queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
		Prev:      prev,
		Next:      next,
		ID:        headerID,
		ExampleID: header.ExampleID,
	})
}

// AppendHeader adds a header to the end of the linked list for an example
func (h HeaderService) AppendHeader(ctx context.Context, header mexampleheader.Header) error {
	// Fetch ordered headers to build a safe append plan
	ordered, err := h.GetHeaderByExampleIDOrdered(ctx, header.ExampleID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	// Convert to MovableItem slice with positions 0..n-1
	items := make([]movable.MovableItem, 0, len(ordered))
	for i := range ordered {
		parent := header.ExampleID
		items = append(items, movable.MovableItem{
			ID:       ordered[i].ID,
			ParentID: &parent,
			Position: i,
			ListType: movable.RequestListTypeHeaders,
		})
	}

	plan, err := movable.PlanAppendAtEndFromItems(header.ExampleID, movable.RequestListTypeHeaders, header.ID, items)
	if err != nil {
		return err
	}

	// Create the header with planner's prev; next is always NULL for new tail
	header.Prev = plan.PrevID
	header.Next = nil
	if err := h.CreateHeader(ctx, header); err != nil {
		return err
	}

	// Update the previous tail's next to point to the new header (if any)
	if plan.PrevID != nil {
		// Optional CAS-like guard: if prev's next is already set, skip to avoid races
		prevRow, getErr := h.GetHeaderByID(ctx, *plan.PrevID)
		if getErr != nil {
			return getErr
		}
		if prevRow.Next == nil {
			if err := h.queries.UpdateHeaderNext(ctx, gen.UpdateHeaderNextParams{
				Next:      &header.ID,
				ID:        *plan.PrevID,
				ExampleID: header.ExampleID,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// AppendBulkHeader adds multiple headers to the end of the linked list while maintaining proper linking
func (h HeaderService) AppendBulkHeader(ctx context.Context, headers []mexampleheader.Header) error {
	if len(headers) == 0 {
		return nil
	}

	// Group headers by example ID to handle linked lists separately for each example
	headersByExample := make(map[idwrap.IDWrap][]mexampleheader.Header)
	for _, header := range headers {
		headersByExample[header.ExampleID] = append(headersByExample[header.ExampleID], header)
	}

	// Process headers for each example separately
	for exampleID, exampleHeaders := range headersByExample {
		// Get the current tail for this example
		tail, err := h.queries.GetHeaderTail(ctx, exampleID)
		var currentTail *idwrap.IDWrap
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			currentTail = &tail.ID
		}

		// Create headers WITHOUT prev/next first to avoid foreign key violations
		headersToCreate := make([]mexampleheader.Header, len(exampleHeaders))
		for i, header := range exampleHeaders {
			headersToCreate[i] = header
			headersToCreate[i].Prev = nil
			headersToCreate[i].Next = nil
		}

		// Create all headers without links first
		if err := h.CreateBulkHeader(ctx, headersToCreate); err != nil {
			return err
		}

		// Now update the links between headers after they all exist
		for i := range exampleHeaders {
			var prevID, nextID *idwrap.IDWrap

			if i == 0 {
				// First header links to existing tail
				prevID = currentTail
			} else {
				// Link to previous header in this batch
				prevID = &exampleHeaders[i-1].ID
			}

			if i < len(exampleHeaders)-1 {
				// Link to next header in this batch
				nextID = &exampleHeaders[i+1].ID
			}

			// Update the header's links using UpdateHeaderOrder
			if prevID != nil || nextID != nil {
				if err := h.queries.UpdateHeaderOrder(ctx, gen.UpdateHeaderOrderParams{
					Prev:      prevID,
					Next:      nextID,
					ID:        exampleHeaders[i].ID,
					ExampleID: exampleID,
				}); err != nil {
					return err
				}
			}
		}

		// Finally update the existing tail to point to the first new header (if there was a tail)
		if currentTail != nil {
			if err := h.queries.UpdateHeaderNext(ctx, gen.UpdateHeaderNextParams{
				Next:      &exampleHeaders[0].ID,
				ID:        *currentTail,
				ExampleID: exampleID,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// UnlinkHeader removes a header from the linked list by updating surrounding pointers
func (h HeaderService) UnlinkHeader(ctx context.Context, headerID idwrap.IDWrap) error {
	// First get the header to know its current prev/next
	header, err := h.GetHeaderByID(ctx, headerID)
	if err != nil {
		return err
	}

	// Update the previous header to point to our next
	if header.Prev != nil {
		if err := h.queries.UpdateHeaderNext(ctx, gen.UpdateHeaderNextParams{
			Next:      header.Next,
			ID:        *header.Prev,
			ExampleID: header.ExampleID,
		}); err != nil {
			return err
		}
	}

	// Update the next header to point to our previous
	if header.Next != nil {
		if err := h.queries.UpdateHeaderPrev(ctx, gen.UpdateHeaderPrevParams{
			Prev:      header.Prev,
			ID:        *header.Next,
			ExampleID: header.ExampleID,
		}); err != nil {
			return err
		}
	}

	// Finally delete the header itself
	return h.DeleteHeader(ctx, headerID)
}

// UpdateHeaderNext updates only the next pointer of a header
func (h HeaderService) UpdateHeaderNext(ctx context.Context, headerID idwrap.IDWrap, next *idwrap.IDWrap) error {
	// Get the header to extract its example ID for validation
	header, err := h.GetHeaderByID(ctx, headerID)
	if err != nil {
		return err
	}

	return h.queries.UpdateHeaderNext(ctx, gen.UpdateHeaderNextParams{
		Next:      next,
		ID:        headerID,
		ExampleID: header.ExampleID,
	})
}

// UpdateHeaderPrev updates only the prev pointer of a header
func (h HeaderService) UpdateHeaderPrev(ctx context.Context, headerID idwrap.IDWrap, prev *idwrap.IDWrap) error {
	// Get the header to extract its example ID for validation
	header, err := h.GetHeaderByID(ctx, headerID)
	if err != nil {
		return err
	}

	return h.queries.UpdateHeaderPrev(ctx, gen.UpdateHeaderPrevParams{
		Prev:      prev,
		ID:        headerID,
		ExampleID: header.ExampleID,
	})
}

// MoveHeader moves a header to a new position in the linked list relative to another header
// Either afterHeaderID or beforeHeaderID should be provided, not both
// If afterHeaderID is provided, the header will be moved to the position after the target
// If beforeHeaderID is provided, the header will be moved to the position before the target
func (h HeaderService) MoveHeader(ctx context.Context, headerID idwrap.IDWrap, afterHeaderID, beforeHeaderID *idwrap.IDWrap, exampleID idwrap.IDWrap) error {
	// Validate that exactly one position is specified
	if (afterHeaderID == nil && beforeHeaderID == nil) || (afterHeaderID != nil && beforeHeaderID != nil) {
		return errors.New("exactly one of afterHeaderID or beforeHeaderID must be specified")
	}

	// Get the header to move
	headerToMove, err := h.GetHeaderByID(ctx, headerID)
	if err != nil {
		return err
	}

	// Validate that header belongs to the specified example
	if headerToMove.ExampleID.Compare(exampleID) != 0 {
		return errors.New("header does not belong to the specified example")
	}

	// Determine the target header
	var targetHeaderID idwrap.IDWrap
	var moveAfter bool
	if afterHeaderID != nil {
		targetHeaderID = *afterHeaderID
		moveAfter = true
	} else {
		targetHeaderID = *beforeHeaderID
		moveAfter = false
	}

	// Get the target header
	targetHeader, err := h.GetHeaderByID(ctx, targetHeaderID)
	if err != nil {
		return err
	}

	// Validate that target header belongs to the same example
	if targetHeader.ExampleID.Compare(exampleID) != 0 {
		return errors.New("target header does not belong to the specified example")
	}

	// Check if the move would result in no change (same position)
	if moveAfter {
		// Moving after target - no change if header is already after target
		if targetHeader.Next != nil && targetHeader.Next.Compare(headerID) == 0 {
			return nil // No change needed
		}
	} else {
		// Moving before target - no change if header is already before target
		if targetHeader.Prev != nil && targetHeader.Prev.Compare(headerID) == 0 {
			return nil // No change needed
		}
	}

	// Step 1: Remember the original neighbors of the header to move
	originalPrev := headerToMove.Prev
	originalNext := headerToMove.Next

	// Step 2: Unlink the header from its current position
	if originalPrev != nil {
		if err := h.UpdateHeaderNext(ctx, *originalPrev, originalNext); err != nil {
			return err
		}
	}
	if originalNext != nil {
		if err := h.UpdateHeaderPrev(ctx, *originalNext, originalPrev); err != nil {
			return err
		}
	}

	// Step 3: Insert the header at the new position
	var newPrev, newNext *idwrap.IDWrap

	if moveAfter {
		// Moving after target: target <- header -> target.next
		newPrev = &targetHeaderID
		newNext = targetHeader.Next

		// Update target's next to point to our header
		if err := h.UpdateHeaderNext(ctx, targetHeaderID, &headerID); err != nil {
			return err
		}

		// If target had a next, update its prev to point to our header
		if targetHeader.Next != nil {
			if err := h.UpdateHeaderPrev(ctx, *targetHeader.Next, &headerID); err != nil {
				return err
			}
		}
	} else {
		// Moving before target: target.prev <- header -> target
		newPrev = targetHeader.Prev
		newNext = &targetHeaderID

		// If target had a prev, update its next to point to our header
		if targetHeader.Prev != nil {
			if err := h.UpdateHeaderNext(ctx, *targetHeader.Prev, &headerID); err != nil {
				return err
			}
		}

		// Update target's prev to point to our header
		if err := h.UpdateHeaderPrev(ctx, targetHeaderID, &headerID); err != nil {
			return err
		}
	}

	// Step 4: Update our header's prev/next pointers
	return h.UpdateHeaderLinks(ctx, headerID, newPrev, newNext)
}
