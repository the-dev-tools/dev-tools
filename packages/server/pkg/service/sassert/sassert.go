package sassert

import (
	"context"
	"database/sql"
	"errors"
	"the-dev-tools/db/pkg/sqlc/gen"
	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
	"the-dev-tools/server/pkg/movable"
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

func (as AssertService) GetAssertsByExampleIDs(ctx context.Context, exampleIDs []idwrap.IDWrap) (map[idwrap.IDWrap][]massert.Assert, error) {
	result := make(map[idwrap.IDWrap][]massert.Assert, len(exampleIDs))
	if len(exampleIDs) == 0 {
		return result, nil
	}

	rows, err := as.queries.GetAssertsByExampleIDs(ctx, exampleIDs)
	if err != nil {
		if err == sql.ErrNoRows {
			return result, nil
		}
		return nil, err
	}

	for _, row := range rows {
		model := ConvertAssertDBToModel(row)
		result[model.ExampleID] = append(result[model.ExampleID], model)
	}

	return result, nil
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

// GetAssertsOrdered walks the linked list from head to tail for an example
func (as AssertService) GetAssertsOrdered(ctx context.Context, exampleID idwrap.IDWrap) ([]massert.Assert, error) {
	dbAsserts, err := as.queries.GetAssertsByExampleIDOrdered(ctx, gen.GetAssertsByExampleIDOrderedParams{
		ExampleID:   exampleID,
		ExampleID_2: exampleID,
	})
	if err != nil {
		return nil, err
	}

	var asserts []massert.Assert
	for _, dbAssert := range dbAsserts {
		// Convert the query row to model Assert
		var deltaParentID *idwrap.IDWrap
		if dbAssert.DeltaParentID != nil {
			id := idwrap.NewFromBytesMust(dbAssert.DeltaParentID)
			deltaParentID = &id
		}

		var prev *idwrap.IDWrap
		if dbAssert.Prev != nil {
			id := idwrap.NewFromBytesMust(dbAssert.Prev)
			prev = &id
		}

		var next *idwrap.IDWrap
		if dbAssert.Next != nil {
			id := idwrap.NewFromBytesMust(dbAssert.Next)
			next = &id
		}

		assert := massert.Assert{
			ID:            idwrap.NewFromBytesMust(dbAssert.ID),
			ExampleID:     idwrap.NewFromBytesMust(dbAssert.ExampleID),
			DeltaParentID: deltaParentID,
			Condition: mcondition.Condition{
				Comparisons: mcondition.Comparison{
					Expression: dbAssert.Expression,
				},
			},
			Enable: dbAssert.Enable,
			Prev:   prev,
			Next:   next,
		}
		asserts = append(asserts, assert)
	}

	return asserts, nil
}

// UpdateAssertLinks updates the prev/next pointers for an assertion
func (as AssertService) UpdateAssertLinks(ctx context.Context, assertID idwrap.IDWrap, prev, next *idwrap.IDWrap) error {
	// Get the assertion to extract its example ID for validation
	assert, err := as.GetAssert(ctx, assertID)
	if err != nil {
		return err
	}

	return as.queries.UpdateAssertOrder(ctx, gen.UpdateAssertOrderParams{
		Prev:      prev,
		Next:      next,
		ID:        assertID,
		ExampleID: assert.ExampleID,
	})
}

// AppendAssert adds an assertion to the end of the linked list for an example
func (as AssertService) AppendAssert(ctx context.Context, assert massert.Assert) error {
	// Build a safe append plan using current ordered assertions
	ordered, err := as.GetAssertsOrdered(ctx, assert.ExampleID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	items := make([]movable.MovableItem, 0, len(ordered))
	for i := range ordered {
		parent := assert.ExampleID
		items = append(items, movable.MovableItem{
			ID:       ordered[i].ID,
			ParentID: &parent,
			Position: i,
			ListType: movable.RequestListTypeHeaders, // reuse request list space; or add RequestListTypeAssertions if desired
		})
	}

	plan, err := movable.PlanAppendAtEndFromItems(assert.ExampleID, movable.RequestListTypeHeaders, assert.ID, items)
	if err != nil {
		return err
	}

	// Create with planner's chosen prev; next always NULL for new tail
	assert.Prev = plan.PrevID
	assert.Next = nil
	if err := as.CreateAssert(ctx, assert); err != nil {
		return err
	}

	// Patch previous tail's next if any (simple CAS guard: only if current next is NULL)
	if plan.PrevID != nil {
		prevRow, getErr := as.GetAssert(ctx, *plan.PrevID)
		if getErr != nil {
			return getErr
		}
		if prevRow.Next == nil {
			if err := as.queries.UpdateAssertNext(ctx, gen.UpdateAssertNextParams{
				Next:      &assert.ID,
				ID:        *plan.PrevID,
				ExampleID: assert.ExampleID,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// AppendBulkAssert adds multiple assertions to the end of the linked list while maintaining proper linking
func (as AssertService) AppendBulkAssert(ctx context.Context, asserts []massert.Assert) error {
	if len(asserts) == 0 {
		return nil
	}

	// Group assertions by example ID to handle linked lists separately for each example
	assertsByExample := make(map[idwrap.IDWrap][]massert.Assert)
	for _, assert := range asserts {
		assertsByExample[assert.ExampleID] = append(assertsByExample[assert.ExampleID], assert)
	}

	// Process assertions for each example separately
	for exampleID, exampleAsserts := range assertsByExample {
		// Get the current tail for this example
		tail, err := as.queries.GetAssertTail(ctx, exampleID)
		var currentTail *idwrap.IDWrap
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			currentTail = &tail.ID
		}

		// Create assertions WITHOUT prev/next first to avoid foreign key violations
		assertsToCreate := make([]massert.Assert, len(exampleAsserts))
		for i, assert := range exampleAsserts {
			assertsToCreate[i] = assert
			assertsToCreate[i].Prev = nil
			assertsToCreate[i].Next = nil
		}

		// Create all assertions without links first
		if err := as.CreateBulkAssert(ctx, assertsToCreate); err != nil {
			return err
		}

		// Now update the links between assertions after they all exist
		for i := range exampleAsserts {
			var prevID, nextID *idwrap.IDWrap

			if i == 0 {
				// First assertion links to existing tail
				prevID = currentTail
			} else {
				// Link to previous assertion in this batch
				prevID = &exampleAsserts[i-1].ID
			}

			if i < len(exampleAsserts)-1 {
				// Link to next assertion in this batch
				nextID = &exampleAsserts[i+1].ID
			}

			// Update the assertion's links using UpdateAssertOrder
			if prevID != nil || nextID != nil {
				if err := as.queries.UpdateAssertOrder(ctx, gen.UpdateAssertOrderParams{
					Prev:      prevID,
					Next:      nextID,
					ID:        exampleAsserts[i].ID,
					ExampleID: exampleID,
				}); err != nil {
					return err
				}
			}
		}

		// Finally update the existing tail to point to the first new assertion (if there was a tail)
		if currentTail != nil {
			if err := as.queries.UpdateAssertNext(ctx, gen.UpdateAssertNextParams{
				Next:      &exampleAsserts[0].ID,
				ID:        *currentTail,
				ExampleID: exampleID,
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// UnlinkAssert removes an assertion from the linked list by updating surrounding pointers
func (as AssertService) UnlinkAssert(ctx context.Context, assertID idwrap.IDWrap) error {
	// First get the assertion to know its current prev/next
	assert, err := as.GetAssert(ctx, assertID)
	if err != nil {
		return err
	}

	// Update the previous assertion to point to our next
	if assert.Prev != nil {
		if err := as.queries.UpdateAssertNext(ctx, gen.UpdateAssertNextParams{
			Next:      assert.Next,
			ID:        *assert.Prev,
			ExampleID: assert.ExampleID,
		}); err != nil {
			return err
		}
	}

	// Update the next assertion to point to our previous
	if assert.Next != nil {
		if err := as.queries.UpdateAssertPrev(ctx, gen.UpdateAssertPrevParams{
			Prev:      assert.Prev,
			ID:        *assert.Next,
			ExampleID: assert.ExampleID,
		}); err != nil {
			return err
		}
	}

	// Finally delete the assertion itself
	return as.DeleteAssert(ctx, assertID)
}

// UpdateAssertNext updates only the next pointer of an assertion
func (as AssertService) UpdateAssertNext(ctx context.Context, assertID idwrap.IDWrap, next *idwrap.IDWrap) error {
	// Get the assertion to extract its example ID for validation
	assert, err := as.GetAssert(ctx, assertID)
	if err != nil {
		return err
	}

	return as.queries.UpdateAssertNext(ctx, gen.UpdateAssertNextParams{
		Next:      next,
		ID:        assertID,
		ExampleID: assert.ExampleID,
	})
}

// UpdateAssertPrev updates only the prev pointer of an assertion
func (as AssertService) UpdateAssertPrev(ctx context.Context, assertID idwrap.IDWrap, prev *idwrap.IDWrap) error {
	// Get the assertion to extract its example ID for validation
	assert, err := as.GetAssert(ctx, assertID)
	if err != nil {
		return err
	}

	return as.queries.UpdateAssertPrev(ctx, gen.UpdateAssertPrevParams{
		Prev:      prev,
		ID:        assertID,
		ExampleID: assert.ExampleID,
	})
}

// MoveAssert moves an assertion to a new position in the linked list relative to another assertion
// Either afterAssertID or beforeAssertID should be provided, not both
// If afterAssertID is provided, the assertion will be moved to the position after the target
// If beforeAssertID is provided, the assertion will be moved to the position before the target
func (as AssertService) MoveAssert(ctx context.Context, assertID idwrap.IDWrap, afterAssertID, beforeAssertID *idwrap.IDWrap, exampleID idwrap.IDWrap) error {
	// Validate that exactly one position is specified
	if (afterAssertID == nil && beforeAssertID == nil) || (afterAssertID != nil && beforeAssertID != nil) {
		return errors.New("exactly one of afterAssertID or beforeAssertID must be specified")
	}

	// Get the assertion to move
	assertToMove, err := as.GetAssert(ctx, assertID)
	if err != nil {
		return err
	}

	// Validate that assertion belongs to the specified example
	if assertToMove.ExampleID.Compare(exampleID) != 0 {
		return errors.New("assertion does not belong to the specified example")
	}

	// Determine the target assertion
	var targetAssertID idwrap.IDWrap
	var moveAfter bool
	if afterAssertID != nil {
		targetAssertID = *afterAssertID
		moveAfter = true
	} else {
		targetAssertID = *beforeAssertID
		moveAfter = false
	}

	// Get the target assertion
	targetAssert, err := as.GetAssert(ctx, targetAssertID)
	if err != nil {
		return err
	}

	// Validate that target assertion belongs to the same example
	if targetAssert.ExampleID.Compare(exampleID) != 0 {
		return errors.New("target assertion does not belong to the specified example")
	}

	// Check if the move would result in no change (same position)
	if moveAfter {
		// Moving after target - no change if assertion is already after target
		if targetAssert.Next != nil && targetAssert.Next.Compare(assertID) == 0 {
			return nil // No change needed
		}
	} else {
		// Moving before target - no change if assertion is already before target
		if targetAssert.Prev != nil && targetAssert.Prev.Compare(assertID) == 0 {
			return nil // No change needed
		}
	}

	// Step 1: Remember the original neighbors of the assertion to move
	originalPrev := assertToMove.Prev
	originalNext := assertToMove.Next

	// Step 2: Unlink the assertion from its current position
	if originalPrev != nil {
		if err := as.UpdateAssertNext(ctx, *originalPrev, originalNext); err != nil {
			return err
		}
	}
	if originalNext != nil {
		if err := as.UpdateAssertPrev(ctx, *originalNext, originalPrev); err != nil {
			return err
		}
	}

	// Step 3: Insert the assertion at the new position
	var newPrev, newNext *idwrap.IDWrap

	if moveAfter {
		// Moving after target: target <- assertion -> target.next
		newPrev = &targetAssertID
		newNext = targetAssert.Next

		// Update target's next to point to our assertion
		if err := as.UpdateAssertNext(ctx, targetAssertID, &assertID); err != nil {
			return err
		}

		// If target had a next, update its prev to point to our assertion
		if targetAssert.Next != nil {
			if err := as.UpdateAssertPrev(ctx, *targetAssert.Next, &assertID); err != nil {
				return err
			}
		}
	} else {
		// Moving before target: target.prev <- assertion -> target
		newPrev = targetAssert.Prev
		newNext = &targetAssertID

		// If target had a prev, update its next to point to our assertion
		if targetAssert.Prev != nil {
			if err := as.UpdateAssertNext(ctx, *targetAssert.Prev, &assertID); err != nil {
				return err
			}
		}

		// Update target's prev to point to our assertion
		if err := as.UpdateAssertPrev(ctx, targetAssertID, &assertID); err != nil {
			return err
		}
	}

	// Step 4: Update our assertion's prev/next pointers
	return as.UpdateAssertLinks(ctx, assertID, newPrev, newNext)
}
