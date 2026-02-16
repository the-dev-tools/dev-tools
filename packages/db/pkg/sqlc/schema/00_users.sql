-- USERS
CREATE TABLE users (
  id BLOB NOT NULL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash BLOB,
  provider_type INT8 NOT NULL DEFAULT 0,
  provider_id TEXT,
  external_id TEXT UNIQUE,
  status INT8 NOT NULL DEFAULT 0,
  name TEXT NOT NULL DEFAULT '',
  image TEXT,
  UNIQUE (provider_type, provider_id)
);
