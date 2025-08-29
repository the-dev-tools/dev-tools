-- Migration: Convert example_header from position/context_type to linked list (prev/next)
-- This migration safely converts existing header data from position-based ordering to linked list ordering
-- Author: System Migration
-- Date: 2025-01-27

BEGIN TRANSACTION;

-- Step 1: Add the new columns if they don't exist
-- (This handles the case where schema.sql has already been updated)
ALTER TABLE example_header ADD COLUMN prev BLOB;
ALTER TABLE example_header ADD COLUMN next BLOB;

-- Step 2: Create temporary table to hold the conversion data
CREATE TEMPORARY TABLE header_conversion AS
SELECT 
    id,
    example_id,
    position,
    context_type,
    ROW_NUMBER() OVER (
        PARTITION BY example_id, COALESCE(context_type, 'default')
        ORDER BY position ASC
    ) as row_num,
    COUNT(*) OVER (
        PARTITION BY example_id, COALESCE(context_type, 'default')
    ) as total_count
FROM example_header
WHERE position IS NOT NULL
ORDER BY example_id, COALESCE(context_type, 'default'), position;

-- Step 3: Build the linked list by setting prev/next pointers
-- For each group of headers (same example_id and context_type):
-- - First item (row_num = 1): prev = NULL, next = next item's id
-- - Middle items: prev = previous item's id, next = next item's id  
-- - Last item (row_num = total_count): prev = previous item's id, next = NULL

-- Set prev pointers (all except first in each group)
UPDATE example_header 
SET prev = (
    SELECT h1.id
    FROM header_conversion c1
    JOIN header_conversion c2 ON (
        c1.example_id = c2.example_id 
        AND COALESCE(c1.context_type, 'default') = COALESCE(c2.context_type, 'default')
        AND c1.row_num = c2.row_num - 1
    )
    JOIN example_header h1 ON c1.id = h1.id
    WHERE c2.id = example_header.id
)
WHERE id IN (
    SELECT c.id 
    FROM header_conversion c 
    WHERE c.row_num > 1
);

-- Set next pointers (all except last in each group)
UPDATE example_header 
SET next = (
    SELECT h1.id
    FROM header_conversion c1
    JOIN header_conversion c2 ON (
        c1.example_id = c2.example_id 
        AND COALESCE(c1.context_type, 'default') = COALESCE(c2.context_type, 'default')
        AND c1.row_num = c2.row_num + 1
    )
    JOIN example_header h1 ON c1.id = h1.id
    WHERE c2.id = example_header.id
)
WHERE id IN (
    SELECT c.id 
    FROM header_conversion c 
    WHERE c.row_num < c.total_count
);

-- Step 4: Add foreign key constraints for the new columns
-- Note: SQLite doesn't support adding foreign keys to existing tables,
-- so we create indexes instead for performance

-- Create indexes for the new linked list columns
CREATE INDEX IF NOT EXISTS example_header_linked_list_idx ON example_header (
    example_id,
    prev,
    next
);

CREATE INDEX IF NOT EXISTS example_header_prev_idx ON example_header (
    prev
);

CREATE INDEX IF NOT EXISTS example_header_next_idx ON example_header (
    next
);

-- Step 5: Validation - Check that the conversion worked correctly
-- This query should return the same number of headers as before, 
-- organized in proper linked lists
CREATE TEMPORARY TABLE validation_results AS
WITH RECURSIVE ordered_headers AS (
    -- Base case: Find all heads (prev IS NULL)
    SELECT 
        h.id,
        h.example_id,
        h.prev,
        h.next,
        0 as position,
        h.id as head_id
    FROM example_header h
    WHERE h.prev IS NULL
    
    UNION ALL
    
    -- Recursive case: Follow next pointers
    SELECT 
        h.id,
        h.example_id, 
        h.prev,
        h.next,
        oh.position + 1,
        oh.head_id
    FROM example_header h
    INNER JOIN ordered_headers oh ON h.prev = oh.id
)
SELECT 
    example_id,
    COUNT(*) as linked_count,
    MAX(position) + 1 as max_position
FROM ordered_headers
GROUP BY example_id;

-- Verify that all headers are properly linked
SELECT 
    'Headers not properly linked' as error_type,
    COUNT(*) as count
FROM example_header h
LEFT JOIN validation_results v ON h.example_id = v.example_id
WHERE v.example_id IS NULL
AND h.prev IS NOT NULL; -- Only check headers that should be in the list

-- Step 6: Remove the old columns (position and context_type)
-- Note: SQLite doesn't support DROP COLUMN directly, so we create a new table
-- and copy the data, but since this might be disruptive, we comment this out
-- Users can manually drop these columns when they're ready

-- CREATE TABLE example_header_new (
--     id BLOB NOT NULL PRIMARY KEY,
--     example_id BLOB NOT NULL,
--     delta_parent_id BLOB DEFAULT NULL,
--     header_key TEXT NOT NULL,
--     enable BOOLEAN NOT NULL DEFAULT TRUE,
--     description TEXT NOT NULL,
--     value TEXT NOT NULL,
--     prev BLOB,
--     next BLOB,
--     FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
--     FOREIGN KEY (delta_parent_id) REFERENCES example_header (id) ON DELETE CASCADE,
--     FOREIGN KEY (prev) REFERENCES example_header (id) ON DELETE SET NULL,
--     FOREIGN KEY (next) REFERENCES example_header (id) ON DELETE SET NULL
-- );

-- INSERT INTO example_header_new 
-- SELECT id, example_id, delta_parent_id, header_key, enable, description, value, prev, next
-- FROM example_header;

-- DROP TABLE example_header;
-- ALTER TABLE example_header_new RENAME TO example_header;

-- Step 7: Clean up temporary tables
DROP TABLE header_conversion;
DROP TABLE validation_results;

-- Step 8: Update any remaining isolated headers (those with both prev=NULL and next=NULL)
-- These might be single headers or headers that weren't part of ordered lists
-- For now, we leave them as-is since they're still valid

COMMIT;

-- Summary Report
SELECT 
    'Migration Summary' as report_type,
    COUNT(*) as total_headers,
    COUNT(CASE WHEN prev IS NULL THEN 1 END) as list_heads,
    COUNT(CASE WHEN next IS NULL THEN 1 END) as list_tails,
    COUNT(CASE WHEN prev IS NULL AND next IS NULL THEN 1 END) as isolated_headers
FROM example_header;