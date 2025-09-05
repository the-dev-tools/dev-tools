package core

import (
    "context"
    "errors"
    orank "the-dev-tools/server/pkg/overlay/rank"
    "the-dev-tools/server/pkg/idwrap"
    deltav1 "the-dev-tools/spec/dist/buf/go/delta/v1"
)

// Values represents the user-facing fields common across families.
type Values struct {
    Key         string
    Value       string
    Description string
    Enabled     bool
}

// Merged holds the merged overlay view for an item.
type Merged struct {
    ID     idwrap.IDWrap
    Values Values
    Origin *Values
    Source deltav1.SourceKind
}

// OrderStore provides operations on the overlay order table.
type OrderStore interface {
    Count(ctx context.Context, exampleID idwrap.IDWrap) (int64, error)
    SelectAsc(ctx context.Context, exampleID idwrap.IDWrap) ([]OrderRow, error)
    LastRank(ctx context.Context, exampleID idwrap.IDWrap) (string, bool, error)
    MaxRevision(ctx context.Context, exampleID idwrap.IDWrap) (int64, error)
    InsertIgnore(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error
    Upsert(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap, rank string, revision int64) error
    DeleteByRef(ctx context.Context, exampleID idwrap.IDWrap, refID idwrap.IDWrap) error
    Exists(ctx context.Context, exampleID idwrap.IDWrap, refKind int8, refID idwrap.IDWrap) (bool, error)
}

// StateRow mirrors state row fields with nullability represented externally.
type StateRow struct {
    Suppressed bool
    Key, Val, Desc *string
    Enabled *bool
}

// StateStore provides operations on overlay state entries.
type StateStore interface {
    Upsert(ctx context.Context, exampleID, originID idwrap.IDWrap, suppressed bool, key, val, desc *string, enabled *bool) error
    Get(ctx context.Context, exampleID, originID idwrap.IDWrap) (StateRow, bool, error)
    ClearOverrides(ctx context.Context, exampleID, originID idwrap.IDWrap) error
    Suppress(ctx context.Context, exampleID, originID idwrap.IDWrap) error
    Unsuppress(ctx context.Context, exampleID, originID idwrap.IDWrap) error
}

// DeltaStore provides operations on overlay delta rows.
type DeltaStore interface {
    Insert(ctx context.Context, exampleID, id idwrap.IDWrap, key, value, desc string, enabled bool) error
    Update(ctx context.Context, exampleID, id idwrap.IDWrap, key, value, desc string, enabled bool) error
    Get(ctx context.Context, exampleID, id idwrap.IDWrap) (key, value, desc string, enabled bool, found bool, err error)
    Exists(ctx context.Context, exampleID, id idwrap.IDWrap) (bool, error)
    Delete(ctx context.Context, exampleID, id idwrap.IDWrap) error
}

// OrderRow is a minimal view for order scanning.
type OrderRow struct {
    RefKind int8
    RefID   idwrap.IDWrap
    Rank    string
}

// Hook functions per family
type FetchOriginFn[O any] func(ctx context.Context, originEx idwrap.IDWrap) ([]O, error)
type ExtractValuesFn[O any] func(o O) Values
// OriginValuesFn returns a map of origin id -> Values for the given IDs.
type OriginValuesFn func(ctx context.Context, ids []idwrap.IDWrap) (map[idwrap.IDWrap]Values, error)
type BuildRPCFn func(m Merged) any

const (
    RefKindOrigin int8 = 1
    RefKindDelta  int8 = 2
)

// Seed creates overlay order entries from origin list if none exist yet.
func Seed[O any](ctx context.Context, order OrderStore, fetch FetchOriginFn[O], deltaEx, originEx idwrap.IDWrap) error {
    cnt, err := order.Count(ctx, deltaEx)
    if err != nil { return err }
    if cnt > 0 { return nil }
    origin, err := fetch(ctx, originEx)
    if err != nil { return err }
    if len(origin) == 0 { return nil }
    rank := ""
    for range origin {
        var next string
        var newRank string
        if rank == "" && next == "" { newRank = orank.First() } else { newRank = orank.Between(rank, next) }
        // The origin ID will be set by the caller when inserting; here we cannot know it from O.
        // Caller should seed using a family-specific routine OR pass a specialized order.InsertIgnore per origin ID.
        // For generic core, we require the caller to perform seeding via family seeding helper.
        rank = newRank
    }
    return nil
}

// List merges overlay for a family and returns built RPC items.
func List[O any](ctx context.Context,
    order OrderStore,
    state StateStore,
    delta DeltaStore,
    fetch FetchOriginFn[O],
    extract ExtractValuesFn[O],
    originVals OriginValuesFn,
    build BuildRPCFn,
    deltaEx, originEx idwrap.IDWrap,
) ([]any, error) {
    // Ensure order present is responsibility of caller.
    ord, err := order.SelectAsc(ctx, deltaEx)
    if err != nil { return nil, err }
    // Build list of origin IDs present in order and fetch their values
    var originIDs []idwrap.IDWrap
    for _, r := range ord { if r.RefKind == RefKindOrigin { originIDs = append(originIDs, r.RefID) } }
    originByID, err := originVals(ctx, originIDs)
    if err != nil { return nil, err }
    // Iterate order and build RPC items using state and delta
    var out []any
    for _, r := range ord {
        switch r.RefKind {
        case RefKindOrigin:
            st, ok, err := state.Get(ctx, deltaEx, r.RefID)
            if err != nil { return nil, err }
            if ok && st.Suppressed { continue }
            orig, ok2 := originByID[r.RefID]
            if !ok2 { continue }
            // Determine overrides
            // Determine MIXED only if any override differs from origin values
            mixed := false
            if st.Key != nil && *st.Key != orig.Key { mixed = true }
            if st.Val != nil && *st.Val != orig.Value { mixed = true }
            if st.Desc != nil && *st.Desc != orig.Description { mixed = true }
            if st.Enabled != nil && *st.Enabled != orig.Enabled { mixed = true }
            if !mixed {
                m := Merged{ ID: r.RefID, Values: orig, Origin: &orig, Source: deltav1.SourceKind_SOURCE_KIND_ORIGIN }
                out = append(out, build(m))
                continue
            }
            // Apply overrides
            merged := orig
            if st.Key != nil { merged.Key = *st.Key }
            if st.Val != nil { merged.Value = *st.Val }
            if st.Desc != nil { merged.Description = *st.Desc }
            if st.Enabled != nil { merged.Enabled = *st.Enabled }
            src := deltav1.SourceKind_SOURCE_KIND_MIXED
            m := Merged{ ID: r.RefID, Values: merged, Origin: &orig, Source: src }
            out = append(out, build(m))
        case RefKindDelta:
            k, v, d, en, found, err := delta.Get(ctx, deltaEx, r.RefID)
            if err != nil { return nil, err }
            if !found { continue }
            m := Merged{ ID: r.RefID, Values: Values{ Key: k, Value: v, Description: d, Enabled: en }, Origin: nil, Source: deltav1.SourceKind_SOURCE_KIND_DELTA }
            out = append(out, build(m))
        }
    }
    return out, nil
}

// CreateDelta inserts delta-only and appends at tail via ranks.
func CreateDelta(ctx context.Context, order OrderStore, delta DeltaStore, deltaEx idwrap.IDWrap) (idwrap.IDWrap, error) {
    id := idwrap.NewNow()
    if err := delta.Insert(ctx, deltaEx, id, "", "", "", true); err != nil { return idwrap.IDWrap{}, err }
    last, ok, err := order.LastRank(ctx, deltaEx)
    if err != nil { return idwrap.IDWrap{}, err }
    var newRank string
    if ok { newRank = orank.Between(last, "") } else { newRank = orank.First() }
    max, err := order.MaxRevision(ctx, deltaEx)
    if err != nil { return idwrap.IDWrap{}, err }
    if err := order.Upsert(ctx, deltaEx, RefKindDelta, id, newRank, max+1); err != nil { return idwrap.IDWrap{}, err }
    return id, nil
}

// Update applies overrides for origin-ref or delta-only.
func Update(ctx context.Context, state StateStore, delta DeltaStore, deltaEx idwrap.IDWrap, id idwrap.IDWrap, vals *Values) error {
    if ok, _ := delta.Exists(ctx, deltaEx, id); ok {
        // load existing
        k, v, d, e, found, err := delta.Get(ctx, deltaEx, id)
        if err != nil { return err }
        if !found { return errors.New("delta disappeared") }
        if vals != nil {
            if vals.Key != "" { k = vals.Key }
            if vals.Value != "" { v = vals.Value }
            if vals.Description != "" { d = vals.Description }
            e = vals.Enabled
        }
        return delta.Update(ctx, deltaEx, id, k, v, d, e)
    }
    var kptr, vptr, dptr *string
    var eptr *bool
    if vals != nil {
        if vals.Key != "" { k := vals.Key; kptr = &k }
        if vals.Value != "" { v := vals.Value; vptr = &v }
        if vals.Description != "" { d := vals.Description; dptr = &d }
        eptr = &vals.Enabled
    }
    return state.Upsert(ctx, deltaEx, id, false, kptr, vptr, dptr, eptr)
}

// Move re-ranks id relative to target.
func Move(ctx context.Context, order OrderStore, deltaEx, id, target idwrap.IDWrap, after bool) error {
    ord, err := order.SelectAsc(ctx, deltaEx)
    if err != nil { return err }
    idxBy := map[string]int{}
    kindBy := map[string]int8{}
    for i, r := range ord { idxBy[r.RefID.String()] = i; kindBy[r.RefID.String()] = r.RefKind }
    // target must exist in current overlay order
    tIdx, ok := idxBy[target.String()]
    if !ok { return errors.New("target not found in overlay order") }
    // compute neighbors around target
    var left, right string
    if after {
        left = ord[tIdx].Rank
        if tIdx+1 < len(ord) { right = ord[tIdx+1].Rank }
    } else {
        right = ord[tIdx].Rank
        if tIdx-1 >= 0 { left = ord[tIdx-1].Rank }
    }
    newRank := orank.Between(left, right)
    max, err := order.MaxRevision(ctx, deltaEx)
    if err != nil { return err }
    // determine ref kind for moving id if it already exists in order
    rk, ok := kindBy[id.String()]
    if !ok {
        // fallback: assume origin-ref if not found
        rk = RefKindOrigin
    }
    return order.Upsert(ctx, deltaEx, rk, id, newRank, max+1)
}

// Reset clears overrides (origin) or values (delta).
func Reset(ctx context.Context, state StateStore, delta DeltaStore, deltaEx, id idwrap.IDWrap) error {
    if ok, _ := delta.Exists(ctx, deltaEx, id); ok {
        return delta.Update(ctx, deltaEx, id, "", "", "", true)
    }
    return state.ClearOverrides(ctx, deltaEx, id)
}

// Delete deletes delta rows or suppresses origin rows and removes order.
func Delete(ctx context.Context, order OrderStore, state StateStore, delta DeltaStore, deltaEx, id idwrap.IDWrap) error {
    // If it's a delta-only row, delete it and its order ref; do not touch origin state.
    if ok, _ := delta.Exists(ctx, deltaEx, id); ok {
        if err := delta.Delete(ctx, deltaEx, id); err != nil { return err }
        if err := order.DeleteByRef(ctx, deltaEx, id); err != nil { return err }
        return nil
    }
    // Otherwise treat as origin-ref: remove from order and mark suppressed (tombstone)
    if err := order.DeleteByRef(ctx, deltaEx, id); err != nil { return err }
    return state.Suppress(ctx, deltaEx, id)
}

// Undelete clears suppression and appends at tail if needed.
func Undelete(ctx context.Context, order OrderStore, state StateStore, deltaEx, id idwrap.IDWrap) error {
    if err := state.Unsuppress(ctx, deltaEx, id); err != nil { return err }
    if ok, _ := order.Exists(ctx, deltaEx, RefKindOrigin, id); ok { return nil }
    last, ok, err := order.LastRank(ctx, deltaEx)
    if err != nil { return err }
    var newRank string
    if ok { newRank = orank.Between(last, "") } else { newRank = orank.First() }
    max, err := order.MaxRevision(ctx, deltaEx)
    if err != nil { return err }
    return order.Upsert(ctx, deltaEx, RefKindOrigin, id, newRank, max+1)
}

// ResolveExampleForBodyID uses delta/order tables to resolve an id → delta example scope.
type ResolveOrderFn func(ctx context.Context, refID idwrap.IDWrap) (idwrap.IDWrap, bool, error)
type ResolveDeltaFn func(ctx context.Context, deltaID idwrap.IDWrap) (idwrap.IDWrap, bool, error)

func ResolveExampleForBodyID(ctx context.Context, resolveDelta ResolveDeltaFn, resolveOrder ResolveOrderFn, id idwrap.IDWrap) (idwrap.IDWrap, bool, error) {
    if ex, ok, err := resolveDelta(ctx, id); err != nil { return idwrap.IDWrap{}, false, err } else if ok { return ex, true, nil }
    if ex, ok, err := resolveOrder(ctx, id); err != nil { return idwrap.IDWrap{}, false, err } else if ok { return ex, true, nil }
    return idwrap.IDWrap{}, false, nil
}
