package sbodyurl

import (
    "context"
    "database/sql"
    "fmt"
    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/server/pkg/idwrap"
    "the-dev-tools/server/pkg/movable"
)

// BodyUrlEncodedMovableRepository implements movable.MovableRepository for URL-encoded bodies
type BodyUrlEncodedMovableRepository struct {
    queries *gen.Queries
}

func NewBodyUrlEncodedMovableRepository(queries *gen.Queries) *BodyUrlEncodedMovableRepository {
    return &BodyUrlEncodedMovableRepository{queries: queries}
}

// TX returns a transaction-bound repository instance
func (r *BodyUrlEncodedMovableRepository) TX(tx *sql.Tx) movable.MovableRepository {
    return &BodyUrlEncodedMovableRepository{queries: r.queries.WithTx(tx)}
}

// GetItemsByParent returns ordered items under exampleID for the given list type
func (r *BodyUrlEncodedMovableRepository) GetItemsByParent(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) ([]movable.MovableItem, error) {
    // Support normal and delta list types only
    if listType.String() != movable.RequestListTypeBodyUrlEncoded.String() && listType.String() != movable.RequestListTypeBodyUrlEncodedDeltas.String() {
        return nil, fmt.Errorf("unsupported listType for url-encoded: %v", listType.String())
    }
    // Fetch all rows (includes prev/next)
    rows, err := r.queries.GetBodyUrlEncodedsByExampleID(ctx, parentID)
    if err != nil { return nil, err }
    if len(rows) == 0 {
        return []movable.MovableItem{}, nil
    }
    // Build map id->(prev,next)
    type link struct{ prev, next *idwrap.IDWrap }
    links := make(map[idwrap.IDWrap]link, len(rows))
    ids := make([]idwrap.IDWrap, 0, len(rows))
    for _, row := range rows {
        ids = append(ids, row.ID)
        var p, n *idwrap.IDWrap
        if len(row.Prev) > 0 { v := idwrap.NewFromBytesMust(row.Prev); p = &v }
        if len(row.Next) > 0 { v := idwrap.NewFromBytesMust(row.Next); n = &v }
        links[row.ID] = link{prev: p, next: n}
    }
    // Find heads (prev == nil or prev not in set)
    idset := make(map[idwrap.IDWrap]struct{}, len(ids))
    for _, id := range ids { idset[id] = struct{}{} }
    heads := make([]idwrap.IDWrap, 0)
    for _, id := range ids {
        l := links[id]
        if l.prev == nil { heads = append(heads, id); continue }
        if _, ok := idset[*l.prev]; !ok { heads = append(heads, id) }
    }
    // Sort heads by ULID for stability
    // Note: no direct sort helper; simple selection sort by Compare
    for i := 0; i < len(heads); i++ {
        min := i
        for j := i+1; j < len(heads); j++ { if heads[j].Compare(heads[min]) < 0 { min = j } }
        heads[i], heads[min] = heads[min], heads[i]
    }
    // Walk from each head following next pointers; avoid cycles
    visited := make(map[idwrap.IDWrap]bool, len(ids))
    ordered := make([]idwrap.IDWrap, 0, len(ids))
    for _, h := range heads {
        cur := h
        for {
            if visited[cur] { break }
            visited[cur] = true
            ordered = append(ordered, cur)
            nx := links[cur].next
            if nx == nil { break }
            cur = *nx
        }
    }
    // Append any unvisited nodes (cycles/islands), sorted by ULID
    rest := make([]idwrap.IDWrap, 0)
    for _, id := range ids { if !visited[id] { rest = append(rest, id) } }
    for i := 0; i < len(rest); i++ {
        min := i
        for j := i+1; j < len(rest); j++ { if rest[j].Compare(rest[min]) < 0 { min = j } }
        rest[i], rest[min] = rest[min], rest[i]
    }
    ordered = append(ordered, rest...)
    // Build MovableItems with positions
    items := make([]movable.MovableItem, len(ordered))
    for i, id := range ordered {
        items[i] = movable.MovableItem{ ID: id, ParentID: &parentID, Position: i, ListType: listType }
    }
    return items, nil
}

// GetMaxPosition returns the max index (len-1) for the list
func (r *BodyUrlEncodedMovableRepository) GetMaxPosition(ctx context.Context, parentID idwrap.IDWrap, listType movable.ListType) (int, error) {
    items, err := r.GetItemsByParent(ctx, parentID, listType)
    if err != nil { return -1, err }
    if len(items) == 0 { return -1, nil }
    return len(items) - 1, nil
}

// UpdatePositions applies batch updates by delegating to UpdatePosition
func (r *BodyUrlEncodedMovableRepository) UpdatePositions(ctx context.Context, tx *sql.Tx, updates []movable.PositionUpdate) error {
    for _, u := range updates {
        if err := r.UpdatePosition(ctx, tx, u.ItemID, u.ListType, u.Position); err != nil {
            return err
        }
    }
    return nil
}

// UpdatePosition moves itemID to target position within example-scoped list
func (r *BodyUrlEncodedMovableRepository) UpdatePosition(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap, listType movable.ListType, position int) error {
    // Bind repo to the provided transaction, if any
    repo := r
    if tx != nil {
        repo = &BodyUrlEncodedMovableRepository{queries: r.queries.WithTx(tx)}
    }

    // 1) Load scope (example_id) and current neighbors of the moving item
    links, err := repo.queries.GetBodyUrlEncodedLinks(ctx, itemID)
    if err != nil {
        return err
    }
    exampleID := links.ExampleID

    // 2) Get current list order using in-memory traversal (cycle-safe) to avoid
    //    any recursive CTE inside an open write transaction.
    items, err := repo.GetItemsByParent(ctx, exampleID, listType)
    if err != nil {
        return err
    }
    if position < 0 {
        return fmt.Errorf("invalid position: %d", position)
    }
    if len(items) == 0 {
        // Nothing to do, but this shouldn't really happen for a known itemID
        return nil
    }
    // Find current index and build a simple ID slice
    ids := make([]idwrap.IDWrap, len(items))
    curIdx := -1
    for i, it := range items {
        ids[i] = it.ID
        if it.ID.Compare(itemID) == 0 {
            curIdx = i
        }
    }
    if curIdx == -1 {
        // Fallback to a linear scan over unordered data (very unlikely)
        rows, err := repo.queries.GetBodyUrlEncodedsByExampleID(ctx, exampleID)
        if err != nil {
            return err
        }
        ids = ids[:0]
        for i, row := range rows {
            ids = append(ids, row.ID)
            if row.ID.Compare(itemID) == 0 {
                curIdx = i
            }
        }
        if curIdx == -1 {
            return fmt.Errorf("item %s not found in example %s", itemID.String(), exampleID.String())
        }
    }

    // Clamp desired position to [0..len(ids)] inclusive, allowing append at end
    if position > len(ids) {
        position = len(ids)
    }
    if position < 0 {
        position = 0
    }
    // Note: do not early-return on curIdx == position, because callers may pass
    // len(ids) to mean append-to-end, which can equal curIdx+1 for tail moves.

    // 3) Unlink the item from its current neighbors
    var prevID, nextID *idwrap.IDWrap
    if len(links.Prev) > 0 { v := idwrap.NewFromBytesMust(links.Prev); prevID = &v }
    if len(links.Next) > 0 { v := idwrap.NewFromBytesMust(links.Next); nextID = &v }
    if prevID != nil {
        if err := repo.queries.UpdateBodyUrlEncodedNext(ctx, gen.UpdateBodyUrlEncodedNextParams{Next: bts(nextID), ID: *prevID, ExampleID: exampleID}); err != nil {
            return err
        }
    }
    if nextID != nil {
        if err := repo.queries.UpdateBodyUrlEncodedPrev(ctx, gen.UpdateBodyUrlEncodedPrevParams{Prev: bts(prevID), ID: *nextID, ExampleID: exampleID}); err != nil {
            return err
        }
    }

    // 4) Compute the new insertion index relative to the list AFTER removal
    //    Allow inserting at the end: targetIdx can equal len(filtered).

    // Build filtered list IDs without the moving item
    filtered := make([]idwrap.IDWrap, 0, len(ids)-1)
    for _, id := range ids {
        if id.Compare(itemID) != 0 {
            filtered = append(filtered, id)
        }
    }
    // Determine target index after removal
    targetIdx := position
    if curIdx < position {
        targetIdx = position - 1
    }
    if targetIdx < 0 {
        targetIdx = 0
    }
    if targetIdx > len(filtered) {
        targetIdx = len(filtered)
    }

    // 5) Determine new neighbors at target index
    var newPrev, newNext *idwrap.IDWrap
    if targetIdx == 0 {
        // New head: no prev, next is current first
        if len(filtered) > 0 {
            v := filtered[0]
            newNext = &v
        }
    } else if targetIdx == len(filtered) {
        // New tail: prev is current last
        v := filtered[len(filtered)-1]
        newPrev = &v
    } else {
        // Insert between prev and next
        pv := filtered[targetIdx-1]
        nx := filtered[targetIdx]
        newPrev = &pv
        newNext = &nx
    }

    // 6) Link the moving item first, then fix neighbors to point back to it
    if err := repo.queries.UpdateBodyUrlEncodedOrder(ctx, gen.UpdateBodyUrlEncodedOrderParams{Prev: bts(newPrev), Next: bts(newNext), ID: itemID, ExampleID: exampleID}); err != nil {
        return err
    }
    if newPrev != nil {
        if err := repo.queries.UpdateBodyUrlEncodedNext(ctx, gen.UpdateBodyUrlEncodedNextParams{Next: itemID.Bytes(), ID: *newPrev, ExampleID: exampleID}); err != nil {
            return err
        }
    }
    if newNext != nil {
        if err := repo.queries.UpdateBodyUrlEncodedPrev(ctx, gen.UpdateBodyUrlEncodedPrevParams{Prev: itemID.Bytes(), ID: *newNext, ExampleID: exampleID}); err != nil {
            return err
        }
    }
    return nil
}

// Helper to convert *IDWrap to []byte
func bts(id *idwrap.IDWrap) []byte { if id == nil { return nil }; return id.Bytes() }

// Remove unlinks a URL-encoded body field from its example-scoped chain
func (r *BodyUrlEncodedMovableRepository) Remove(ctx context.Context, tx *sql.Tx, itemID idwrap.IDWrap) error {
    repo := r
    if tx != nil {
        repo = &BodyUrlEncodedMovableRepository{queries: r.queries.WithTx(tx)}
    }
    links, err := repo.queries.GetBodyUrlEncodedLinks(ctx, itemID)
    if err != nil { return err }
    exampleID := links.ExampleID
    var prevID, nextID *idwrap.IDWrap
    if len(links.Prev) > 0 { v := idwrap.NewFromBytesMust(links.Prev); prevID = &v }
    if len(links.Next) > 0 { v := idwrap.NewFromBytesMust(links.Next); nextID = &v }
    if prevID != nil {
        if err := repo.queries.UpdateBodyUrlEncodedNext(ctx, gen.UpdateBodyUrlEncodedNextParams{Next: bts(nextID), ID: *prevID, ExampleID: exampleID}); err != nil { return err }
    }
    if nextID != nil {
        if err := repo.queries.UpdateBodyUrlEncodedPrev(ctx, gen.UpdateBodyUrlEncodedPrevParams{Prev: bts(prevID), ID: *nextID, ExampleID: exampleID}); err != nil { return err }
    }
    // Isolate the removed node
    return repo.queries.UpdateBodyUrlEncodedOrder(ctx, gen.UpdateBodyUrlEncodedOrderParams{Prev: nil, Next: nil, ID: itemID, ExampleID: exampleID})
}
