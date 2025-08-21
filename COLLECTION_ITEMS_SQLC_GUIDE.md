# Collection Items SQLC Queries Guide

This document explains the comprehensive SQLC query definitions for the unified `collection_items` table operations.

## Table Schema Overview

```sql
CREATE TABLE collection_items (
  id BLOB NOT NULL PRIMARY KEY,                   -- ULID (16 bytes)
  collection_id BLOB NOT NULL,                    -- Foreign key to collections
  parent_folder_id BLOB,                          -- NULL for root level items
  item_type INT8 NOT NULL,                        -- 0 = folder, 1 = endpoint
  folder_id BLOB,                                  -- NULL if item_type = 1
  endpoint_id BLOB,                                -- NULL if item_type = 0
  name TEXT NOT NULL,                              -- Display name
  prev_id BLOB,                                    -- Previous item in linked list
  next_id BLOB,                                    -- Next item in linked list
  -- Constraints and foreign keys...
);
```

## Generated Go Struct

SQLC will generate the following Go struct:

```go
type CollectionItem struct {
    ID               idwrap.IDWrap
    CollectionID     idwrap.IDWrap
    ParentFolderID   *idwrap.IDWrap  // NULL for root level
    ItemType         int8            // 0 = folder, 1 = endpoint
    FolderID         *idwrap.IDWrap  // NULL if endpoint
    EndpointID       *idwrap.IDWrap  // NULL if folder
    Name             string
    PrevID           *idwrap.IDWrap  // NULL for head of list
    NextID           *idwrap.IDWrap  // NULL for tail of list
}
```

## Core Queries

### 1. GetCollectionItem
**Purpose**: Retrieve a single collection item by ID
**Usage**: Direct item lookup
**Parameters**: `id BLOB`
**Returns**: Single `CollectionItem`

### 2. GetCollectionItemsInOrder
**Purpose**: Get all items in correct linked list order for a collection/parent folder
**Usage**: Display items in UI with proper ordering
**Parameters**: 
- `collection_id BLOB` 
- `parent_folder_id BLOB` (use NULL for root level)
- `collection_id BLOB` (repeated for recursive CTE)
**Returns**: Multiple `CollectionItem` with `position` field
**Performance**: Requires index on `(collection_id, prev_id)` for optimal performance

```go
// Example usage in Go:
items, err := queries.GetCollectionItemsInOrder(ctx, db.GetCollectionItemsInOrderParams{
    CollectionID:    collectionID,
    ParentFolderID:  &parentFolderID, // or nil for root
    CollectionID_2:  collectionID,    // Required for recursive CTE
})
```

### 3. InsertCollectionItem
**Purpose**: Insert a new item into the collection with proper linked list positioning
**Usage**: Creating new folders or endpoints
**Parameters**: All CollectionItem fields
**Linked List Logic**: 
- For insertion at end: `prev_id = current_tail.id, next_id = NULL`
- For insertion at start: `prev_id = NULL, next_id = current_head.id`
- For insertion between items: `prev_id = item_a.id, next_id = item_b.id`

### 4. UpdateCollectionItemOrder
**Purpose**: Update linked list pointers for reordering operations
**Usage**: Drag and drop reordering, moving items
**Parameters**: `prev_id BLOB`, `next_id BLOB`, `id BLOB`

## Specialized Queries

### Filtering by Type
- `GetCollectionFolderItems`: Only folders (item_type = 0)
- `GetCollectionEndpointItems`: Only endpoints (item_type = 1)
- `GetCollectionItemsByType`: Filter by any item_type

### Linked List Navigation
- `GetCollectionItemPredecessor`: Find item that points to given item
- `GetCollectionItemSuccessor`: Find item that given item points to
- `GetCollectionItemTail`: Find last item in list (next_id IS NULL)

### Maintenance and Validation
- `ValidateCollectionItemOrder`: Check for broken linked list integrity
- Returns items with status: 'ORPHANED_PREV', 'ORPHANED_NEXT', 'PREV_MISMATCH', 'NEXT_MISMATCH', 'OK'

## Linked List Operations Guide

### Insert at End (Append)
```go
// 1. Get current tail
tail, err := queries.GetCollectionItemTail(ctx, db.GetCollectionItemTailParams{
    CollectionID:    collectionID,
    ParentFolderID:  parentFolderID,
})

// 2. Insert new item
err = queries.InsertCollectionItem(ctx, db.InsertCollectionItemParams{
    ID:               newItemID,
    CollectionID:     collectionID,
    ParentFolderID:   parentFolderID,
    ItemType:         itemType,
    FolderID:         folderID,    // or nil
    EndpointID:       endpointID,  // or nil
    Name:             name,
    PrevID:           &tail.ID,    // or nil if no tail
    NextID:           nil,
})

// 3. Update previous tail to point to new item
if tail != nil {
    err = queries.UpdateCollectionItemOrder(ctx, db.UpdateCollectionItemOrderParams{
        PrevID: &tail.PrevID,
        NextID: &newItemID,
        ID:     tail.ID,
    })
}
```

### Delete Item (Maintain List Integrity)
```go
// 1. Get item to delete
item, err := queries.GetCollectionItem(ctx, itemID)

// 2. Get predecessor and successor
var pred, succ *CollectionItem
if item.PrevID != nil {
    pred, _ = queries.GetCollectionItem(ctx, *item.PrevID)
}
if item.NextID != nil {
    succ, _ = queries.GetCollectionItem(ctx, *item.NextID)
}

// 3. Link predecessor to successor
if pred != nil {
    err = queries.UpdateCollectionItemOrder(ctx, db.UpdateCollectionItemOrderParams{
        PrevID: pred.PrevID,
        NextID: item.NextID,
        ID:     pred.ID,
    })
}

// 4. Link successor to predecessor
if succ != nil {
    err = queries.UpdateCollectionItemOrder(ctx, db.UpdateCollectionItemOrderParams{
        PrevID: item.PrevID,
        NextID: succ.NextID,
        ID:     succ.ID,
    })
}

// 5. Delete the item
err = queries.DeleteCollectionItem(ctx, itemID)
```

### Move Item to Different Parent
```go
// Use MoveCollectionItemToParent query
err = queries.MoveCollectionItemToParent(ctx, db.MoveCollectionItemToParentParams{
    ParentFolderID: &newParentID,  // or nil for root
    PrevID:         &newPrevID,    // position in new parent
    NextID:         &newNextID,    // position in new parent  
    ID:             itemID,
})
```

## Performance Considerations

### Required Indexes
```sql
-- Primary index (already exists)
CREATE INDEX collection_items_idx1 ON collection_items (collection_id, parent_folder_id);

-- Linked list traversal (critical for GetCollectionItemsInOrder)
CREATE INDEX collection_items_idx2 ON collection_items (collection_id, prev_id);

-- Type filtering
CREATE INDEX collection_items_idx3 ON collection_items (collection_id, parent_folder_id, item_type);

-- Navigation queries
CREATE INDEX collection_items_idx4 ON collection_items (prev_id);
CREATE INDEX collection_items_idx5 ON collection_items (next_id);
```

### Query Performance
- `GetCollectionItemsInOrder`: O(n) with proper indexing, where n = number of items
- Single item operations: O(1) with proper indexing
- Bulk operations: Consider using transactions

## Error Handling Patterns

### Linked List Integrity
Always validate linked list integrity after modifications:

```go
// Check for broken links
brokenItems, err := queries.ValidateCollectionItemOrder(ctx, collectionID)
if err != nil || len(brokenItems) > 0 {
    // Handle broken links - repair or log error
}
```

### Transaction Usage
Wrap multi-step linked list operations in transactions:

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

qtx := queries.WithTx(tx)

// Perform multiple linked list operations
// ...

return tx.Commit()
```

## Integration with Existing Code

### Replacing Separate Tables
This unified table replaces the need for:
- Separate `item_folder` and `item_api` linked list management
- Complex joins between folder and API items
- Duplicate ordering logic

### Migration Strategy
1. Create `collection_items` table alongside existing tables
2. Migrate data from `item_folder` and `item_api` tables
3. Update application code to use unified queries
4. Remove old tables and queries

### Backward Compatibility
During migration, you can maintain views that emulate the old table structure:

```sql
CREATE VIEW item_folder_compat AS
SELECT folder_id as id, collection_id, parent_folder_id as parent_id, 
       name, prev_id as prev, next_id as next
FROM collection_items 
WHERE item_type = 0;
```

## Usage Examples

### Display Collection Structure
```go
func GetCollectionStructure(ctx context.Context, q *db.Queries, collectionID idwrap.IDWrap) ([]CollectionItem, error) {
    return q.GetCollectionItemsInOrder(ctx, db.GetCollectionItemsInOrderParams{
        CollectionID:   collectionID,
        ParentFolderID: nil,        // Root level
        CollectionID_2: collectionID,
    })
}
```

### Create New Folder
```go
func CreateFolder(ctx context.Context, q *db.Queries, collectionID idwrap.IDWrap, parentID *idwrap.IDWrap, name string) error {
    newID := idwrap.NewIDWrap()
    
    // Get current tail for positioning
    tail, _ := q.GetCollectionItemTail(ctx, db.GetCollectionItemTailParams{
        CollectionID:   collectionID,
        ParentFolderID: parentID,
    })
    
    var prevID *idwrap.IDWrap
    if tail != nil {
        prevID = &tail.ID
    }
    
    return q.InsertCollectionItem(ctx, db.InsertCollectionItemParams{
        ID:             newID,
        CollectionID:   collectionID,
        ParentFolderID: parentID,
        ItemType:       0, // folder
        FolderID:       &newID, // Self-reference for folders
        EndpointID:     nil,
        Name:           name,
        PrevID:         prevID,
        NextID:         nil,
    })
}
```