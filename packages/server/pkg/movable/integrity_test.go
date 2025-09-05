package movable

import (
    "testing"
    "the-dev-tools/server/pkg/idwrap"
)

func TestCheckListIntegrity_OKZeroBased(t *testing.T) {
    parent := idwrap.NewNow()
    a, b := idwrap.NewNow(), idwrap.NewNow()
    items := []MovableItem{
        {ID: a, ParentID: &parent, Position: 0, ListType: CollectionListTypeItems},
        {ID: b, ParentID: &parent, Position: 1, ListType: CollectionListTypeItems},
    }
    warnings, err := CheckListIntegrity(parent, items)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if len(warnings) != 0 { t.Fatalf("unexpected warnings: %v", warnings) }
}

func TestCheckListIntegrity_DuplicateID_Error(t *testing.T) {
    parent := idwrap.NewNow()
    a := idwrap.NewNow()
    items := []MovableItem{
        {ID: a, ParentID: &parent, Position: 0, ListType: CollectionListTypeItems},
        {ID: a, ParentID: &parent, Position: 1, ListType: CollectionListTypeItems},
    }
    if _, err := CheckListIntegrity(parent, items); err == nil {
        t.Fatalf("expected duplicate id error")
    }
}

func TestCheckListIntegrity_DuplicatePosition_Error(t *testing.T) {
    parent := idwrap.NewNow()
    a, b := idwrap.NewNow(), idwrap.NewNow()
    items := []MovableItem{
        {ID: a, ParentID: &parent, Position: 0, ListType: CollectionListTypeItems},
        {ID: b, ParentID: &parent, Position: 0, ListType: CollectionListTypeItems},
    }
    if _, err := CheckListIntegrity(parent, items); err == nil {
        t.Fatalf("expected duplicate position error")
    }
}

func TestCheckListIntegrity_ParentMismatch_Error(t *testing.T) {
    parent1 := idwrap.NewNow()
    parent2 := idwrap.NewNow()
    a := idwrap.NewNow()
    items := []MovableItem{
        {ID: a, ParentID: &parent2, Position: 0, ListType: CollectionListTypeItems},
    }
    if _, err := CheckListIntegrity(parent1, items); err == nil {
        t.Fatalf("expected parent mismatch error")
    }
}

func TestCheckListIntegrity_Gapped_Warning(t *testing.T) {
    parent := idwrap.NewNow()
    a, b := idwrap.NewNow(), idwrap.NewNow()
    items := []MovableItem{
        {ID: a, ParentID: &parent, Position: 2, ListType: CollectionListTypeItems},
        {ID: b, ParentID: &parent, Position: 4, ListType: CollectionListTypeItems},
    }
    warnings, err := CheckListIntegrity(parent, items)
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    if len(warnings) == 0 { t.Fatalf("expected warning for gaps/non-zero-based") }
}

