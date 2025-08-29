-- Cross-Collection Move Performance Validation Script
-- This script validates that the new indexes are being used effectively
-- Run with: sqlite3 database.db < validate_cross_collection_performance.sql

.headers on
.mode table

-- Enable query plan output
PRAGMA automatic_index = OFF;

SELECT '=== Cross-Collection Move Performance Analysis ===' as analysis_section;

-- Test 1: ValidateCollectionsInSameWorkspace query plan
SELECT '--- Test 1: ValidateCollectionsInSameWorkspace Query Plan ---' as test;

EXPLAIN QUERY PLAN 
SELECT 
  c1.workspace_id = c2.workspace_id AS same_workspace,
  c1.workspace_id AS source_workspace_id,
  c2.workspace_id AS target_workspace_id
FROM collections c1, collections c2 
WHERE c1.id = X'01234567890123456789012345678901' AND c2.id = X'01234567890123456789012345678902';

-- Test 2: GetCollectionWorkspaceByItemId query plan  
SELECT '--- Test 2: GetCollectionWorkspaceByItemId Query Plan ---' as test;

EXPLAIN QUERY PLAN
SELECT c.workspace_id 
FROM collections c
JOIN collection_items ci ON ci.collection_id = c.id
WHERE ci.id = X'01234567890123456789012345678903';

-- Test 3: UpdateCollectionItemCollectionId query plan
SELECT '--- Test 3: UpdateCollectionItemCollectionId Query Plan ---' as test;

EXPLAIN QUERY PLAN
UPDATE collection_items 
SET collection_id = X'01234567890123456789012345678904', parent_folder_id = NULL
WHERE id = X'01234567890123456789012345678905';

-- Test 4: UpdateItemApiCollectionId query plan
SELECT '--- Test 4: UpdateItemApiCollectionId Query Plan ---' as test;

EXPLAIN QUERY PLAN
UPDATE item_api
SET collection_id = X'01234567890123456789012345678906'
WHERE id = X'01234567890123456789012345678907';

-- Test 5: UpdateItemFolderCollectionId query plan
SELECT '--- Test 5: UpdateItemFolderCollectionId Query Plan ---' as test;

EXPLAIN QUERY PLAN  
UPDATE item_folder
SET collection_id = X'01234567890123456789012345678908'
WHERE id = X'01234567890123456789012345678909';

-- Test 6: GetCollectionItemsInOrderForCollection query plan
SELECT '--- Test 6: GetCollectionItemsInOrderForCollection Query Plan ---' as test;

EXPLAIN QUERY PLAN
SELECT id, collection_id, parent_folder_id, item_type, folder_id, endpoint_id, name, prev_id, next_id
FROM collection_items 
WHERE collection_id = X'0123456789012345678901234567890A' AND parent_folder_id IS NULL
ORDER BY CASE WHEN prev_id IS NULL THEN 0 ELSE 1 END, id;

-- Index usage validation
SELECT '--- Index Usage Validation ---' as test;

SELECT 
  name as index_name,
  type,
  tbl_name as table_name
FROM sqlite_master 
WHERE type = 'index' 
  AND name IN (
    'collections_workspace_lookup',
    'collection_items_workspace_lookup', 
    'workspaces_users_collection_access',
    'collections_workspace_id_lookup',
    'item_api_collection_update',
    'item_folder_collection_update'
  )
ORDER BY tbl_name, name;

-- Database statistics
SELECT '--- Table Statistics ---' as test;

SELECT 
  name as table_name,
  CASE 
    WHEN name = 'collections' THEN '(Core table for workspaces/collections)'
    WHEN name = 'collection_items' THEN '(Primary table for unified item ordering)'
    WHEN name = 'item_api' THEN '(Legacy API endpoints table)'
    WHEN name = 'item_folder' THEN '(Legacy folders table)'
    WHEN name = 'workspaces_users' THEN '(User access control table)'
    ELSE '(Other table)'
  END as description
FROM sqlite_master 
WHERE type = 'table' 
  AND name IN ('collections', 'collection_items', 'item_api', 'item_folder', 'workspaces_users')
ORDER BY name;

SELECT '=== Performance Validation Complete ===' as analysis_section;

-- Notes for manual testing:
-- 1. Run this script against a database with sample data
-- 2. Verify that query plans use indexes (look for "USING INDEX" in output)
-- 3. Query plans should show constant-time lookups (O(1)) for primary operations
-- 4. No table scans should appear for the main queries
-- 
-- Expected good patterns in query plans:
-- - "SEARCH TABLE collections USING INDEX collections_workspace_lookup"
-- - "SEARCH TABLE collection_items USING INDEX collection_items_workspace_lookup" 
-- - "SEARCH TABLE item_api USING COVERING INDEX item_api_collection_update"
-- - "SEARCH TABLE item_folder USING COVERING INDEX item_folder_collection_update"
--
-- Bad patterns (indicate missing/unused indexes):
-- - "SCAN TABLE [table_name]"
-- - "USING INTEGER PRIMARY KEY" (when we expect named index usage)
-- - Large cost estimates in query plans