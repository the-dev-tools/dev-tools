# Collection Items Movable Repository

This package implements the `movable.MovableRepository` interface for collection items, enabling drag-and-drop functionality for mixed folder/endpoint ordering within collections.

## Overview

The `CollectionItemsMovableRepository` handles the unified `collection_items` table which stores both folders (type=0) and endpoints (type=1) in a single linked list structure using `prev_id`/`next_id` pointers.

## Key Features

- **Mixed Ordering**: Supports drag-and-drop between folders and endpoints in the same parent context
- **Linked List Operations**: Maintains order using prev/next pointer relationships
- **Transaction Support**: All operations support database transactions for atomicity
- **Batch Updates**: Efficient batch position updates for multiple items
- **Data Integrity**: Comprehensive validation to maintain linked list consistency

## Usage

```go
import (
    "the-dev-tools/db/pkg/sqlc/gen"
    "the-dev-tools/server/pkg/service/scollectionitem"
    "the-dev-tools/server/pkg/movable"
)

// Create repository
queries := gen.New(db)
repo := scollectionitem.NewCollectionItemsMovableRepository(queries)

// Update single item position
err := repo.UpdatePosition(ctx, tx, itemID, movable.CollectionListTypeItems, newPosition)

// Batch update multiple items
updates := []movable.PositionUpdate{
    {ItemID: item1ID, Position: 0},
    {ItemID: item2ID, Position: 1},
}
err = repo.UpdatePositions(ctx, tx, updates)

// Get ordered items in a parent context
items, err := repo.GetItemsByParent(ctx, parentID, movable.CollectionListTypeItems)

// Helper methods for advanced operations
err = repo.InsertAtPosition(ctx, tx, itemID, collectionID, parentFolderID, position)
err = repo.RemoveFromPosition(ctx, tx, itemID)
```

## Parent Context

Items are grouped by their parent context, which is determined by:
- `collection_id`: The collection they belong to
- `parent_folder_id`: The folder they're in (nil for root-level items)

Items can only be reordered within the same parent context.

## Database Schema

The repository works with the `collection_items` table:

```sql
CREATE TABLE collection_items (
    id BLOB PRIMARY KEY,
    collection_id BLOB NOT NULL,
    parent_folder_id BLOB, -- NULL for root-level items
    item_type INTEGER NOT NULL, -- 0=folder, 1=endpoint
    folder_id BLOB,    -- Set when item_type=0
    endpoint_id BLOB,  -- Set when item_type=1
    name TEXT NOT NULL,
    prev_id BLOB,      -- Previous item in linked list
    next_id BLOB       -- Next item in linked list
);
```

## Error Handling

The repository provides detailed error messages for:
- Invalid positions (out of range)
- Items not in the same parent context
- Missing items or broken linked list integrity
- Database operation failures

## Testing

The package includes comprehensive test coverage with:
- Unit tests for helper functions
- Integration test templates (require database setup)
- Table-driven tests for various scenarios
- Benchmark test templates for performance testing

To run tests:
```bash
go test ./pkg/service/scollectionitem/... -v
```

## Implementation Notes

- Uses SQLC generated queries for type safety
- Follows existing repository patterns in the codebase
- Implements proper ULID handling with idwrap.IDWrap
- Maintains backwards compatibility with existing collection structure
- Optimized for both single and batch operations