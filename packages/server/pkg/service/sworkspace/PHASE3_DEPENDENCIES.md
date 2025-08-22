# Phase 3 Implementation Dependencies

This document outlines the database schema and query dependencies that must be implemented in Phase 1-2 for the Phase 3 service layer enhancements to function properly.

## Database Schema Changes Required

The workspace table needs to be extended with prev/next columns for linked-list ordering:

```sql
-- Add ordering columns to workspaces table
ALTER TABLE workspaces ADD COLUMN prev BLOB;
ALTER TABLE workspaces ADD COLUMN next BLOB;

-- Add indexes for performance
CREATE INDEX workspace_ordering ON workspaces (user_id, prev, next);
CREATE INDEX workspace_user_lookup ON workspaces (id, user_id);
```

## Required SQLC Queries

The following queries need to be added to `query.sql` and generated:

### 1. GetWorkspacesByUserIDOrdered
```sql
-- name: GetWorkspacesByUserIDOrdered :many
-- Uses WITH RECURSIVE CTE to traverse linked list from head to tail
-- Requires index on (user_id, prev) for optimal performance
WITH RECURSIVE ordered_workspaces AS (
  -- Base case: Find the head (prev IS NULL)
  SELECT
    w.id,
    w.name,
    w.updated,
    w.collection_count,
    w.flow_count,
    w.active_env,
    w.global_env,
    w.prev,
    w.next,
    0 as position
  FROM workspaces w
  JOIN workspaces_users wu ON w.id = wu.workspace_id
  WHERE wu.user_id = ? AND w.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointer
  SELECT
    w.id,
    w.name,
    w.updated,
    w.collection_count,
    w.flow_count,
    w.active_env,
    w.global_env,
    w.prev,
    w.next,
    oe.position + 1
  FROM workspaces w
  JOIN workspaces_users wu ON w.id = wu.workspace_id
  JOIN ordered_workspaces oe ON w.prev = oe.id
  WHERE wu.user_id = ?
)
SELECT
  oe.id,
  oe.name,
  oe.updated,
  oe.collection_count,
  oe.flow_count,
  oe.active_env,
  oe.global_env,
  oe.position
FROM
  ordered_workspaces oe
ORDER BY
  oe.position;
```

### 2. UpdateWorkspaceOrder
```sql
-- name: UpdateWorkspaceOrder :exec
UPDATE workspaces
SET prev = ?, next = ?
WHERE id = ?
  AND EXISTS (
    SELECT 1 FROM workspaces_users wu 
    WHERE wu.workspace_id = workspaces.id 
    AND wu.user_id = ?
  );
```

### 3. UpdateWorkspaceNext
```sql
-- name: UpdateWorkspaceNext :exec
UPDATE workspaces
SET next = ?
WHERE id = ?
  AND EXISTS (
    SELECT 1 FROM workspaces_users wu 
    WHERE wu.workspace_id = workspaces.id 
    AND wu.user_id = ?
  );
```

### 4. UpdateWorkspacePrev
```sql
-- name: UpdateWorkspacePrev :exec
UPDATE workspaces
SET prev = ?
WHERE id = ?
  AND EXISTS (
    SELECT 1 FROM workspaces_users wu 
    WHERE wu.workspace_id = workspaces.id 
    AND wu.user_id = ?
  );
```

## Security Considerations

All workspace move operations include user access validation through the workspace_users relationship to ensure:
- Users can only move workspaces they have access to
- Cross-user workspace operations are prevented
- Proper user isolation is maintained

## Service Layer Implementation Status

✅ **Completed in Phase 3:**
- Added `movableRepository` field to `WorkspaceService`
- Updated all constructors (`New`, `TX`, `NewTX`) to initialize movable repository
- Added `Repository()` getter method for transaction support
- Implemented `GetWorkspacesByUserIDOrdered()` method
- Implemented `MoveWorkspace()` and `MoveWorkspaceTX()` methods
- Implemented position-based move methods:
  - `MoveWorkspaceAfter()` / `MoveWorkspaceAfterTX()`
  - `MoveWorkspaceBefore()` / `MoveWorkspaceBeforeTX()`
- Added bulk reorder methods (`ReorderWorkspaces()` / `ReorderWorkspacesTX()`)
- Maintained full backward compatibility with existing methods
- Added comprehensive user access validation
- Added transaction support throughout

## Testing Requirements

Once the database dependencies are resolved, the implementation should be tested for:
- Ordered workspace retrieval respects linked-list structure
- Move operations are atomic and maintain data integrity
- User isolation prevents cross-user operations
- Transaction handling works correctly
- Existing functionality remains unaffected

## Files Modified

- `/home/electwix/dev/work/dev-tools/packages/server/pkg/service/sworkspace/sworkspace.go` - Enhanced service with ordering and move functionality