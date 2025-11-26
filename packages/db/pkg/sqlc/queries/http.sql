--
-- HTTP Core Queries
--

-- name: GetHTTP :one

SELECT

  id,

  workspace_id,

  folder_id,

  name,

  url,

  method,

  body_kind,

  description,

  parent_http_id,

  is_delta,

  delta_name,

  delta_url,

  delta_method,

  delta_body_kind,

  delta_description,

  last_run_at,

  created_at,

  updated_at

FROM http

WHERE id = ? LIMIT 1;

-- name: GetHTTPsByWorkspaceID :many
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  body_kind,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_body_kind,
  delta_description,
  last_run_at,
  created_at,
  updated_at
FROM http
WHERE workspace_id = ? AND is_delta = FALSE
ORDER BY updated_at DESC;

-- name: GetHTTPsByFolderID :many
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  body_kind,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_body_kind,
  delta_description,
  created_at,
  updated_at
FROM http
WHERE folder_id = ? AND is_delta = FALSE
ORDER BY updated_at DESC;

-- name: GetHTTPsByIDs :many
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  body_kind,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_body_kind,
  delta_description,
  created_at,
  updated_at
FROM http
WHERE id IN (sqlc.slice('ids'));

-- name: GetHTTPWorkspaceID :one
SELECT workspace_id
FROM http
WHERE id = ?
LIMIT 1;

-- name: CreateHTTP :exec
INSERT INTO http (
  id, workspace_id, folder_id, name, url, method, body_kind, description,
  parent_http_id, is_delta, delta_name, delta_url, delta_method, delta_body_kind, delta_description,
  last_run_at, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTP :exec
UPDATE http
SET
  folder_id = ?,
  name = ?,
  url = ?,
  method = ?,
  body_kind = ?,
  description = ?,
  last_run_at = COALESCE(?, last_run_at),
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPDelta :exec
UPDATE http
SET
  delta_name = ?,
  delta_url = ?,
  delta_method = ?,
  delta_body_kind = ?,
  delta_description = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteHTTP :exec
DELETE FROM http
WHERE id = ?;

-- name: GetHTTPDeltasByParentID :many
SELECT
  id,
  workspace_id,
  folder_id,
  name,
  url,
  method,
  body_kind,
  description,
  parent_http_id,
  is_delta,
  delta_name,
  delta_url,
  delta_method,
  delta_body_kind,
  delta_description,
  created_at,
  updated_at
FROM http
WHERE parent_http_id = ? AND is_delta = TRUE
ORDER BY created_at DESC;

--
-- HTTP Search Parameter Queries
--

-- name: GetHTTPSearchParams :many
SELECT
  id,
  http_id,
  key,
  value,
  description,
  enabled,
  parent_http_search_param_id,
  is_delta,
  delta_key,
  delta_value,
  delta_description,
  delta_enabled,
  "order",
  created_at,
  updated_at
FROM http_search_param
WHERE http_id = ? AND is_delta = FALSE
ORDER BY "order";

-- name: GetHTTPSearchParamsByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  description,
  enabled,
  parent_http_search_param_id,
  is_delta,
  delta_key,
  delta_value,
  delta_description,
  delta_enabled,
  "order",
  created_at,
  updated_at
FROM http_search_param
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPSearchParam :exec
INSERT INTO http_search_param (
  id, http_id, key, value, description, enabled, "order",
  parent_http_search_param_id, is_delta, delta_key, delta_value,
  delta_description, delta_enabled, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPSearchParam :exec
UPDATE http_search_param
SET
  key = ?,
  value = ?,
  description = ?,
  enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPSearchParamDelta :exec
UPDATE http_search_param
SET
  delta_key = ?,
  delta_value = ?,
  delta_description = ?,
  delta_enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPSearchParamOrder :exec
UPDATE http_search_param
SET "order" = ?
WHERE id = ? AND http_id = ?;

-- name: DeleteHTTPSearchParam :exec
DELETE FROM http_search_param
WHERE id = ?;

--
-- HTTP Header Queries
--

-- name: GetHTTPHeaders :many
SELECT
  id,
  http_id,
  header_key,
  header_value,
  description,
  enabled,
  parent_header_id,
  is_delta,
  delta_header_key,
  delta_header_value,
  delta_description,
  delta_enabled,
  prev,
  next,
  created_at,
  updated_at
FROM http_header
WHERE http_id = ? AND is_delta = FALSE
ORDER BY created_at ASC;

-- name: GetHTTPHeadersByIDs :many
SELECT
  id,
  http_id,
  header_key,
  header_value,
  description,
  enabled,
  parent_header_id,
  is_delta,
  delta_header_key,
  delta_header_value,
  delta_description,
  delta_enabled,
  prev,
  next,
  created_at,
  updated_at
FROM http_header
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPHeader :exec
INSERT INTO http_header (
  id, http_id, header_key, header_value, description, enabled,
  parent_header_id, is_delta, delta_header_key, delta_header_value,
  delta_description, delta_enabled, prev, next, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPHeader :exec
UPDATE http_header
SET
  header_key = ?,
  header_value = ?,
  description = ?,
  enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPHeaderDelta :exec
UPDATE http_header
SET
  delta_header_key = ?,
  delta_header_value = ?,
  delta_description = ?,
  delta_enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPHeaderOrder :exec
UPDATE http_header
SET prev = ?, next = ?
WHERE id = ? AND http_id = ?;

-- name: DeleteHTTPHeader :exec
DELETE FROM http_header
WHERE id = ?;

--
-- HTTP Body Form Queries
--

-- name: GetHTTPBodyForms :many
SELECT
  id,
  http_id,
  key,
  value,
  description,
  enabled,
  parent_http_body_form_id,
  is_delta,
  delta_key,
  delta_value,
  delta_description,
  delta_enabled,
  "order",
  created_at,
  updated_at
FROM http_body_form
WHERE http_id = ? AND is_delta = FALSE
ORDER BY "order";

-- name: GetHTTPBodyFormsByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  description,
  enabled,
  parent_http_body_form_id,
  is_delta,
  delta_key,
  delta_value,
  delta_description,
  delta_enabled,
  "order",
  created_at,
  updated_at
FROM http_body_form
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPBodyForm :exec
INSERT INTO http_body_form (
  id, http_id, key, value, description, enabled, "order",
  parent_http_body_form_id, is_delta, delta_key, delta_value,
  delta_description, delta_enabled, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPBodyForm :exec
UPDATE http_body_form
SET
  key = ?,
  value = ?,
  description = ?,
  enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPBodyFormDelta :exec
UPDATE http_body_form
SET
  delta_key = ?,
  delta_value = ?,
  delta_description = ?,
  delta_enabled = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: ResetHTTPBodyFormDelta :exec
UPDATE http_body_form
SET
  is_delta = false,
  parent_http_body_form_id = NULL,
  delta_key = NULL,
  delta_value = NULL,
  delta_description = NULL,
  delta_enabled = NULL,
  delta_order = NULL,
  updated_at = unixepoch()
WHERE id = ?;

-- name: UpdateHTTPBodyFormOrder :exec
UPDATE http_body_form
SET "order" = ?
WHERE id = ? AND http_id = ?;

-- name: DeleteHTTPBodyForm :exec
DELETE FROM http_body_form WHERE id = ?;

--
-- HTTP Body URL-Encoded Queries (TypeSpec-compliant)
--

-- name: GetHTTPBodyUrlEncoded :one
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_body_urlencoded_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_body_urlencoded
WHERE id = ?
LIMIT 1;

-- name: GetHTTPBodyUrlEncodedByHttpID :many
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_body_urlencoded_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_body_urlencoded
WHERE http_id = ? AND is_delta = FALSE
ORDER BY "order";

-- name: GetHTTPBodyUrlEncodedsByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_body_urlencoded_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_body_urlencoded
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPBodyUrlEncoded :exec
INSERT INTO http_body_urlencoded (
  id, http_id, key, value, enabled, description, "order",
  parent_http_body_urlencoded_id, is_delta, delta_key, delta_value,
  delta_enabled, delta_description, delta_order, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateHTTPBodyUrlEncodedBulk :exec
INSERT INTO http_body_urlencoded (
  id, http_id, key, value, enabled, description, "order",
  parent_http_body_urlencoded_id, is_delta, delta_key, delta_value,
  delta_enabled, delta_description, delta_order, created_at, updated_at
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPBodyUrlEncoded :exec
UPDATE http_body_urlencoded
SET
  key = ?,
  value = ?,
  enabled = ?,
  description = ?,
  "order" = ?,
  delta_key = ?,
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_order = ?,
  updated_at = ?
WHERE id = ?;

-- name: UpdateHTTPBodyUrlEncodedDelta :exec
UPDATE http_body_urlencoded
SET
  delta_key = ?,
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_order = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteHTTPBodyUrlEncoded :exec
DELETE FROM http_body_urlencoded WHERE id = ?;

--
-- HTTP Assert Queries (TypeSpec-compliant)
--

-- name: GetHTTPAssert :one
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_assert_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_assert
WHERE id = ?
LIMIT 1;

-- name: GetHTTPAssertsByHttpID :many
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_assert_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_assert
WHERE http_id = ? AND is_delta = FALSE
ORDER BY "order";

-- name: GetHTTPAssertsByIDs :many
SELECT
  id,
  http_id,
  key,
  value,
  enabled,
  description,
  "order",
  parent_http_assert_id,
  is_delta,
  delta_key,
  delta_value,
  delta_enabled,
  delta_description,
  delta_order,
  created_at,
  updated_at
FROM http_assert
WHERE id IN (sqlc.slice('ids'));

-- name: CreateHTTPAssert :exec
INSERT INTO http_assert (
  id, http_id, key, value, enabled, description, "order",
  parent_http_assert_id, is_delta, delta_key, delta_value,
  delta_enabled, delta_description, delta_order, created_at, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateHTTPAssertBulk :exec
INSERT INTO http_assert (
  id, http_id, key, value, enabled, description, "order",
  parent_http_assert_id, is_delta, delta_key, delta_value,
  delta_enabled, delta_description, delta_order, created_at, updated_at
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPAssert :exec
UPDATE http_assert
SET
  key = ?,
  value = ?,
  enabled = ?,
  description = ?,
  "order" = ?,
  delta_key = ?,
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_order = ?,
  updated_at = ?
WHERE id = ?;

-- name: UpdateHTTPAssertDelta :exec
UPDATE http_assert
SET
  delta_key = ?,
  delta_value = ?,
  delta_enabled = ?,
  delta_description = ?,
  delta_order = ?,
  updated_at = unixepoch()
WHERE id = ?;

-- name: DeleteHTTPAssert :exec
DELETE FROM http_assert WHERE id = ?;

--
-- HTTP Response Queries (TypeSpec-compliant)
--
-- HTTP Response Queries (TypeSpec-compliant)
--

-- name: GetHTTPResponse :one
SELECT
  id,
  http_id,
  status,
  body,
  time,
  duration,
  size,
  created_at
FROM http_response
WHERE id = ?
LIMIT 1;

-- name: GetHTTPResponsesByHttpID :many
SELECT
  id,
  http_id,
  status,
  body,
  time,
  duration,
  size,
  created_at
FROM http_response
WHERE http_id = ?
ORDER BY time DESC;

-- name: GetHTTPResponsesByIDs :many
SELECT
  id,
  http_id,
  status,
  body,
  time,
  duration,
  size,
  created_at
FROM http_response
WHERE id IN (sqlc.slice('ids'))
ORDER BY time DESC;

-- name: CreateHTTPResponse :exec
INSERT INTO http_response (
  id, http_id, status, body, time, duration, size, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateHTTPResponseBulk :exec
INSERT INTO http_response (
  id, http_id, status, body, time, duration, size, created_at
)
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPResponse :exec
UPDATE http_response
SET
  status = ?,
  body = ?,
  time = ?,
  duration = ?,
  size = ?
WHERE id = ?;

-- name: DeleteHTTPResponse :exec
DELETE FROM http_response WHERE id = ?;

--
-- HTTP Response Header Queries (TypeSpec-compliant)
--

-- name: GetHTTPResponseHeader :one
SELECT
  id,
  response_id,
  key,
  value,
  created_at
FROM http_response_header
WHERE id = ?
LIMIT 1;

-- name: GetHTTPResponseHeadersByResponseID :many
SELECT
  id,
  response_id,
  key,
  value,
  created_at
FROM http_response_header
WHERE response_id = ?
ORDER BY key;

-- name: GetHTTPResponseHeadersByIDs :many
SELECT
  id,
  response_id,
  key,
  value,
  created_at
FROM http_response_header
WHERE id IN (sqlc.slice('ids'))
ORDER BY response_id, key;

-- name: CreateHTTPResponseHeader :exec
INSERT INTO http_response_header (
  id, response_id, key, value, created_at
)
VALUES (?, ?, ?, ?, ?);

-- name: CreateHTTPResponseHeaderBulk :exec
INSERT INTO http_response_header (
  id, response_id, key, value, created_at
)
VALUES
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?);

-- name: UpdateHTTPResponseHeader :exec
UPDATE http_response_header
SET
  key = ?,
  value = ?
WHERE id = ?;

-- name: DeleteHTTPResponseHeader :exec
DELETE FROM http_response_header WHERE id = ?;

--
-- HTTP Response Assert Queries (TypeSpec-compliant)
--

-- name: GetHTTPResponseAssert :one
SELECT
  id,
  response_id,
  value,
  success,
  created_at
FROM http_response_assert
WHERE id = ?
LIMIT 1;

-- name: GetHTTPResponseAssertsByResponseID :many
SELECT
  id,
  response_id,
  value,
  success,
  created_at
FROM http_response_assert
WHERE response_id = ?
ORDER BY created_at DESC;

-- name: GetHTTPResponseAssertsByIDs :many
SELECT
  id,
  response_id,
  value,
  success,
  created_at
FROM http_response_assert
WHERE id IN (sqlc.slice('ids'))
ORDER BY created_at DESC;

-- name: CreateHTTPResponseAssert :exec
INSERT INTO http_response_assert (
  id, response_id, value, success, created_at
)
VALUES (?, ?, ?, ?, ?);

-- name: CreateHTTPResponseAssertBulk :exec
INSERT INTO http_response_assert (
  id, response_id, value, success, created_at
)
VALUES
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?),
  (?, ?, ?, ?, ?);

-- name: UpdateHTTPResponseAssert :exec
UPDATE http_response_assert
SET
  value = ?,
  success = ?
WHERE id = ?;

-- name: DeleteHTTPResponseAssert :exec
DELETE FROM http_response_assert WHERE id = ?;

-- HTTP Body Raw queries
-- name: GetHTTPBodyRawByID :one
SELECT
  id,
  http_id,
  raw_data,
  content_type,
  compression_type,
  parent_body_raw_id,
  is_delta,
  delta_raw_data,
  delta_content_type,
  delta_compression_type,
  created_at,
  updated_at
FROM
  http_body_raw
WHERE
  id = ?
LIMIT 1;

-- name: GetHTTPBodyRaw :one
SELECT
  id,
  http_id,
  raw_data,
  content_type,
  compression_type,
  parent_body_raw_id,
  is_delta,
  delta_raw_data,
  delta_content_type,
  delta_compression_type,
  created_at,
  updated_at
FROM
  http_body_raw
WHERE
  http_id = ?
LIMIT 1;

-- name: CreateHTTPBodyRaw :exec
INSERT INTO
  http_body_raw (
    id,
    http_id,
    raw_data,
    content_type,
    compression_type,
    parent_body_raw_id,
    is_delta,
    delta_raw_data,
    delta_content_type,
    delta_compression_type,
    created_at,
    updated_at
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHTTPBodyRaw :exec
UPDATE http_body_raw
SET
  raw_data = ?,
  content_type = ?,
  compression_type = ?,
  updated_at = ?
WHERE
  id = ?;

-- name: DeleteHTTPBodyRaw :exec
DELETE FROM http_body_raw
WHERE
  id = ?;
-- name: CreateHttpVersion :exec
INSERT INTO http_version (
  id, http_id, version_name, version_description, is_active, created_at, created_by
)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetHttpVersionsByHttpID :many
SELECT id, http_id, version_name, version_description, is_active, created_at, created_by
FROM http_version
WHERE http_id = ?
ORDER BY created_at DESC;
-- name: GetHTTPResponseHeadersByHttpID :many
SELECT hrh.id, hrh.response_id, hrh.key, hrh.value, hrh.created_at
FROM http_response_header hrh
JOIN http_response hr ON hrh.response_id = hr.id
WHERE hr.http_id = ?
ORDER BY hrh.created_at DESC;

-- name: GetHTTPResponseAssertsByHttpID :many
SELECT hra.id, hra.response_id, hra.value, hra.success, hra.created_at
FROM http_response_assert hra
JOIN http_response hr ON hra.response_id = hr.id
WHERE hr.http_id = ?
ORDER BY hra.created_at DESC;
