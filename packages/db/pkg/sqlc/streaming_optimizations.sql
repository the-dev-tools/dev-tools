-- ========================================
-- STREAMING OPTIMIZATIONS FOR HTTP SCHEMA
-- ========================================
-- Phase 2a: Real-time streaming workload optimizations
-- Focus: HTTP-related tables from Phase 1

-- ========================================
-- 1. STREAMING-SPECIFIC INDEXES
-- ========================================

-- Primary streaming index for time-based queries
-- Optimizes: "Get all HTTP entries updated since X for workspace Y"
CREATE INDEX http_workspace_streaming_idx ON http (workspace_id, updated_at DESC);

-- Composite index for workspace + method filtering (common in streaming)
-- Optimizes: "Get all POST requests for workspace Y updated since X"
CREATE INDEX http_workspace_method_streaming_idx ON http (workspace_id, method, updated_at DESC);

-- Index for delta resolution during streaming
-- Optimizes: "Find all deltas for parent HTTP X"
CREATE INDEX http_delta_resolution_idx ON http (parent_http_id, is_delta, updated_at DESC);

-- Folder-based streaming with time ordering
-- Optimizes: "Get all HTTP entries in folder X updated since Y"
CREATE INDEX http_folder_streaming_idx ON http (folder_id, updated_at DESC) WHERE folder_id IS NOT NULL;

-- ========================================
-- 2. SNAPSHOT GENERATION OPTIMIZATIONS
-- ========================================

-- Workspace snapshot with pre-joined data
-- This view reduces JOIN overhead for initial snapshots
CREATE VIEW http_workspace_snapshot AS
SELECT 
    h.id,
    h.workspace_id,
    h.folder_id,
    h.name,
    h.url,
    h.method,
    h.description,
    h.created_at,
    h.updated_at,
    -- Pre-computed delta flags for faster filtering
    CASE WHEN h.is_delta = TRUE THEN 1 ELSE 0 END as is_delta_flag,
    CASE WHEN h.parent_http_id IS NOT NULL THEN 1 ELSE 0 END as has_parent_flag
FROM http h
WHERE h.is_delta = FALSE; -- Only base records for snapshots

-- Index to support the snapshot view
CREATE INDEX http_snapshot_workspace_idx ON http (workspace_id, is_delta, updated_at DESC);

-- ========================================
-- 3. CONCURRENT STREAMING OPERATIONS
-- ========================================

-- Partial indexes for enabled items (commonly filtered in streaming)
CREATE INDEX http_search_param_enabled_idx ON http_search_param (http_id, enabled) WHERE enabled = TRUE;
CREATE INDEX http_header_enabled_idx ON http_header (http_id, enabled) WHERE enabled = TRUE;
CREATE INDEX http_body_form_enabled_idx ON http_body_form (http_id, enabled) WHERE enabled = TRUE;
CREATE INDEX http_body_urlencoded_enabled_idx ON http_body_urlencoded (http_id, enabled) WHERE enabled = TRUE;
CREATE INDEX http_assert_enabled_idx ON http_assert (http_id, enabled) WHERE enabled = TRUE;

-- Ordering indexes for linked-list traversal (critical for streaming consistency)
CREATE INDEX http_search_param_ordering_stream_idx ON http_search_param (http_id, prev, next);
CREATE INDEX http_header_ordering_stream_idx ON http_header (http_id, prev, next);
CREATE INDEX http_body_form_ordering_stream_idx ON http_body_form (http_id, prev, next);
CREATE INDEX http_body_urlencoded_ordering_stream_idx ON http_body_urlencoded (http_id, prev, next);
CREATE INDEX http_assert_ordering_stream_idx ON http_assert (http_id, prev, next);

-- ========================================
-- 4. BATCH OPERATIONS FOR STREAMING
-- ========================================

-- Composite indexes for batch updates during streaming
CREATE INDEX http_batch_update_idx ON http (workspace_id, folder_id, updated_at);
CREATE INDEX http_response_batch_idx ON http_response (http_id, executed_at DESC);

-- Response streaming optimization
CREATE INDEX http_response_streaming_idx ON http_response (http_id, status_code, executed_at DESC);
CREATE INDEX http_response_header_streaming_idx ON http_response_header (response_id, header_key);

-- ========================================
-- 5. FOREIGN KEY OPTIMIZATIONS
-- ========================================

-- Indexed foreign keys for better constraint performance
-- These help with concurrent operations and cascade operations
CREATE INDEX http_workspace_fk_optimized ON http (workspace_id);
CREATE INDEX http_folder_fk_optimized ON http (folder_id) WHERE folder_id IS NOT NULL;
CREATE INDEX http_parent_fk_optimized ON http (parent_http_id) WHERE parent_http_id IS NOT NULL;

-- Child table foreign key optimizations
CREATE INDEX http_search_param_http_fk_optimized ON http_search_param (http_id);
CREATE INDEX http_header_http_fk_optimized ON http_header (http_id);
CREATE INDEX http_body_form_http_fk_optimized ON http_body_form (http_id);
CREATE INDEX http_body_urlencoded_http_fk_optimized ON http_body_urlencoded (http_id);
CREATE INDEX http_body_raw_http_fk_optimized ON http_body_raw (http_id);
CREATE INDEX http_response_http_fk_optimized ON http_response (http_id);
CREATE INDEX http_assert_http_fk_optimized ON http_assert (http_id);

-- ========================================
-- 6. REAL-TIME MONITORING INDEXES
-- ========================================

-- Recent activity monitoring (for streaming dashboards)
CREATE INDEX http_recent_activity_idx ON http (updated_at DESC, workspace_id) WHERE updated_at > (strftime('%s', 'now') - 86400);
CREATE INDEX http_response_recent_idx ON http_response (executed_at DESC, http_id) WHERE executed_at > (strftime('%s', 'now') - 86400);

-- Error tracking for streaming (failed responses)
CREATE INDEX http_response_errors_idx ON http_response (http_id, status_code, executed_at DESC) WHERE status_code >= 400;

-- ========================================
-- 7. PARTIAL INDEXES FOR COMMON FILTERS
-- ========================================

-- Only index non-delta records for most streaming queries
CREATE INDEX http_base_records_idx ON http (workspace_id, updated_at DESC) WHERE is_delta = FALSE;

-- Only index delta records for delta resolution
CREATE INDEX http_delta_records_idx ON http (parent_http_id, updated_at DESC) WHERE is_delta = TRUE;

-- Active versions only
CREATE INDEX http_active_versions_idx ON http_version (http_id, is_active) WHERE is_active = TRUE;

-- ========================================
-- 8. STREAMING QUERY OPTIMIZATIONS
-- ========================================

-- Materialized view-like approach for frequently accessed data
-- This table can be refreshed periodically for streaming performance
CREATE TABLE IF NOT EXISTS http_streaming_cache (
    http_id BLOB PRIMARY KEY,
    workspace_id BLOB NOT NULL,
    folder_id BLOB,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    method TEXT NOT NULL,
    last_updated BIGINT NOT NULL,
    delta_count INTEGER DEFAULT 0,
    response_count INTEGER DEFAULT 0,
    last_response_status INTEGER,
    cache_updated_at BIGINT NOT NULL DEFAULT (unixepoch())
);

-- Indexes for the cache table
CREATE INDEX http_streaming_cache_workspace_idx ON http_streaming_cache (workspace_id, last_updated DESC);
CREATE INDEX http_streaming_cache_method_idx ON http_streaming_cache (method, last_updated DESC);
CREATE INDEX http_streaming_cache_errors_idx ON http_streaming_cache (last_response_status, last_updated DESC) WHERE last_response_status >= 400;

-- ========================================
-- 9. CONCURRENCY CONTROL INDEXES
-- ========================================

-- Locking optimization indexes to reduce contention
CREATE INDEX http_lock_workspace_idx ON http (workspace_id, id);
CREATE INDEX http_lock_folder_idx ON http (folder_id, id) WHERE folder_id IS NOT NULL;

-- Child table locking optimizations
CREATE INDEX http_search_param_lock_idx ON http_search_param (http_id, id);
CREATE INDEX http_header_lock_idx ON http_header (http_id, id);
CREATE INDEX http_body_form_lock_idx ON http_body_form (http_id, id);
CREATE INDEX http_body_urlencoded_lock_idx ON http_body_urlencoded (http_id, id);

-- ========================================
-- 10. PERFORMANCE MONITORING
-- ========================================

-- Table to track streaming performance metrics
CREATE TABLE IF NOT EXISTS streaming_performance_log (
    id BLOB PRIMARY KEY,
    workspace_id BLOB NOT NULL,
    operation_type TEXT NOT NULL, -- 'snapshot', 'delta', 'query', 'update'
    table_name TEXT NOT NULL,
    record_count INTEGER NOT NULL,
    duration_ms INTEGER NOT NULL,
    timestamp BIGINT NOT NULL DEFAULT (unixepoch()),
    error_message TEXT
);

CREATE INDEX streaming_perf_workspace_idx ON streaming_performance_log (workspace_id, timestamp DESC);
CREATE INDEX streaming_perf_operation_idx ON streaming_performance_log (operation_type, timestamp DESC);
CREATE INDEX streaming_perf_table_idx ON streaming_performance_log (table_name, timestamp DESC);