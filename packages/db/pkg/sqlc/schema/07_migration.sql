CREATE TABLE migration (
  id BLOB NOT NULL PRIMARY KEY,
  version INT NOT NULL,
  description TEXT NOT NULL,
  apply_at BIGINT NOT NULL
);
