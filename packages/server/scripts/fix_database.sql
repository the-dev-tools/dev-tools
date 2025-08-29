-- Script to fix database issues with header ordering system
-- Run with: sqlite3 development.db < scripts/fix_database.sql

-- First, check current state
SELECT '=== Current Examples ===' as info;
SELECT id, hex(id) as hex_id, name, hex(version_parent_id) as parent_hex FROM item_api_example;

SELECT '=== Current Headers ===' as info;
SELECT hex(id), hex(example_id), header_key FROM example_header;

-- Clean up orphaned headers (headers pointing to non-existent examples)
DELETE FROM example_header 
WHERE example_id NOT IN (SELECT id FROM item_api_example);

-- Fix any headers with invalid prev/next pointers
UPDATE example_header 
SET prev = NULL 
WHERE prev IS NOT NULL 
  AND prev NOT IN (SELECT id FROM example_header WHERE example_id = example_header.example_id);

UPDATE example_header 
SET next = NULL 
WHERE next IS NOT NULL 
  AND next NOT IN (SELECT id FROM example_header WHERE example_id = example_header.example_id);

-- Report what was cleaned up
SELECT '=== Cleanup Complete ===' as info;
SELECT 'Examples remaining: ' || COUNT(*) as status FROM item_api_example;
SELECT 'Headers remaining: ' || COUNT(*) as status FROM example_header;
SELECT 'Headers with valid linked list: ' || COUNT(*) as status 
FROM example_header 
WHERE (prev IS NULL OR prev IN (SELECT id FROM example_header))
  AND (next IS NULL OR next IN (SELECT id FROM example_header));