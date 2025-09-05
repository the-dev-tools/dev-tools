package movable

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"the-dev-tools/server/pkg/idwrap"
)

// AppendPlan describes the outcome of a safe append planning step.
// It contains the values the service should apply within a DB transaction.
type AppendPlan struct {
	ParentID  idwrap.IDWrap
	ListType  ListType
	NewItemID idwrap.IDWrap
	// PrevID is the node that should become the new item's Prev (nil when list empty).
	PrevID *idwrap.IDWrap
	// Position is the position to assign to the new item. Uses max(position)+1 to be robust to gaps.
	Position int
	// Warnings collects non-fatal findings (e.g., gapped positions). On fatal issues, planner returns error.
	Warnings []string
}

// PlanAppendAtEndFromItems computes an AppendPlan from already-fetched items.
// It performs preflight checks and returns warnings for non-fatal anomalies.
func PlanAppendAtEndFromItems(parentID idwrap.IDWrap, listType ListType, newID idwrap.IDWrap, items []MovableItem) (AppendPlan, error) {
	if listType == nil {
		return AppendPlan{}, fmt.Errorf("listType cannot be nil")
	}
	if isZeroID(newID) {
		return AppendPlan{}, fmt.Errorf("newID must be a valid, non-zero ULID")
	}

	warnings, err := CheckListIntegrity(parentID, items)
	if err != nil {
		return AppendPlan{}, err
	}

	// Detect if newID already exists in the list (idempotency guard).
	for _, it := range items {
		if it.ID.Compare(newID) == 0 {
			return AppendPlan{}, fmt.Errorf("item %s already present in list", newID.String())
		}
	}

	plan := AppendPlan{
		ParentID:  parentID,
		ListType:  listType,
		NewItemID: newID,
		Warnings:  warnings,
	}

	if len(items) == 0 {
		plan.PrevID = nil
		plan.Position = 0
		return plan, nil
	}

	// Compute tail as the item with the maximum Position. Also compute Position = max+1.
	maxPos := items[0].Position
	tail := items[0].ID
	for i := 1; i < len(items); i++ {
		if items[i].Position > maxPos {
			maxPos = items[i].Position
			tail = items[i].ID
		}
	}
	planPrev := tail
	plan.PrevID = &planPrev
	plan.Position = maxPos + 1
	return plan, nil
}

// BuildAppendPlanFromRepo fetches the current items via MovableRepository and
// returns a plan for appending a new item at the end.
func BuildAppendPlanFromRepo(ctx context.Context, repo MovableRepository, parentID idwrap.IDWrap, listType ListType, newID idwrap.IDWrap) (AppendPlan, error) {
	if repo == nil {
		return AppendPlan{}, fmt.Errorf("repo cannot be nil")
	}
	items, err := repo.GetItemsByParent(ctx, parentID, listType)
	if err != nil {
		return AppendPlan{}, err
	}
	return PlanAppendAtEndFromItems(parentID, listType, newID, items)
}

// AppendAtEndTx orchestrates a safe append using functional closures. The caller
// should invoke this inside a DB transaction; the closures should be transaction-bound.
type FetchItemsFn func(ctx context.Context, parentID idwrap.IDWrap, listType ListType) ([]MovableItem, error)
type InsertFn func(ctx context.Context, newID idwrap.IDWrap, parentID idwrap.IDWrap, prevID *idwrap.IDWrap, position int) error
type CASTailFn func(ctx context.Context, expectedTail idwrap.IDWrap, newID idwrap.IDWrap) (bool, error)

func AppendAtEndTx(ctx context.Context, fetch FetchItemsFn, insert InsertFn, casTail CASTailFn, parentID idwrap.IDWrap, listType ListType, newID idwrap.IDWrap) error {
	if fetch == nil || insert == nil || casTail == nil {
		return errors.New("append: fetch/insert/casTail functions cannot be nil")
	}
	items, err := fetch(ctx, parentID, listType)
	if err != nil {
		return err
	}
	plan, err := PlanAppendAtEndFromItems(parentID, listType, newID, items)
	if err != nil {
		return err
	}

	if err := insert(ctx, plan.NewItemID, plan.ParentID, plan.PrevID, plan.Position); err != nil {
		return fmt.Errorf("append: insert failed: %w", err)
	}
	if plan.PrevID != nil {
		ok, err := casTail(ctx, *plan.PrevID, plan.NewItemID)
		if err != nil {
			return fmt.Errorf("append: tail CAS failed: %w", err)
		}
		if !ok {
			return fmt.Errorf("append: concurrent tail advance detected; aborting")
		}
	}
	return nil
}

// CheckListIntegrity validates list invariants prior to append and issues warnings for non-fatal anomalies.
func CheckListIntegrity(parentID idwrap.IDWrap, items []MovableItem) ([]string, error) {
	warnings := make([]string, 0)

	// Validate parent and collect ids/positions.
	seen := make(map[string]struct{})
	posCount := make(map[int]int)
	for _, it := range items {
		if it.ParentID == nil || it.ParentID.Compare(parentID) != 0 {
			return nil, fmt.Errorf("integrity: item %s has mismatched or nil parent", it.ID.String())
		}
		key := it.ID.String()
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("integrity: duplicate item id %s", key)
		}
		seen[key] = struct{}{}
		if it.Position < 0 {
			return nil, fmt.Errorf("integrity: negative position %d for %s", it.Position, key)
		}
		posCount[it.Position]++
		if posCount[it.Position] > 1 {
			return nil, fmt.Errorf("integrity: duplicate position %d detected", it.Position)
		}
	}

	if len(items) <= 1 {
		return warnings, nil
	}

	// Check ordering & gaps: allow gaps with a warning; planner uses max+1.
	// Make a copy of positions and sort to detect monotonic sequence.
	positions := make([]int, len(items))
	for i := range items {
		positions[i] = items[i].Position
	}
	// Verify strictly increasing order when sorted by index. If not, warn once.
	for i := 1; i < len(positions); i++ {
		if positions[i] < positions[i-1] {
			warnings = append(warnings, "positions not non-decreasing; proceeding with max+1 strategy")
			break
		}
	}

	// Detect non zero-based sequence for awareness.
	minPos := positions[0]
	maxPos := positions[0]
	for _, p := range positions[1:] {
		if p < minPos {
			minPos = p
		}
		if p > maxPos {
			maxPos = p
		}
	}
	if minPos != 0 || maxPos != len(items)-1 {
		warnings = append(warnings, "positions not zero-based contiguous; using max+1 for new item")
	}

	return warnings, nil
}

// isZeroID returns true when id is an all-zero ULID (invalid newID for creation semantics).
func isZeroID(id idwrap.IDWrap) bool {
	b := id.Bytes()
	return len(b) == 16 && bytes.Equal(b, make([]byte, 16))
}
