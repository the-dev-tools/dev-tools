//nolint:revive // exported
package assertiondelta

import (
	"context"
	"errors"
	"sort"

	"the-dev-tools/server/pkg/idwrap"
	"the-dev-tools/server/pkg/model/massert"
	"the-dev-tools/server/pkg/model/mcondition"
)

// ExampleMeta describes an example's identity and whether it is a delta clone
// (HasVersionParent == true) or an origin/default example.
type ExampleMeta struct {
	ID               idwrap.IDWrap
	HasVersionParent bool
}

// Store provides the minimal persistence operations required by the resolver.
type Store interface {
	GetAssert(ctx context.Context, id idwrap.IDWrap) (massert.Assert, error)
	ListByExample(ctx context.Context, example idwrap.IDWrap) ([]massert.Assert, error)
	ListByDeltaParent(ctx context.Context, parent idwrap.IDWrap) ([]massert.Assert, error)
	UpdateAssert(ctx context.Context, assert massert.Assert) error
	CreateAssert(ctx context.Context, assert massert.Assert) error
	DeleteAssert(ctx context.Context, id idwrap.IDWrap) error
}

// EffectiveAssert represents a merged assertion paired with the source marker
// that callers (UI/runtime) should surface.
type EffectiveAssert struct {
	Assert massert.Assert
	Source massert.AssertSource
}

// EffectiveSet bundles the origin, delta, and merged views of assertions.
type EffectiveSet struct {
	Origin []massert.Assert
	Delta  []massert.Assert
	Merged []EffectiveAssert
}

// LoadInput instructs the resolver which examples should be merged.
type LoadInput struct {
	Origin ExampleMeta
	Delta  *ExampleMeta
}

// LoadEffective merges the origin + delta assertion state into a single
// effective projection. The returned ordering mirrors the linked-list ordering
// in origin, with delta overrides applied on top when they exist.
func LoadEffective(ctx context.Context, store Store, input LoadInput) (EffectiveSet, error) {
	originAsserts, err := store.ListByExample(ctx, input.Origin.ID)
	if err != nil {
		return EffectiveSet{}, err
	}

	var deltaAsserts []massert.Assert
	if input.Delta != nil {
		deltaAsserts, err = store.ListByExample(ctx, input.Delta.ID)
		if err != nil {
			return EffectiveSet{}, err
		}
	}

	merged := mergeAsserts(originAsserts, deltaAsserts)
	mergedEff := make([]EffectiveAssert, 0, len(merged))

	for _, a := range merged {
		source := classifySource(a, input)
		mergedEff = append(mergedEff, EffectiveAssert{Assert: a, Source: source})
	}

	return EffectiveSet{Origin: originAsserts, Delta: deltaAsserts, Merged: mergedEff}, nil
}

func classifySource(a massert.Assert, input LoadInput) massert.AssertSource {
	switch {
	case input.Delta != nil && a.ExampleID.Compare(input.Delta.ID) == 0:
		return a.DetermineDeltaType(true)
	default:
		return a.DetermineDeltaType(input.Origin.HasVersionParent)
	}
}

// ApplyUpdateInput captures a request to update an origin assertion and ensure
// downstream delta copies stay in sync.
type ApplyUpdateInput struct {
	Origin    ExampleMeta
	Delta     []ExampleMeta
	AssertID  idwrap.IDWrap
	Condition mcondition.Condition
	Enable    bool
}

// ApplyUpdateResult reports what the resolver mutated.
type ApplyUpdateResult struct {
	Origin        massert.Assert
	UpdatedDeltas []massert.Assert
	CreatedDeltas []massert.Assert
}

var ErrOriginMismatch = errors.New("assertion does not belong to expected origin example")

// ApplyUpdate mutates the origin assertion and refreshes any delta assertions
// that reference it. When a delta example is supplied but lacks a delta row,
// the resolver will synthesize one so runtime + UI converge.
func ApplyUpdate(ctx context.Context, store Store, input ApplyUpdateInput) (ApplyUpdateResult, error) {
	originAssert, err := store.GetAssert(ctx, input.AssertID)
	if err != nil {
		return ApplyUpdateResult{}, err
	}
	if originAssert.ExampleID.Compare(input.Origin.ID) != 0 {
		return ApplyUpdateResult{}, ErrOriginMismatch
	}

	originAssert.Condition = input.Condition
	originAssert.Enable = input.Enable
	if err := store.UpdateAssert(ctx, originAssert); err != nil {
		return ApplyUpdateResult{}, err
	}

	updated := make([]massert.Assert, 0)
	seen := make(map[string]struct{})

	applyDeltaUpdate := func(delta massert.Assert) error {
		delta.Condition = input.Condition
		delta.Enable = input.Enable
		if err := store.UpdateAssert(ctx, delta); err != nil {
			return err
		}
		updated = append(updated, delta)
		seen[delta.ID.String()] = struct{}{}
		return nil
	}

	created := make([]massert.Assert, 0)

	for _, deltaMeta := range input.Delta {
		deltaAsserts, err := store.ListByExample(ctx, deltaMeta.ID)
		if err != nil {
			return ApplyUpdateResult{}, err
		}
		if !deltaMeta.HasVersionParent {
			for i := range deltaAsserts {
				if err := applyDeltaUpdate(deltaAsserts[i]); err != nil {
					return ApplyUpdateResult{}, err
				}
			}
			continue
		}
		matching := filterByParent(deltaAsserts, originAssert.ID)
		for i := range matching {
			if err := applyDeltaUpdate(matching[i]); err != nil {
				return ApplyUpdateResult{}, err
			}
		}
		if len(matching) == 0 {
			newDelta := cloneForDelta(originAssert, deltaMeta.ID)
			newDelta.Condition = input.Condition
			newDelta.Enable = input.Enable
			if err := store.CreateAssert(ctx, newDelta); err != nil {
				return ApplyUpdateResult{}, err
			}
			updated = append(updated, newDelta)
			created = append(created, newDelta)
			seen[newDelta.ID.String()] = struct{}{}
		}
	}

	// Update any other delta asserts wired to this origin (e.g. legacy default copies)
	siblings, err := store.ListByDeltaParent(ctx, originAssert.ID)
	if err != nil {
		return ApplyUpdateResult{}, err
	}
	for i := range siblings {
		if _, ok := seen[siblings[i].ID.String()]; ok {
			continue
		}
		siblings[i].Condition = input.Condition
		siblings[i].Enable = input.Enable
		if err := store.UpdateAssert(ctx, siblings[i]); err != nil {
			return ApplyUpdateResult{}, err
		}
		updated = append(updated, siblings[i])
	}

	return ApplyUpdateResult{Origin: originAssert, UpdatedDeltas: updated, CreatedDeltas: created}, nil
}

// ApplyDeleteInput removes an origin assertion and any delta references.
type ApplyDeleteInput struct {
    Origin   ExampleMeta
    Delta    []ExampleMeta
    AssertID idwrap.IDWrap
}

// ApplyDeleteResult lists the assertion IDs that were removed.
type ApplyDeleteResult struct {
	DeletedIDs []idwrap.IDWrap
}

// ApplyDelete deletes an origin assertion and cascades the removal to all delta
// rows that reference it.
func ApplyDelete(ctx context.Context, store Store, input ApplyDeleteInput) (ApplyDeleteResult, error) {
    var deleted []idwrap.IDWrap

    origin, err := store.GetAssert(ctx, input.AssertID)
    if err != nil {
        return ApplyDeleteResult{}, err
    }

    deltas, err := store.ListByDeltaParent(ctx, input.AssertID)
    if err != nil {
        return ApplyDeleteResult{}, err
    }
    for _, delta := range deltas {
        if err := store.DeleteAssert(ctx, delta.ID); err != nil {
            return ApplyDeleteResult{}, err
        }
        deleted = append(deleted, delta.ID)
    }

    for _, meta := range input.Delta {
        asserts, err := store.ListByExample(ctx, meta.ID)
        if err != nil {
            return ApplyDeleteResult{}, err
        }
        for _, a := range asserts {
            if a.DeltaParentID != nil && a.DeltaParentID.Compare(input.AssertID) == 0 {
                if err := store.DeleteAssert(ctx, a.ID); err != nil {
                    return ApplyDeleteResult{}, err
                }
                deleted = append(deleted, a.ID)
                continue
            }
            if !meta.HasVersionParent && sameAssert(origin, a) {
                if err := store.DeleteAssert(ctx, a.ID); err != nil {
                    return ApplyDeleteResult{}, err
                }
                deleted = append(deleted, a.ID)
            }
        }
    }

    if err := store.DeleteAssert(ctx, input.AssertID); err != nil {
        return ApplyDeleteResult{}, err
    }
    deleted = append(deleted, input.AssertID)

    return ApplyDeleteResult{DeletedIDs: deleted}, nil
}

func sameAssert(origin, candidate massert.Assert) bool {
    return origin.Condition.Comparisons.Expression == candidate.Condition.Comparisons.Expression &&
        origin.Enable == candidate.Enable &&
        compareIDs(origin.Prev, candidate.Prev) &&
        compareIDs(origin.Next, candidate.Next)
}

func compareIDs(a, b *idwrap.IDWrap) bool {
    switch {
    case a == nil && b == nil:
        return true
    case a == nil || b == nil:
        return false
    default:
        return a.Compare(*b) == 0
    }
}

func filterByParent(asserts []massert.Assert, parent idwrap.IDWrap) []massert.Assert {
	out := make([]massert.Assert, 0)
	for _, a := range asserts {
		if a.DeltaParentID != nil && a.DeltaParentID.Compare(parent) == 0 {
			out = append(out, a)
		}
	}
	return out
}

func cloneForDelta(origin massert.Assert, deltaExampleID idwrap.IDWrap) massert.Assert {
	newID := idwrap.NewNow()
	parent := origin.ID
	return massert.Assert{
		ID:            newID,
		ExampleID:     deltaExampleID,
		DeltaParentID: &parent,
		Condition:     origin.Condition,
		Enable:        origin.Enable,
		Prev:          origin.Prev,
		Next:          origin.Next,
	}
}

func mergeAsserts(baseAsserts, deltaAsserts []massert.Assert) []massert.Assert {
	if len(deltaAsserts) == 0 {
		// Preserve base ordering
		return append([]massert.Assert(nil), orderAsserts(baseAsserts)...)
	}

	orderedBase := orderAsserts(baseAsserts)
	orderedDelta := orderAsserts(deltaAsserts)

	baseMap := make(map[idwrap.IDWrap]massert.Assert, len(orderedBase))
	baseOrder := make([]idwrap.IDWrap, 0, len(orderedBase))

	for _, a := range orderedBase {
		baseMap[a.ID] = a
		baseOrder = append(baseOrder, a.ID)
	}

	additions := make([]massert.Assert, 0)
	for _, delta := range orderedDelta {
		if delta.DeltaParentID != nil {
			if _, ok := baseMap[*delta.DeltaParentID]; ok {
				baseMap[*delta.DeltaParentID] = delta
				continue
			}
		}
		additions = append(additions, delta)
	}

	merged := make([]massert.Assert, 0, len(baseMap)+len(additions))
	for _, id := range baseOrder {
		if assert, ok := baseMap[id]; ok {
			merged = append(merged, assert)
		}
	}
	if len(additions) > 0 {
		merged = append(merged, orderAsserts(additions)...)
	}
	return merged
}

func orderAsserts(asserts []massert.Assert) []massert.Assert {
	if len(asserts) <= 1 {
		return append([]massert.Assert(nil), asserts...)
	}

	byID := make(map[idwrap.IDWrap]*massert.Assert, len(asserts))
	var head *massert.Assert
	for i := range asserts {
		a := asserts[i]
		byID[a.ID] = &asserts[i]
		if a.Prev == nil {
			copy := a
			head = &copy
		}
	}
	// fallback if no explicit head
	if head == nil {
		sorted := append([]massert.Assert(nil), asserts...)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].ID.String() < sorted[j].ID.String()
		})
		return sorted
	}

	ordered := make([]massert.Assert, 0, len(asserts))
	visited := make(map[idwrap.IDWrap]bool, len(asserts))

	for current := head; current != nil; {
		if visited[current.ID] {
			break
		}
		ordered = append(ordered, *current)
		visited[current.ID] = true
		if current.Next == nil {
			break
		}
		next, ok := byID[*current.Next]
		if !ok {
			break
		}
		copy := *next
		current = &copy
	}

	if len(ordered) == len(asserts) {
		return ordered
	}

	remaining := make([]massert.Assert, 0, len(asserts)-len(ordered))
	for _, a := range asserts {
		if !visited[a.ID] {
			remaining = append(remaining, a)
		}
	}
	sort.Slice(remaining, func(i, j int) bool {
		return remaining[i].ID.String() < remaining[j].ID.String()
	})
	return append(ordered, remaining...)
}
