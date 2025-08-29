-- Debug script for HeaderDeltaList foreign key error
-- Run with: sqlite3 development.db < scripts/debug_foreign_key.sql

.mode column
.headers on
.width 20 20 20 20

SELECT '=== Step 1: Check Examples ===' as debug_step;
SELECT 
    hex(id) as id, 
    name,
    hex(version_parent_id) as parent_id,
    CASE WHEN version_parent_id IS NULL THEN 'ORIGIN' ELSE 'DELTA' END as type,
    hex(item_api_id) as endpoint_id
FROM item_api_example;

SELECT '' as '';
SELECT '=== Step 2: Check Headers ===' as debug_step;
SELECT 
    hex(id) as header_id,
    hex(example_id) as example_id, 
    header_key,
    hex(prev) as prev_id,
    hex(next) as next_id,
    hex(delta_parent_id) as delta_parent
FROM example_header;

SELECT '' as '';
SELECT '=== Step 3: Check Header Linked List Integrity ===' as debug_step;
-- Check for headers with invalid prev pointers
SELECT 
    'Invalid prev pointer' as issue,
    hex(id) as header_id,
    hex(prev) as pointing_to
FROM example_header 
WHERE prev IS NOT NULL 
  AND prev NOT IN (SELECT id FROM example_header WHERE example_id = example_header.example_id);

-- Check for headers with invalid next pointers  
SELECT 
    'Invalid next pointer' as issue,
    hex(id) as header_id,
    hex(next) as pointing_to
FROM example_header 
WHERE next IS NOT NULL 
  AND next NOT IN (SELECT id FROM example_header WHERE example_id = example_header.example_id);

SELECT '' as '';
SELECT '=== Step 4: Check for Orphaned Headers ===' as debug_step;
SELECT 
    hex(id) as orphaned_header_id,
    hex(example_id) as points_to_example
FROM example_header
WHERE example_id NOT IN (SELECT id FROM item_api_example);

SELECT '' as '';
SELECT '=== Step 5: Check Delta Examples ===' as debug_step;
SELECT 
    d.name as delta_name,
    hex(d.id) as delta_id,
    o.name as origin_name,
    hex(o.id) as origin_id
FROM item_api_example d
LEFT JOIN item_api_example o ON d.version_parent_id = o.id
WHERE d.version_parent_id IS NOT NULL;

SELECT '' as '';
SELECT '=== Step 6: Headers by Example ===' as debug_step;
SELECT 
    e.name as example_name,
    COUNT(h.id) as header_count,
    GROUP_CONCAT(h.header_key) as headers
FROM item_api_example e
LEFT JOIN example_header h ON e.id = h.example_id
GROUP BY e.id, e.name;