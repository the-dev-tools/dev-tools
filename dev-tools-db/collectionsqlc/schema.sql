CREATE TABLE IF NOT EXISTS collections (
    id BLOB PRIMARY KEY,
    owner_id BLOB NOT NULL,
    name TEXT NOT NULL
)

CREATE INDEX Idx1 ON collections(owner_id);


