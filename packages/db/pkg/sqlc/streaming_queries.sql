-- ========================================
-- STREAMING-OPTIMIZED QUERIES FOR HTTP
-- ========================================
-- Phase 2a: High-performance queries for real-time streaming

-- ========================================
-- 1. SNAPSHOT GENERATION QUERIES
-- ========================================

-- name: GetHTTPWorkspaceSnapshot :many
-- Optimized snapshot query for initial workspace state
-- Uses composite index (workspace_id, is_delta, updated_at DESC)
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
  h.parent_http_id,
  h.is_delta
FROM http h
WHERE h.workspace_id = ?
  AND h.is_delta = FALSE
ORDER BY h.updated_at DESC
LIMIT ?;  -- Pagination support

-- name: GetHTTPWorkspaceSnapshotSince :many
-- Incremental snapshot for delta updates since timestamp
-- Optimized for streaming: only returns changes since last update
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
  h.parent_http_id,
  h.is_delta,
  CASE 
    WHEN h.updated_at > ? THEN 'updated'
    WHEN h.created_at > ? THEN 'created'
    ELSE 'unchanged'
  END as change_type
FROM http h
WHERE h.workspace_id = ?
  AND (h.updated_at > ? OR h.created_at > ?)
ORDER BY h.updated_at ASC;  -- ASC for chronological processing

-- name: GetHTTPDeltasSince :many
-- Get all delta changes for a parent HTTP since timestamp
-- Critical for delta resolution in streaming
SELECT
  h.id,
  h.parent_http_id,
  h.delta_name,
  h.delta_url,
  h.delta_method,
  h.delta_description,
  h.updated_at
FROM http h
WHERE h.parent_http_id = ?
  AND h.is_delta = TRUE
  AND h.updated_at > ?
ORDER BY h.updated_at ASC;

-- ========================================
-- 2. REAL-TIME STREAMING QUERIES
-- ========================================

-- name: StreamHTTPByWorkspace :many
-- Primary streaming query for workspace-scoped HTTP changes
-- Uses http_workspace_streaming_idx (workspace_id, updated_at DESC)
SELECT
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.updated_at,
  h.created_at,
  h.is_delta,
  h.parent_http_id
FROM http h
WHERE h.workspace_id = ?
  AND h.updated_at > ?
ORDER BY h.updated_at ASC
LIMIT ?;  -- Batch size control for streaming

-- name: StreamHTTPByWorkspaceAndMethod :many
-- Method-filtered streaming for specialized consumers
-- Uses http_workspace_method_streaming_idx (workspace_id, method, updated_at DESC)
SELECT
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.updated_at,
  h.created_at
FROM http h
WHERE h.workspace_id = ?
  AND h.method = ?
  AND h.updated_at > ?
ORDER BY h.updated_at ASC
LIMIT ?;

-- name: StreamHTTPByFolder :many
-- Folder-scoped streaming for hierarchical updates
-- Uses http_folder_streaming_idx (folder_id, updated_at DESC)
SELECT
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.updated_at,
  h.created_at
FROM http h
WHERE h.folder_id = ?
  AND h.updated_at > ?
ORDER BY h.updated_at ASC
LIMIT ?;

-- ========================================
-- 3. BATCH OPERATIONS FOR STREAMING
-- ========================================

-- name: GetHTTPBatchForProcessing :many
-- Batch query for high-throughput processing
-- Uses http_batch_update_idx (workspace_id, folder_id, updated_at)
SELECT
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.updated_at,
  h.created_at
FROM http h
WHERE h.workspace_id = ?
  AND h.updated_at BETWEEN ? AND ?
ORDER BY h.workspace_id, h.folder_id, h.updated_at
LIMIT ?;  -- Configurable batch size

-- name: GetHTTPWithChildDataBatch :many
-- Optimized batch query with pre-joined child data
-- Reduces N+1 query problems in streaming
SELECT
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.updated_at,
  h.created_at,
  -- Pre-aggregated child counts for faster filtering
  (SELECT COUNT(*) FROM http_search_param sp WHERE sp.http_id = h.id AND sp.enabled = TRUE) as param_count,
  (SELECT COUNT(*) FROM http_header hh WHERE hh.http_id = h.id AND hh.enabled = TRUE) as header_count,
  (SELECT COUNT(*) FROM http_body_form bf WHERE bf.http_id = h.id AND bf.enabled = TRUE) as form_count,
  (SELECT COUNT(*) FROM http_assert ha WHERE ha.http_id = h.id AND ha.enabled = TRUE) as assert_count
FROM http h
WHERE h.workspace_id = ?
  AND h.updated_at > ?
ORDER BY h.updated_at ASC
LIMIT ?;

-- ========================================
-- 4. CONCURRENT STREAMING OPERATIONS
-- ========================================

-- name: LockHTTPForUpdate :one
-- Pessimistic locking for concurrent streaming updates
-- Uses http_lock_workspace_idx to prevent contention
SELECT 
  h.id,
  h.workspace_id,
  h.updated_at
FROM http h
WHERE h.id = ?
  AND h.workspace_id = ?
FOR UPDATE;  -- Row-level lock

-- name: GetHTTPWithConcurrencyControl :many
-- Concurrency-aware streaming query
-- Uses SKIP LOCKED for non-blocking reads in high contention
SELECT
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.description,
  h.updated_at
FROM http h
WHERE h.workspace_id = ?
  AND h.updated_at > ?
ORDER BY h.updated_at ASC
LIMIT ?
FOR UPDATE SKIP LOCKED;  -- Non-blocking concurrent access

-- ========================================
-- 5. DELTA RESOLUTION QUERIES
-- ========================================

-- name: ResolveHTTPWithDeltas :one
-- Resolve HTTP record with all applicable deltas
-- Critical for delta system performance in streaming
WITH RECURSIVE delta_chain AS (
  -- Base case: start with parent HTTP
  SELECT 
    h.id,
    h.name,
    h.url,
    h.method,
    h.description,
    h.updated_at,
    0 as level
  FROM http h
  WHERE h.id = ? AND h.is_delta = FALSE
  
  UNION ALL
  
  -- Recursive case: apply deltas in order
  SELECT 
    d.id,
    COALESCE(d.delta_name, dc.name) as name,
    COALESCE(d.delta_url, dc.url) as url,
    COALESCE(d.delta_method, dc.method) as method,
    COALESCE(d.delta_description, dc.description) as description,
    d.updated_at,
    dc.level + 1 as level
  FROM http d
  INNER JOIN delta_chain dc ON d.parent_http_id = dc.id
  WHERE d.is_delta = TRUE
    AND d.parent_http_id = dc.id
    AND d.updated_at <= ?  -- Point-in-time resolution
)
SELECT 
  id,
  name,
  url,
  method,
  description,
  updated_at
FROM delta_chain
ORDER BY level DESC
LIMIT 1;

-- name: GetHTTPDeltasBatch :many
-- Batch delta resolution for multiple HTTP records
-- Optimizes bulk delta operations in streaming
SELECT
  h.id as delta_id,
  h.parent_http_id,
  h.delta_name,
  h.delta_url,
  h.delta_method,
  h.delta_description,
  h.updated_at,
  ROW_NUMBER() OVER (PARTITION BY h.parent_http_id ORDER BY h.updated_at DESC) as delta_order
FROM http h
WHERE h.parent_http_id IN (sqlc.slice('parent_ids'))
  AND h.is_delta = TRUE
  AND h.updated_at > ?
ORDER BY h.parent_http_id, h.updated_at DESC;

-- ========================================
-- 6. PERFORMANCE MONITORING QUERIES
-- ========================================

-- name: GetStreamingStats :one
-- Real-time streaming performance statistics
-- Uses streaming cache for fast metrics
SELECT
  COUNT(*) as total_http,
  COUNT(CASE WHEN updated_at > ? THEN 1 END) as recent_updates,
  COUNT(CASE WHEN is_delta = TRUE THEN 1 END) as delta_count,
  MAX(updated_at) as last_update,
  AVG(CASE WHEN created_at > ? THEN 1 ELSE 0 END) as creation_rate
FROM http
WHERE workspace_id = ?;

-- name: LogStreamingPerformance :exec
-- Log streaming operation performance
INSERT INTO streaming_performance_log (
  id, workspace_id, operation_type, table_name, 
  record_count, duration_ms, timestamp, error_message
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetStreamingPerformanceMetrics :many
-- Analyze streaming performance over time
SELECT
  operation_type,
  table_name,
  AVG(duration_ms) as avg_duration,
  MAX(duration_ms) as max_duration,
  COUNT(*) as operation_count,
  SUM(record_count) as total_records,
  timestamp
FROM streaming_performance_log
WHERE workspace_id = ?
  AND timestamp > ?
GROUP BY operation_type, table_name, timestamp
ORDER BY timestamp DESC;

-- ========================================
-- 7. CACHE MANAGEMENT QUERIES
-- ========================================

-- name: RefreshHTTPStreamingCache :exec
-- Update streaming cache for performance
-- Uses UPSERT pattern for cache maintenance
INSERT INTO http_streaming_cache (
  http_id, workspace_id, folder_id, name, url, method,
  last_updated, delta_count, response_count, last_response_status, cache_updated_at
)
SELECT 
  h.id,
  h.workspace_id,
  h.folder_id,
  h.name,
  h.url,
  h.method,
  h.updated_at,
  (SELECT COUNT(*) FROM http d WHERE d.parent_http_id = h.id AND d.is_delta = TRUE) as delta_count,
  (SELECT COUNT(*) FROM http_response r WHERE r.http_id = h.id) as response_count,
  (SELECT r.status_code FROM http_response r WHERE r.http_id = h.id ORDER BY r.executed_at DESC LIMIT 1) as last_response_status,
  unixepoch()
FROM http h
WHERE h.workspace_id = ?
  AND h.updated_at > ?
ON CONFLICT(http_id) DO UPDATE SET
  workspace_id = excluded.workspace_id,
  folder_id = excluded.folder_id,
  name = excluded.name,
  url = excluded.url,
  method = excluded.method,
  last_updated = excluded.last_updated,
  delta_count = excluded.delta_count,
  response_count = excluded.response_count,
  last_response_status = excluded.last_response_status,
  cache_updated_at = excluded.cache_updated_at;

-- name: GetHTTPFromCache :many
-- Fast cache lookup for streaming queries
-- Uses http_streaming_cache_workspace_idx
SELECT
  http_id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  last_updated,
  delta_count,
  response_count,
  last_response_status
FROM http_streaming_cache
WHERE workspace_id = ?
  AND last_updated > ?
ORDER BY last_updated DESC
LIMIT ?;

-- ========================================
-- 8. FOREIGN KEY VALIDATION QUERIES
-- ========================================

-- name: ValidateHTTPForeignKeysForStreaming :many
-- Batch foreign key validation for streaming operations
-- Optimized for concurrent constraint checking
SELECT
  h.id,
  h.workspace_id,
  h.folder_id,
  h.parent_http_id,
  CASE 
    WHEN w.id IS NULL THEN 'invalid_workspace'
    WHEN h.folder_id IS NOT NULL AND f.id IS NULL THEN 'invalid_folder'
    WHEN h.parent_http_id IS NOT NULL AND p.id IS NULL THEN 'invalid_parent'
    ELSE 'valid'
  END as validation_status
FROM http h
LEFT JOIN workspaces w ON h.workspace_id = w.id
LEFT JOIN files f ON h.folder_id = f.id
LEFT JOIN http p ON h.parent_http_id = p.id
WHERE h.workspace_id = ?
  AND h.updated_at > ?
ORDER BY h.updated_at ASC;

-- name: GetHTTPChildDataForValidation :many
-- Validate child data integrity for streaming
-- Ensures referential integrity during high-frequency updates
SELECT
  h.id as http_id,
  COUNT(sp.id) as search_param_count,
  COUNT(hh.id) as header_count,
  COUNT(bf.id) as body_form_count,
  COUNT(bu.id) as body_urlencoded_count,
  COUNT(br.id) as body_raw_count,
  COUNT(ar.id) as assert_count,
  COUNT(rs.id) as response_count
FROM http h
LEFT JOIN http_search_param sp ON h.id = sp.http_id
LEFT JOIN http_header hh ON h.id = hh.http_id
LEFT JOIN http_body_form bf ON h.id = bf.http_id
LEFT JOIN http_body_urlencoded bu ON h.id = bu.http_id
LEFT JOIN http_body_raw br ON h.id = br.http_id
LEFT JOIN http_assert ar ON h.id = ar.http_id
LEFT JOIN http_response rs ON h.id = rs.http_id
WHERE h.workspace_id = ?
  AND h.updated_at > ?
GROUP BY h.id
ORDER BY h.updated_at ASC;