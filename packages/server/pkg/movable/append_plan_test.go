package movable

import (
    "testing"
    "the-dev-tools/server/pkg/idwrap"
)

// helper to build ID quickly
func newID() idwrap.IDWrap { return idwrap.NewNow() }

func TestPlanAppend_EmptyList(t *testing.T) {
    parent := newID()
    newItem := newID()
    plan, err := PlanAppendAtEndFromItems(parent, CollectionListTypeItems, newItem, nil)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if plan.PrevID != nil { t.Fatalf("expected nil PrevID, got %v", plan.PrevID) }
    if plan.Position != 0 { t.Fatalf("expected position 0, got %d", plan.Position) }
}

func TestPlanAppend_SingleItem(t *testing.T) {
    parent := newID()
    first := newID()
    newItem := newID()
    items := []MovableItem{{ID: first, ParentID: &parent, Position: 0, ListType: CollectionListTypeItems}}
    plan, err := PlanAppendAtEndFromItems(parent, CollectionListTypeItems, newItem, items)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if plan.PrevID == nil || plan.PrevID.Compare(first) != 0 {
        t.Fatalf("PrevID mismatch: want %s", first.String())
    }
    if plan.Position != 1 { t.Fatalf("position want 1, got %d", plan.Position) }
}

func TestPlanAppend_MultiItem_ZeroBased(t *testing.T) {
    parent := newID()
    a, b, c := newID(), newID(), newID()
    newItem := newID()
    items := []MovableItem{
        {ID: a, ParentID: &parent, Position: 0, ListType: CollectionListTypeItems},
        {ID: b, ParentID: &parent, Position: 1, ListType: CollectionListTypeItems},
        {ID: c, ParentID: &parent, Position: 2, ListType: CollectionListTypeItems},
    }
    plan, err := PlanAppendAtEndFromItems(parent, CollectionListTypeItems, newItem, items)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if plan.PrevID == nil || plan.PrevID.Compare(c) != 0 {
        t.Fatalf("PrevID want tail %s", c.String())
    }
    if plan.Position != 3 { t.Fatalf("position want 3, got %d", plan.Position) }
}

func TestPlanAppend_MultiItem_GappedPositions(t *testing.T) {
    parent := newID()
    a, b := newID(), newID()
    newItem := newID()
    items := []MovableItem{
        {ID: a, ParentID: &parent, Position: 10, ListType: CollectionListTypeItems},
        {ID: b, ParentID: &parent, Position: 12, ListType: CollectionListTypeItems},
    }
    plan, err := PlanAppendAtEndFromItems(parent, CollectionListTypeItems, newItem, items)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if plan.PrevID == nil || plan.PrevID.Compare(b) != 0 {
        t.Fatalf("PrevID want max-pos tail %s", b.String())
    }
    if plan.Position != 13 { t.Fatalf("position want max+1=13, got %d", plan.Position) }
    if len(plan.Warnings) == 0 { t.Fatalf("expected warnings for non zero-based sequence") }
}

func TestPlanAppend_DuplicateNewID_Error(t *testing.T) {
    parent := newID()
    dup := newID()
    items := []MovableItem{{ID: dup, ParentID: &parent, Position: 0, ListType: CollectionListTypeItems}}
    if _, err := PlanAppendAtEndFromItems(parent, CollectionListTypeItems, dup, items); err == nil {
        t.Fatalf("expected error for duplicate newID")
    }
}

func TestPlanAppend_ZeroNewID_Error(t *testing.T) {
    parent := newID()
    zero := idwrap.IDWrap{} // invalid for creation
    if _, err := PlanAppendAtEndFromItems(parent, CollectionListTypeItems, zero, nil); err == nil {
        t.Fatalf("expected error for zero newID")
    }
}

