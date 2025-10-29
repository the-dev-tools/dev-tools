-- Backfill script to populate the new float-based ordering columns
-- for environments and variables using the existing prev/next linked
-- list pointers. Run this once before dropping the prev/next columns.

BEGIN TRANSACTION;

-- Populate environment.display_order by walking the linked list per workspace.
WITH RECURSIVE ordered_environments AS (
  SELECT
    e.id,
    e.workspace_id,
    0 AS position
  FROM
    environment e
  WHERE
    e.prev IS NULL

  UNION ALL

  SELECT
    child.id,
    child.workspace_id,
    parent.position + 1 AS position
  FROM
    environment child
    INNER JOIN ordered_environments parent
      ON child.prev = parent.id
      AND child.workspace_id = parent.workspace_id
)
UPDATE environment
SET display_order = (
  SELECT position
  FROM ordered_environments oe
  WHERE oe.id = environment.id
    AND oe.workspace_id = environment.workspace_id
)
WHERE EXISTS (
  SELECT 1
  FROM ordered_environments oe
  WHERE oe.id = environment.id
    AND oe.workspace_id = environment.workspace_id
);

-- Populate variable.display_order by walking the linked list per environment.
WITH RECURSIVE ordered_variables AS (
  SELECT
    v.id,
    v.env_id,
    0 AS position
  FROM
    variable v
  WHERE
    v.prev IS NULL

  UNION ALL

  SELECT
    child.id,
    child.env_id,
    parent.position + 1 AS position
  FROM
    variable child
    INNER JOIN ordered_variables parent
      ON child.prev = parent.id
      AND child.env_id = parent.env_id
)
UPDATE variable
SET display_order = (
  SELECT position
  FROM ordered_variables ov
  WHERE ov.id = variable.id
    AND ov.env_id = variable.env_id
)
WHERE EXISTS (
  SELECT 1
  FROM ordered_variables ov
  WHERE ov.id = variable.id
    AND ov.env_id = variable.env_id
);

COMMIT;
