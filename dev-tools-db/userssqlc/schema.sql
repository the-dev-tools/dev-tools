CREATE TABLE IF NOT EXISTS users (
    id BLOB PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash BLOB,
    platform_type INTEGER,
    platform_id TEXT UNIQUE,
    UNIQUE(platform_type, platform_id)

)
