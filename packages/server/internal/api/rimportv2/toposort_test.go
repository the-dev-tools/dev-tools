package rimportv2

import (
	"testing"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
)

// testEntity is a simple entity for testing topological sort
type testEntity struct {
	ID       idwrap.IDWrap
	ParentID *idwrap.IDWrap
	Name     string
}

func getTestID(e testEntity) idwrap.IDWrap {
	return e.ID
}

func getTestParentID(e testEntity) *idwrap.IDWrap {
	return e.ParentID
}

func TestTopologicalSort_EmptyInput(t *testing.T) {
	entities := []testEntity{}

	sorted, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 0 {
		t.Errorf("expected empty result, got %d entities", len(sorted))
	}
}

func TestTopologicalSort_SingleEntity(t *testing.T) {
	id := idwrap.NewNow()
	entities := []testEntity{
		{ID: id, Name: "A"},
	}

	sorted, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(sorted))
	}
	if sorted[0].Name != "A" {
		t.Errorf("expected A, got %s", sorted[0].Name)
	}
}

func TestTopologicalSort_SimpleChain(t *testing.T) {
	// A -> B -> C (C depends on B, B depends on A)
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()
	idC := idwrap.NewNow()

	// Input order: C, A, B (deliberately out of order)
	entities := []testEntity{
		{ID: idC, ParentID: &idB, Name: "C"},
		{ID: idA, ParentID: nil, Name: "A"},
		{ID: idB, ParentID: &idA, Name: "B"},
	}

	sorted, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(sorted))
	}

	// Verify order: A must come before B, B must come before C
	posA, posB, posC := -1, -1, -1
	for i, e := range sorted {
		switch e.Name {
		case "A":
			posA = i
		case "B":
			posB = i
		case "C":
			posC = i
		}
	}

	if posA > posB {
		t.Errorf("A (pos %d) should come before B (pos %d)", posA, posB)
	}
	if posB > posC {
		t.Errorf("B (pos %d) should come before C (pos %d)", posB, posC)
	}
}

func TestTopologicalSort_DeepChain(t *testing.T) {
	// A -> B -> C -> D -> E (5 levels deep)
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()
	idC := idwrap.NewNow()
	idD := idwrap.NewNow()
	idE := idwrap.NewNow()

	// Input in reverse order
	entities := []testEntity{
		{ID: idE, ParentID: &idD, Name: "E"},
		{ID: idD, ParentID: &idC, Name: "D"},
		{ID: idC, ParentID: &idB, Name: "C"},
		{ID: idB, ParentID: &idA, Name: "B"},
		{ID: idA, ParentID: nil, Name: "A"},
	}

	sorted, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 5 {
		t.Fatalf("expected 5 entities, got %d", len(sorted))
	}

	// Verify strict ordering: A < B < C < D < E
	positions := make(map[string]int)
	for i, e := range sorted {
		positions[e.Name] = i
	}

	expected := []string{"A", "B", "C", "D", "E"}
	for i := 0; i < len(expected)-1; i++ {
		if positions[expected[i]] > positions[expected[i+1]] {
			t.Errorf("%s (pos %d) should come before %s (pos %d)",
				expected[i], positions[expected[i]],
				expected[i+1], positions[expected[i+1]])
		}
	}
}

func TestTopologicalSort_ExternalParent(t *testing.T) {
	// External parent (not in batch) - treated as root
	externalParentID := idwrap.NewNow()
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()

	entities := []testEntity{
		{ID: idB, ParentID: &idA, Name: "B"},    // B depends on A (in batch)
		{ID: idA, ParentID: &externalParentID, Name: "A"}, // A depends on external (not in batch)
	}

	sorted, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(sorted))
	}

	// A should be treated as root (external parent), B depends on A
	posA, posB := -1, -1
	for i, e := range sorted {
		switch e.Name {
		case "A":
			posA = i
		case "B":
			posB = i
		}
	}

	if posA > posB {
		t.Errorf("A (pos %d) should come before B (pos %d)", posA, posB)
	}
}

func TestTopologicalSort_MultipleRoots(t *testing.T) {
	// Multiple independent trees
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()
	idC := idwrap.NewNow()
	idD := idwrap.NewNow()

	// Tree 1: A -> B
	// Tree 2: C -> D
	entities := []testEntity{
		{ID: idB, ParentID: &idA, Name: "B"},
		{ID: idD, ParentID: &idC, Name: "D"},
		{ID: idA, ParentID: nil, Name: "A"},
		{ID: idC, ParentID: nil, Name: "C"},
	}

	sorted, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 4 {
		t.Fatalf("expected 4 entities, got %d", len(sorted))
	}

	positions := make(map[string]int)
	for i, e := range sorted {
		positions[e.Name] = i
	}

	// A must come before B, C must come before D
	if positions["A"] > positions["B"] {
		t.Errorf("A should come before B")
	}
	if positions["C"] > positions["D"] {
		t.Errorf("C should come before D")
	}
}

func TestTopologicalSort_DiamondDependency(t *testing.T) {
	// Diamond: A -> B, A -> C, B -> D, C -> D
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()
	idC := idwrap.NewNow()
	idD := idwrap.NewNow()

	// Note: In our model, an entity can only have ONE parent,
	// so we'll test: A -> B -> D, A -> C (D has one parent B)
	// This tests multiple children of same parent
	entities := []testEntity{
		{ID: idD, ParentID: &idB, Name: "D"},
		{ID: idC, ParentID: &idA, Name: "C"},
		{ID: idB, ParentID: &idA, Name: "B"},
		{ID: idA, ParentID: nil, Name: "A"},
	}

	sorted, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sorted) != 4 {
		t.Fatalf("expected 4 entities, got %d", len(sorted))
	}

	positions := make(map[string]int)
	for i, e := range sorted {
		positions[e.Name] = i
	}

	// A must come before B and C, B must come before D
	if positions["A"] > positions["B"] {
		t.Errorf("A should come before B")
	}
	if positions["A"] > positions["C"] {
		t.Errorf("A should come before C")
	}
	if positions["B"] > positions["D"] {
		t.Errorf("B should come before D")
	}
}

func TestTopologicalSort_CycleDetection(t *testing.T) {
	// Cycle: A -> B -> C -> A
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()
	idC := idwrap.NewNow()

	entities := []testEntity{
		{ID: idA, ParentID: &idC, Name: "A"},
		{ID: idB, ParentID: &idA, Name: "B"},
		{ID: idC, ParentID: &idB, Name: "C"},
	}

	_, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if err != ErrCycleDetected {
		t.Errorf("expected ErrCycleDetected, got %v", err)
	}
}

func TestTopologicalSort_SelfReference(t *testing.T) {
	// Self-reference: A -> A (degenerate cycle)
	idA := idwrap.NewNow()

	entities := []testEntity{
		{ID: idA, ParentID: &idA, Name: "A"},
	}

	_, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err == nil {
		t.Fatal("expected cycle error for self-reference, got nil")
	}
	if err != ErrCycleDetected {
		t.Errorf("expected ErrCycleDetected, got %v", err)
	}
}

func TestTopologicalSortWithFallback_NoCycle(t *testing.T) {
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()

	entities := []testEntity{
		{ID: idB, ParentID: &idA, Name: "B"},
		{ID: idA, ParentID: nil, Name: "A"},
	}

	sorted := TopologicalSortWithFallback(entities, getTestID, getTestParentID, nil)

	if len(sorted) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(sorted))
	}

	// A should come before B
	posA, posB := -1, -1
	for i, e := range sorted {
		switch e.Name {
		case "A":
			posA = i
		case "B":
			posB = i
		}
	}

	if posA > posB {
		t.Errorf("A should come before B")
	}
}

func TestTopologicalSortWithFallback_CycleFallback(t *testing.T) {
	// Cycle: A -> B -> A
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()

	entities := []testEntity{
		{ID: idA, ParentID: &idB, Name: "A"},
		{ID: idB, ParentID: &idA, Name: "B"},
	}

	cycleCalled := false
	onCycle := func(e []testEntity) {
		cycleCalled = true
	}

	sorted := TopologicalSortWithFallback(entities, getTestID, getTestParentID, onCycle)

	if !cycleCalled {
		t.Error("expected onCycle callback to be called")
	}

	// Should return original order as fallback
	if len(sorted) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(sorted))
	}
	if sorted[0].Name != "A" || sorted[1].Name != "B" {
		t.Errorf("expected original order [A, B], got [%s, %s]", sorted[0].Name, sorted[1].Name)
	}
}

func TestTopologicalSort_PreservesStableOrderForSiblings(t *testing.T) {
	// When entities have no dependencies between each other (all roots),
	// the original order should be preserved
	idA := idwrap.NewNow()
	idB := idwrap.NewNow()
	idC := idwrap.NewNow()

	entities := []testEntity{
		{ID: idA, ParentID: nil, Name: "A"},
		{ID: idB, ParentID: nil, Name: "B"},
		{ID: idC, ParentID: nil, Name: "C"},
	}

	sorted, err := TopologicalSort(entities, getTestID, getTestParentID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With all roots, the order should be preserved (stable)
	if sorted[0].Name != "A" || sorted[1].Name != "B" || sorted[2].Name != "C" {
		t.Errorf("expected order [A, B, C], got [%s, %s, %s]",
			sorted[0].Name, sorted[1].Name, sorted[2].Name)
	}
}
