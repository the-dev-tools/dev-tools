-- Test variable ordering implementation
-- This file demonstrates the variable ordering functionality

-- Create test environment
INSERT INTO environment (id, workspace_id, type, name, description) 
VALUES (X'0123456789ABCDEF0123456789ABCDEF', X'1111111111111111111111111111111111', 0, 'Test Environment', 'Test description');

-- Create test variables in order
-- Variable 1 (head of list - prev = NULL)
INSERT INTO variable (id, env_id, var_key, value, enabled, description, prev, next) 
VALUES (X'VAR1111111111111111111111111111111', X'0123456789ABCDEF0123456789ABCDEF', 'VAR_1', 'value1', true, 'First variable', NULL, X'VAR2222222222222222222222222222222');

-- Variable 2 (middle)
INSERT INTO variable (id, env_id, var_key, value, enabled, description, prev, next) 
VALUES (X'VAR2222222222222222222222222222222', X'0123456789ABCDEF0123456789ABCDEF', 'VAR_2', 'value2', true, 'Second variable', X'VAR1111111111111111111111111111111', X'VAR3333333333333333333333333333333');

-- Variable 3 (tail of list - next = NULL)
INSERT INTO variable (id, env_id, var_key, value, enabled, description, prev, next) 
VALUES (X'VAR3333333333333333333333333333333', X'0123456789ABCDEF0123456789ABCDEF', 'VAR_3', 'value3', true, 'Third variable', X'VAR2222222222222222222222222222222', NULL);

-- Test the ordered query
-- Should return variables in order: VAR_1, VAR_2, VAR_3
WITH RECURSIVE ordered_variables AS (
  -- Base case: Find the head (prev IS NULL)
  SELECT
    v.id,
    v.env_id,
    v.var_key,
    v.value,
    v.enabled,
    v.description,
    v.prev,
    v.next,
    0 as position
  FROM
    variable v
  WHERE
    v.env_id = X'0123456789ABCDEF0123456789ABCDEF' AND
    v.prev IS NULL
  
  UNION ALL
  
  -- Recursive case: Follow the next pointers
  SELECT
    v.id,
    v.env_id,
    v.var_key,
    v.value,
    v.enabled,
    v.description,
    v.prev,
    v.next,
    ov.position + 1
  FROM
    variable v
  INNER JOIN ordered_variables ov ON v.prev = ov.id
  WHERE
    v.env_id = X'0123456789ABCDEF0123456789ABCDEF'
)
SELECT
  ov.var_key,
  ov.value,
  ov.position
FROM
  ordered_variables ov
ORDER BY
  ov.position;

-- Expected output:
-- VAR_1 | value1 | 0
-- VAR_2 | value2 | 1
-- VAR_3 | value3 | 2

-- Test moving VAR_2 to the front
-- 1. Update VAR_1 to point to VAR_3 (skip VAR_2)
UPDATE variable 
SET next = X'VAR3333333333333333333333333333333'
WHERE id = X'VAR1111111111111111111111111111111';

-- 2. Update VAR_3 to point back to VAR_1 (skip VAR_2)
UPDATE variable 
SET prev = X'VAR1111111111111111111111111111111'
WHERE id = X'VAR3333333333333333333333333333333';

-- 3. Update VAR_2 to become new head
UPDATE variable 
SET prev = NULL, next = X'VAR1111111111111111111111111111111'
WHERE id = X'VAR2222222222222222222222222222222';

-- 4. Update VAR_1 to point back to VAR_2
UPDATE variable 
SET prev = X'VAR2222222222222222222222222222222'
WHERE id = X'VAR1111111111111111111111111111111';

-- Verify new order: should be VAR_2, VAR_1, VAR_3
WITH RECURSIVE ordered_variables AS (
  SELECT
    v.id,
    v.env_id,
    v.var_key,
    v.value,
    v.enabled,
    v.description,
    v.prev,
    v.next,
    0 as position
  FROM
    variable v
  WHERE
    v.env_id = X'0123456789ABCDEF0123456789ABCDEF' AND
    v.prev IS NULL
  
  UNION ALL
  
  SELECT
    v.id,
    v.env_id,
    v.var_key,
    v.value,
    v.enabled,
    v.description,
    v.prev,
    v.next,
    ov.position + 1
  FROM
    variable v
  INNER JOIN ordered_variables ov ON v.prev = ov.id
  WHERE
    v.env_id = X'0123456789ABCDEF0123456789ABCDEF'
)
SELECT
  ov.var_key,
  ov.value,
  ov.position
FROM
  ordered_variables ov
ORDER BY
  ov.position;

-- Expected output after move:
-- VAR_2 | value2 | 0
-- VAR_1 | value1 | 1
-- VAR_3 | value3 | 2