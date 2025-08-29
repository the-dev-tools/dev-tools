-- Test script to validate the new header position tracking schema changes
-- This script creates a minimal schema and tests the new functionality

BEGIN TRANSACTION;

-- Create minimal required tables for testing
CREATE TABLE item_api_example (
  id BLOB NOT NULL PRIMARY KEY,
  name TEXT NOT NULL
);

-- Create the updated example_header table with new columns
CREATE TABLE example_header (
  id BLOB NOT NULL PRIMARY KEY,
  example_id BLOB NOT NULL,
  delta_parent_id BLOB DEFAULT NULL,
  header_key TEXT NOT NULL,
  enable BOOLEAN NOT NULL DEFAULT TRUE,
  description TEXT NOT NULL,
  value TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  context_type TEXT,
  FOREIGN KEY (example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (delta_parent_id) REFERENCES example_header (id) ON DELETE CASCADE
);

-- Create the new indexes
CREATE INDEX example_header_position_idx ON example_header (
  example_id,
  context_type,
  position
);

CREATE INDEX example_header_context_position_idx ON example_header (
  example_id,
  position
) WHERE context_type IS NULL;

-- Insert test data
INSERT INTO item_api_example (id, name) VALUES 
  (randomblob(16), 'Test Example');

-- Get the example ID for reference
CREATE TEMP TABLE test_example AS 
SELECT id as example_id FROM item_api_example LIMIT 1;

-- Test collection context headers (context_type = NULL)
INSERT INTO example_header (id, example_id, header_key, enable, description, value, position, context_type) 
SELECT randomblob(16), example_id, 'Authorization', TRUE, 'Auth header', 'Bearer token123', 0, NULL FROM test_example;

INSERT INTO example_header (id, example_id, header_key, enable, description, value, position, context_type) 
SELECT randomblob(16), example_id, 'Content-Type', TRUE, 'Content type', 'application/json', 1, NULL FROM test_example;

INSERT INTO example_header (id, example_id, header_key, enable, description, value, position, context_type) 
SELECT randomblob(16), example_id, 'Accept', TRUE, 'Accept header', 'application/json', 2, NULL FROM test_example;

-- Test flow context headers (context_type = 'flow')
INSERT INTO example_header (id, example_id, header_key, enable, description, value, position, context_type) 
SELECT randomblob(16), example_id, 'X-Flow-ID', TRUE, 'Flow tracking', 'flow-123', 0, 'flow' FROM test_example;

INSERT INTO example_header (id, example_id, header_key, enable, description, value, position, context_type) 
SELECT randomblob(16), example_id, 'Content-Type', TRUE, 'Flow content type', 'application/xml', 1, 'flow' FROM test_example;

-- Test queries to validate functionality

-- 1. Get headers in collection order
.print "=== Collection Context Headers (ordered by position) ==="
SELECT header_key, position, context_type FROM example_header 
WHERE example_id = (SELECT example_id FROM test_example) AND context_type IS NULL 
ORDER BY position;

-- 2. Get headers in flow order
.print "=== Flow Context Headers (ordered by position) ==="
SELECT header_key, position, context_type FROM example_header 
WHERE example_id = (SELECT example_id FROM test_example) AND context_type = 'flow'
ORDER BY position;

-- 3. Test getting max position for new header insertion
.print "=== Next position for collection context ==="
SELECT COALESCE(MAX(position), -1) + 1 as next_position
FROM example_header
WHERE example_id = (SELECT example_id FROM test_example) AND context_type IS NULL;

.print "=== Next position for flow context ==="
SELECT COALESCE(MAX(position), -1) + 1 as next_position
FROM example_header
WHERE example_id = (SELECT example_id FROM test_example) AND context_type = 'flow';

-- 4. Test position update (simulating drag-and-drop)
.print "=== Before position update ==="
SELECT header_key, position FROM example_header 
WHERE example_id = (SELECT example_id FROM test_example) AND context_type IS NULL
ORDER BY position;

-- Move "Accept" header from position 2 to position 0 (make it first)
-- First shift other headers down
UPDATE example_header 
SET position = position + 1 
WHERE example_id = (SELECT example_id FROM test_example)
  AND context_type IS NULL 
  AND position >= 0 AND position < 2;

-- Then set the moved header to position 0
UPDATE example_header 
SET position = 0 
WHERE header_key = 'Accept' AND example_id = (SELECT example_id FROM test_example);

.print "=== After position update (Accept moved to first) ==="
SELECT header_key, position FROM example_header 
WHERE example_id = (SELECT example_id FROM test_example) AND context_type IS NULL
ORDER BY position;

-- 5. Verify indexes are being used
.print "=== Query plan for position-ordered query ==="
EXPLAIN QUERY PLAN 
SELECT * FROM example_header 
WHERE example_id = (SELECT example_id FROM test_example) AND context_type IS NULL 
ORDER BY position;

COMMIT;

.print "=== Schema validation completed successfully! ==="