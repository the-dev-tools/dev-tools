/*
 *
 * GRAPHQL DELTA SYSTEM
 * Adds delta/variant support to GraphQL tables for flow node overrides
 *
 */

-- Add delta system fields to graphql table
ALTER TABLE graphql ADD COLUMN parent_graphql_id BLOB DEFAULT NULL;
ALTER TABLE graphql ADD COLUMN is_delta BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE graphql ADD COLUMN is_snapshot BOOLEAN NOT NULL DEFAULT FALSE;

-- Add delta override fields to graphql table
ALTER TABLE graphql ADD COLUMN delta_name TEXT NULL;
ALTER TABLE graphql ADD COLUMN delta_url TEXT NULL;
ALTER TABLE graphql ADD COLUMN delta_query TEXT NULL;
ALTER TABLE graphql ADD COLUMN delta_variables TEXT NULL;
ALTER TABLE graphql ADD COLUMN delta_description TEXT NULL;

-- Add foreign key for parent relationship (SQLite requires recreating the table)
-- Since we can't add FK constraints to existing tables in SQLite, we'll handle this
-- at the application level for now and add it in the next major migration

-- Add indexes for delta resolution and performance
CREATE INDEX graphql_parent_delta_idx ON graphql (parent_graphql_id, is_delta);
CREATE INDEX graphql_delta_resolution_idx ON graphql (parent_graphql_id, is_delta, updated_at DESC);
CREATE INDEX graphql_active_streaming_idx ON graphql (workspace_id, updated_at DESC) WHERE is_delta = FALSE;

-- Add delta system fields to graphql_header table
ALTER TABLE graphql_header ADD COLUMN parent_graphql_header_id BLOB DEFAULT NULL;
ALTER TABLE graphql_header ADD COLUMN is_delta BOOLEAN NOT NULL DEFAULT FALSE;

-- Add delta override fields to graphql_header table
ALTER TABLE graphql_header ADD COLUMN delta_header_key TEXT NULL;
ALTER TABLE graphql_header ADD COLUMN delta_header_value TEXT NULL;
ALTER TABLE graphql_header ADD COLUMN delta_description TEXT NULL;
ALTER TABLE graphql_header ADD COLUMN delta_enabled BOOLEAN NULL;
ALTER TABLE graphql_header ADD COLUMN delta_display_order REAL NULL;

-- Add indexes for graphql_header delta support
CREATE INDEX graphql_header_parent_delta_idx ON graphql_header (parent_graphql_header_id, is_delta);
CREATE INDEX graphql_header_delta_streaming_idx ON graphql_header (parent_graphql_header_id, is_delta, updated_at DESC);

-- Add delta system fields to graphql_assert table
ALTER TABLE graphql_assert ADD COLUMN parent_graphql_assert_id BLOB DEFAULT NULL;
ALTER TABLE graphql_assert ADD COLUMN is_delta BOOLEAN NOT NULL DEFAULT FALSE;

-- Add delta override fields to graphql_assert table
ALTER TABLE graphql_assert ADD COLUMN delta_value TEXT NULL;
ALTER TABLE graphql_assert ADD COLUMN delta_enabled BOOLEAN NULL;
ALTER TABLE graphql_assert ADD COLUMN delta_description TEXT NULL;
ALTER TABLE graphql_assert ADD COLUMN delta_display_order REAL NULL;

-- Add indexes for graphql_assert delta support
CREATE INDEX graphql_assert_parent_delta_idx ON graphql_assert (parent_graphql_assert_id, is_delta);
CREATE INDEX graphql_assert_delta_streaming_idx ON graphql_assert (parent_graphql_assert_id, is_delta, updated_at DESC);
