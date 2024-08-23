CREATE TABLE users (
    id BLOB PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash BLOB,
    provider_type INT8 NOT NULL DEFAULT 0,
    provider_id TEXT,
    status INT8 NOT NULL DEFAULT 0,
    CHECK( status IN (0,1,2,3) ),
    CHECK( provider_type IN (0,1,2,3) ),
    UNIQUE(provider_type, provider_id)
);

CREATE TABLE collections (
    id BLOB PRIMARY KEY,
    owner_id BLOB NOT NULL,
    name TEXT NOT NULL
);

CREATE INDEX Idx1 ON collections(owner_id);

CREATE TABLE item_api (
        id BLOB PRIMARY KEY,
        collection_id BLOB NOT NULL,
        parent_id BLOB,
        name TEXT NOT NULL,
        url TEXT NOT NULL,
        method TEXT NOT NULL,
        headers BLOB NOT NULL,
        query BLOB NOT NULL,
        body BLOB NOT NULL,
        FOREIGN KEY (collection_id) REFERENCES collections(id) ON DELETE CASCADE,
        FOREIGN KEY (parent_id) REFERENCES item_folder(id) ON DELETE CASCADE
);

CREATE INDEX Idx2 ON item_api(collection_id, parent_id);

CREATE TABLE item_folder (
        id BLOB PRIMARY KEY,
        collection_id BLOB NOT NULL,
        parent_id BLOB,
        name TEXT NOT NULL,
        FOREIGN KEY (collection_id) REFERENCES collections (id) ON DELETE CASCADE,
        FOREIGN KEY (parent_id) REFERENCES item_folder(id) ON DELETE CASCADE
);

CREATE INDEX Idx3 ON item_folder(collection_id, parent_id);

CREATE TABLE workspaces (
        id BLOB PRIMARY KEY,
        name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS workspaces_users (
        id BLOB PRIMARY KEY,
        workspace_id BLOB NOT NULL,
        user_id BLOB NOT NULL,
        UNIQUE(workspace_id, user_id),
        FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
)
