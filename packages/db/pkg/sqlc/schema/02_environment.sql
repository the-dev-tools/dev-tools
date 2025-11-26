-- TODO: env shouldn't active field it should be in workspace
CREATE TABLE environment (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  type INT8 NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  display_order REAL NOT NULL DEFAULT 0,
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX environment_idx1 ON environment (workspace_id, type, name);
CREATE INDEX environment_workspace_lookup ON environment (id, workspace_id);
CREATE INDEX environment_order_idx ON environment (workspace_id, display_order);

CREATE TABLE variable (
    id BLOB NOT NULL PRIMARY KEY,
    env_id BLOB NOT NULL,
    var_key TEXT NOT NULL,
    value TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT NOT NULL,
    display_order REAL NOT NULL DEFAULT 0,
    UNIQUE (env_id, var_key),
    FOREIGN KEY (env_id) REFERENCES environment(id) ON DELETE CASCADE
);

CREATE INDEX variable_idx1 ON variable (env_id, var_key);
CREATE INDEX variable_order_idx ON variable (env_id, display_order);
