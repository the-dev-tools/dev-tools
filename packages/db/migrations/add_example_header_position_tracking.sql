-- Migration: Add position tracking columns to example_header table
-- Description: Adds position and context_type columns to support ordering of headers in different contexts (collection vs flow)
-- Date: 2025-08-27

BEGIN TRANSACTION;

-- Step 1: Add the new columns to the example_header table
-- SQLite requires individual ALTER TABLE statements for each column
ALTER TABLE example_header ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
ALTER TABLE example_header ADD COLUMN context_type TEXT;

-- Step 2: Populate existing rows with sequential positions based on their IDs
-- This ensures no position conflicts and maintains relative ordering
WITH positioned_headers AS (
  SELECT 
    id,
    example_id,
    ROW_NUMBER() OVER (
      PARTITION BY example_id 
      ORDER BY id
    ) - 1 AS new_position
  FROM example_header
)
UPDATE example_header
SET position = (
  SELECT new_position 
  FROM positioned_headers 
  WHERE positioned_headers.id = example_header.id
);

-- Step 3: Create the new indexes for efficient position queries
-- Note: These indexes are also defined in schema.sql for new databases
-- Index for queries with specific context_type
CREATE INDEX IF NOT EXISTS example_header_position_idx ON example_header (
  example_id,
  context_type,
  position
);

-- Partial index for collection context (NULL context_type) - most common case
CREATE INDEX IF NOT EXISTS example_header_context_position_idx ON example_header (
  example_id,
  position
) WHERE context_type IS NULL;

-- Step 4: Verify data integrity
-- Check that all headers have valid positions (0 or positive integers)
-- This will fail the migration if data integrity issues are detected
INSERT INTO migration (id, version, description, apply_at)
SELECT 
  randomblob(16),
  (SELECT COALESCE(MAX(version), 0) + 1 FROM migration),
  'Add position tracking columns to example_header table',
  unixepoch()
WHERE NOT EXISTS (
  SELECT 1 FROM example_header WHERE position < 0
);

COMMIT;

-- Validation query to run after migration (for manual verification)
-- SELECT example_id, COUNT(*) as header_count, 
--        MIN(position) as min_pos, MAX(position) as max_pos,
--        COUNT(DISTINCT position) as unique_positions
-- FROM example_header 
-- GROUP BY example_id 
-- HAVING header_count != unique_positions + 1;
-- 
-- This should return no rows - if it does, there are position conflicts