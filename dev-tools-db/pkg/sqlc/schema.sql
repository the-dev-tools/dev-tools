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

CREATE VIEW collections_view AS
    SELECT * FROM collections;

CREATE TRIGGER insert_collections_view 
INSTEAD OF INSERT ON collections_view 
BEGIN
  INSERT INTO collections (id, owner_id, name, updated)
  VALUES (NEW.id, NEW.owner_id, NEW.name, NEW.updated);

  UPDATE workspaces SET
  updated = unixepoch()
  WHERE
  id = NEW.owner_id;
END;

CREATE TRIGGER update_collections_view 
INSTEAD OF UPDATE ON collections_view 
BEGIN
  UPDATE collections SET
  name = NEW.name,
  updated = unixepoch()
  WHERE
  id = NEW.id AND (OLD.name IS NOT NEW.name);
  UPDATE workspaces SET
  updated = unixepoch()
  WHERE
  id = NEW.owner_id; 
END;

CREATE TRIGGER delete_collections_view 
INSTEAD OF DELETE ON collections_view 
BEGIN
  DELETE FROM collections
  WHERE
  id = OLD.id;
  UPDATE workspaces SET
  updated = unixepoch()
  WHERE
  id = OLD.owner_id;
END;

/*
*
* ITEM FOLDER
*
*/
-- ITEM FOLDER BASE TABLE
CREATE TABLE item_folder (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  parent_id BLOB,
  name TEXT NOT NULL,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
);

CREATE INDEX Idx3 ON item_folder (collection_id, parent_id);

CREATE VIEW item_folder_view AS
SELECT * FROM item_folder;

CREATE TRIGGER insert_item_folder_view 
INSTEAD OF INSERT ON item_folder_view 
BEGIN
  INSERT INTO item_folder (id, collection_id, parent_id, name)
  VALUES (NEW.id, NEW.collection_id, NEW.parent_id, NEW.name);

  UPDATE collections_view SET
  updated = unixepoch()
  WHERE
  id = NEW.collection_id;
END;

CREATE TRIGGER update_item_folder_view
INSTEAD OF UPDATE ON item_folder_view
BEGIN
    UPDATE item_folder SET
    name = NEW.name,
    parent_id = new.parent_id
    WHERE
    id = NEW.id AND (
        (OLD.name IS NOT NEW.name) OR
        (OLD.parent_id IS NOT NEW.name)
    );

    UPDATE collections_view SET
    updated = unixepoch()
    WHERE
    id = NEW.collection_id;
END;

CREATE TRIGGER delete_item_folder_view
INSTEAD OF DELETE ON item_folder_view
BEGIN
    DELETE FROM item_folder
    WHERE
    id = OLD.id;
    UPDATE collections_view SET
    updated = unixepoch()
    WHERE
    id = OLD.collection_id;
END;

/*
* 
* ITEM API
*
*/

-- ITEM API
CREATE TABLE item_api (
  id BLOB NOT NULL PRIMARY KEY,
  collection_id BLOB NOT NULL,
  parent_id BLOB,
  name TEXT NOT NULL,
  url TEXT NOT NULL,
  method TEXT NOT NULL,
  FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
  FOREIGN KEY (parent_id) REFERENCES item_folder (id) ON DELETE CASCADE
);

CREATE INDEX Idx2 ON item_api (collection_id, parent_id);

-- ITEM API VIEW
CREATE VIEW item_api_view AS
SELECT
  *
FROM
  item_api;

CREATE TRIGGER insert_item_api_view
INSTEAD OF INSERT ON item_api_view
BEGIN
    INSERT INTO item_api (id, collection_id, parent_id, name, url, method)
    VALUES (NEW.id, NEW.collection_id, NEW.parent_id, NEW.name, NEW.url, NEW.method);

    UPDATE collections_view SET
    updated = unixepoch()
    WHERE
    id = NEW.collection_id;
END;

CREATE TRIGGER update_item_api_view
INSTEAD OF UPDATE ON item_api_view
BEGIN
    UPDATE item_api SET
    name = NEW.name,
    url = NEW.url,
    method = NEW.method
    WHERE
    id = NEW.id AND (
        (OLD.name IS NOT NEW.name) OR
        (OLD.url IS NOT NEW.url) OR
        (OLD.method IS NOT NEW.method)
    );

    UPDATE collections_view SET
    updated = unixepoch()
    WHERE
    id = NEW.collection_id;
END;

CREATE TRIGGER delete_item_api_view
INSTEAD OF DELETE ON item_api_view
BEGIN
    DELETE FROM item_api
    WHERE id = NEW.id;

    UPDATE collections_view SET
    updated = unixepoch()
    WHERE id = NEW.collection_id;
END;


/*
*
* ITEM API EXAMPLE
*
*/
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

CREATE VIEW item_api_example_view AS
SELECT * FROM item_api_example;

CREATE TRIGGER insert_item_api_example_view
INSTEAD OF INSERT ON item_api_example_view
BEGIN
    INSERT INTO item_api_example 
    (id, item_api_id, collection_id, parent_example_id, 
    is_default, name, headers, query, compressed, body)
    VALUES 
    (NEW.id, NEW.item_api_id, NEW.collection_id, NEW.parent_example_id, 
    NEW.is_default, NEW.name, NEW.headers, NEW.query, NEW.compressed, NEW.body);

    UPDATE item_api_view SET
    updated = unixepoch()
    WHERE
    id = NEW.item_api_id;
    UPDATE collections_view SET
    updated = unixepoch()
    WHERE
    id = NEW.collection_id;
END;

CREATE TRIGGER update_item_api_example_view
INSTEAD OF UPDATE ON item_api_example_view
BEGIN
    UPDATE item_api_example SET
    name = NEW.name,
    headers = NEW.headers,
    query = NEW.query,
    compressed = NEW.compressed,
    body = NEW.body
    WHERE
    id = NEW.id AND (
        (OLD.name IS NOT NEW.name) OR
        (OLD.headers IS NOT NEW.headers) OR
        (OLD.query IS NOT NEW.query) OR
        (OLD.compressed IS NOT NEW.compressed) OR
        (OLD.body IS NOT NEW.body)
    );

    UPDATE item_api_view SET
    updated = unixepoch()
    WHERE
    id = NEW.item_api_id;
    UPDATE collections_view SET
    updated = unixepoch()
    WHERE
    id = NEW.collection_id;
END;

CREATE TRIGGER delete_item_api_example_view
INSTEAD OF DELETE ON item_api_example_view
BEGIN
    DELETE FROM item_api_example
    WHERE
    id = OLD.id;

    UPDATE item_api_view SET
    updated = unixepoch()
    WHERE
    id = OLD.item_api_id;
    UPDATE collections_view SET
    updated = unixepoch()
    WHERE
    id = OLD.collection_id;
END;

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

