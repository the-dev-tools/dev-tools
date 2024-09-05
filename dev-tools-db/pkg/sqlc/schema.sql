-- USERS 
CREATE TABLE users (
  id BLOB NOT NULL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB,
  provider_type INT8 NOT NULL DEFAULT 0,
  provider_id TEXT,
  status INT8 NOT NULL DEFAULT 0,
  CHECK (status IN (0, 1, 2, 3)),
  CHECK (provider_type IN (0, 1, 2, 3)),
  UNIQUE (provider_type, provider_id)
);

-- COLLECTIONS
CREATE TABLE collections (
  id BLOB NOT NULL PRIMARY KEY,
  owner_id BLOB NOT NULL,
  name TEXT NOT NULL,
  updated TIMESTAMP NOT NULL DEFAULT (unixepoch ()),
  FOREIGN KEY (owner_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX Idx1 ON collections (owner_id);

CREATE TRIGGER insert_collections AFTER INSERT ON collections BEGIN
UPDATE workspaces
SET
  updated = unixepoch ()
WHERE
  id = NEW.owner_id;

END;

CREATE TRIGGER update_collections AFTER
UPDATE ON collections BEGIN
UPDATE workspaces
SET
  updated = unixepoch ()
WHERE
  id = NEW.owner_id;

END;

CREATE TRIGGER delete_collections AFTER DELETE ON collections BEGIN
UPDATE workspaces
SET
  updated = unixepoch ()
WHERE
  id = OLD.owner_id;

END;

-- ITEM API
CREATE TABLE item_api (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  parent_id BLOB,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  method TEXT NOT NULL,
  updated TIMESTAMP NOT NULL DEFAULT (unixepoch ()),
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
);

CREATE INDEX Idx2 ON item_api (collection_id, parent_id);

CREATE TRIGGER update_item_api AFTER
UPDATE ON item_api BEGIN
UPDATE collections
SET
  updated = unixepoch ()
WHERE
  id = NEW.collection_id;

UPDATE item_api
SET
  updated = unixepoch ()
WHERE
  id = NEW.id;

END;

CREATE TRIGGER delete_item_api AFTER DELETE ON item_api BEGIN
UPDATE collections
SET
  updated = unixepoch ()
WHERE
  id = OLD.collection_id;

END;

-- ITEM API EXAMPLE
CREATE TABLE item_api_example (
  id BLOB NOT NULL PRIMARY KEY,
  item_api_id BLOB NOT NULL,
  collection_id BLOB NOT NULL,
  parent_example_id BLOB,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,
  name TEXT NOT NULL,
  headers BLOB NOT NULL,
  query BLOB NOT NULL,
  compressed BOOLEAN NOT NULL DEFAULT FALSE,
  body BLOB NOT NULL,
  updated TIMESTAMP NOT NULL DEFAULT (unixepoch ()),
  FOREIGN KEY (parent_example_id) REFERENCES item_api_example (id) ON DELETE CASCADE,
  FOREIGN KEY (item_api_id) REFERENCES item_api (id) ON DELETE CASCADE,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE
);

CREATE INDEX item_api_example_idx1 ON item_api_example (
  item_api_id,
  collection_id,
  is_default,
  parent_example_id
);

CREATE TRIGGER update_item_api_example AFTER
UPDATE ON item_api_example BEGIN
UPDATE item_api
SET
  updated = unixepoch ()
WHERE
  id = NEW.item_api_id;

UPDATE item_api_example
SET
  updated = unixepoch ()
WHERE
  rowid = NEW.rowid;

END;

CREATE TRIGGER delete_item_api_example AFTER DELETE ON item_api_example BEGIN
UPDATE item_api
SET
  updated = unixepoch ()
WHERE
  id = OLD.item_api_id;

END;

-- ITEM FOLDER
CREATE TABLE item_folder (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  parent_id BLOB,
  name TEXT NOT NULL,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
);

CREATE TRIGGER update_item_folder AFTER
UPDATE ON item_folder BEGIN
UPDATE collections
SET
  updated = unixepoch ()
WHERE
  id = NEW.collection_id;

END;

CREATE TRIGGER delete_item_folder AFTER DELETE ON item_folder BEGIN
UPDATE collections
SET
  updated = unixepoch ()
WHERE
  id = OLD.collection_id;

END;

CREATE INDEX Idx3 ON item_folder (collection_id, parent_id);

-- WORK SPACES 
CREATE TABLE workspaces (
  id BLOB NOT NULL PRIMARY KEY,
  name TEXT NOT NULL,
  updated TIMESTAMP NOT NULL DEFAULT (unixepoch ())
);

-- WORKSPACE USERS
CREATE TABLE workspaces_users (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  user_id BLOB NOT NULL,
  role INT8 NOT NULL DEFAULT 1,
  CHECK (role IN (1, 2, 3)),
  UNIQUE (workspace_id, user_id),
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE,
  FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
);

-- RESULT API
CREATE TABLE result_api (
  id BLOB NOT NULL PRIMARY KEY,
  trigger_type TINYINT,
  trigger_by BLOB,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  time TIMESTAMP NOT NULL DEFAULT (unixepoch ()),
  duration BIGINT NOT NULL,
  http_resp BLOB
);

CREATE INDEX Idx4 ON result_api (trigger_by, trigger_type);
