--
-- Overlay (Delta) queries: URL-encoded
--
-- Order
-- name: DeltaUrlencOrderCount :one
SELECT COUNT(*) FROM delta_urlenc_order WHERE example_id = ?;

-- name: DeltaUrlencOrderSelectAsc :many
SELECT ref_kind, ref_id, rank, revision FROM delta_urlenc_order WHERE example_id = ? ORDER BY rank ASC;

-- name: DeltaUrlencOrderLastRank :one
SELECT rank FROM delta_urlenc_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1;

SELECT COALESCE(MAX(revision), 0) FROM delta_urlenc_order WHERE example_id = ?;

-- name: DeltaUrlencOrderInsertIgnore :exec
INSERT OR IGNORE INTO delta_urlenc_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?);

-- name: DeltaUrlencOrderUpsert :exec
INSERT INTO delta_urlenc_order(example_id, ref_kind, ref_id, rank, revision)
VALUES (?,?,?,?,?)
ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision;

-- name: DeltaUrlencOrderDeleteByRef :exec
DELETE FROM delta_urlenc_order WHERE example_id = ? AND ref_id = ?;

-- name: DeltaUrlencOrderExists :one
SELECT 1 FROM delta_urlenc_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?;

-- Delta rows
-- name: DeltaUrlencDeltaInsert :exec
INSERT INTO delta_urlenc_delta(example_id, id, body_key, value, description, enabled) VALUES (?,?,?,?,?, ?);

-- name: DeltaUrlencDeltaUpdate :exec
UPDATE delta_urlenc_delta SET body_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?;

-- name: DeltaUrlencDeltaExists :one
SELECT 1 FROM delta_urlenc_delta WHERE example_id = ? AND id = ?;

-- name: DeltaUrlencDeltaGet :one
SELECT body_key, value, description, enabled FROM delta_urlenc_delta WHERE example_id = ? AND id = ?;

-- name: DeltaUrlencDeltaDelete :exec
DELETE FROM delta_urlenc_delta WHERE example_id = ? AND id = ?;

-- State
-- name: DeltaUrlencStateUpsert :exec
INSERT INTO delta_urlenc_state(example_id, origin_id, suppressed, body_key, value, description, enabled, updated_at)
VALUES (?,?,?,?,?,?,?, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET
  suppressed=excluded.suppressed,
  body_key=COALESCE(excluded.body_key, delta_urlenc_state.body_key),
  value=COALESCE(excluded.value, delta_urlenc_state.value),
  description=COALESCE(excluded.description, delta_urlenc_state.description),
  enabled=COALESCE(excluded.enabled, delta_urlenc_state.enabled),
  updated_at=unixepoch();

-- name: DeltaUrlencStateGet :one
SELECT suppressed, body_key, value, description, enabled FROM delta_urlenc_state WHERE example_id = ? AND origin_id = ?;

-- name: DeltaUrlencStateClearOverrides :exec
UPDATE delta_urlenc_state SET body_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;

-- name: DeltaUrlencStateSuppress :exec
INSERT INTO delta_urlenc_state(example_id, origin_id, suppressed, updated_at)
VALUES (?,?, TRUE, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch();

-- name: DeltaUrlencStateUnsuppress :exec
UPDATE delta_urlenc_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;

-- name: DeltaUrlencResolveExampleByDeltaID :one
SELECT example_id FROM delta_urlenc_delta WHERE id = ? LIMIT 1;

-- name: DeltaUrlencResolveExampleByOrderRefID :one
SELECT example_id FROM delta_urlenc_order WHERE ref_id = ? LIMIT 1;

--
-- Overlay (Delta) queries: Headers
--
-- Order
-- name: DeltaHeaderOrderCount :one
SELECT COUNT(*) FROM delta_header_order WHERE example_id = ?;
-- name: DeltaHeaderOrderSelectAsc :many
SELECT ref_kind, ref_id, rank, revision FROM delta_header_order WHERE example_id = ? ORDER BY rank ASC;
-- name: DeltaHeaderOrderLastRank :one
SELECT rank FROM delta_header_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1;
SELECT COALESCE(MAX(revision), 0) FROM delta_header_order WHERE example_id = ?;
-- name: DeltaHeaderOrderInsertIgnore :exec
INSERT OR IGNORE INTO delta_header_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?);
-- name: DeltaHeaderOrderUpsert :exec
INSERT INTO delta_header_order(example_id, ref_kind, ref_id, rank, revision)
VALUES (?,?,?,?,?)
ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision;
-- name: DeltaHeaderOrderDeleteByRef :exec
DELETE FROM delta_header_order WHERE example_id = ? AND ref_id = ?;
-- name: DeltaHeaderOrderExists :one
SELECT 1 FROM delta_header_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?;

-- Delta rows
-- name: DeltaHeaderDeltaInsert :exec
INSERT INTO delta_header_delta(example_id, id, header_key, value, description, enabled) VALUES (?,?,?,?,?, ?);
-- name: DeltaHeaderDeltaUpdate :exec
UPDATE delta_header_delta SET header_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?;
-- name: DeltaHeaderDeltaGet :one
SELECT header_key, value, description, enabled FROM delta_header_delta WHERE example_id = ? AND id = ?;
-- name: DeltaHeaderDeltaExists :one
SELECT 1 FROM delta_header_delta WHERE example_id = ? AND id = ?;
-- name: DeltaHeaderDeltaDelete :exec
DELETE FROM delta_header_delta WHERE example_id = ? AND id = ?;

-- State
-- name: DeltaHeaderStateUpsert :exec
INSERT INTO delta_header_state(example_id, origin_id, suppressed, header_key, value, description, enabled, updated_at)
VALUES (?,?,?,?,?,?,?, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET
  suppressed=excluded.suppressed,
  header_key=COALESCE(excluded.header_key, delta_header_state.header_key),
  value=COALESCE(excluded.value, delta_header_state.value),
  description=COALESCE(excluded.description, delta_header_state.description),
  enabled=COALESCE(excluded.enabled, delta_header_state.enabled),
  updated_at=unixepoch();
-- name: DeltaHeaderStateGet :one
SELECT suppressed, header_key, value, description, enabled FROM delta_header_state WHERE example_id = ? AND origin_id = ?;
-- name: DeltaHeaderStateClearOverrides :exec
UPDATE delta_header_state SET header_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaHeaderStateSuppress :exec
INSERT INTO delta_header_state(example_id, origin_id, suppressed, updated_at)
VALUES (?,?, TRUE, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch();
-- name: DeltaHeaderStateUnsuppress :exec
UPDATE delta_header_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaHeaderResolveExampleByDeltaID :one
SELECT example_id FROM delta_header_delta WHERE id = ? LIMIT 1;
-- name: DeltaHeaderResolveExampleByOrderRefID :one
SELECT example_id FROM delta_header_order WHERE ref_id = ? LIMIT 1;

--
-- Overlay (Delta) queries: Queries
--
-- Order
-- name: DeltaQueryOrderCount :one
SELECT COUNT(*) FROM delta_query_order WHERE example_id = ?;
-- name: DeltaQueryOrderSelectAsc :many
SELECT ref_kind, ref_id, rank, revision FROM delta_query_order WHERE example_id = ? ORDER BY rank ASC;
-- name: DeltaQueryOrderLastRank :one
SELECT rank FROM delta_query_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1;
SELECT COALESCE(MAX(revision), 0) FROM delta_query_order WHERE example_id = ?;
-- name: DeltaQueryOrderInsertIgnore :exec
INSERT OR IGNORE INTO delta_query_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?);
-- name: DeltaQueryOrderUpsert :exec
INSERT INTO delta_query_order(example_id, ref_kind, ref_id, rank, revision)
VALUES (?,?,?,?,?)
ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision;
-- name: DeltaQueryOrderDeleteByRef :exec
DELETE FROM delta_query_order WHERE example_id = ? AND ref_id = ?;
-- name: DeltaQueryOrderExists :one
SELECT 1 FROM delta_query_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?;

-- Delta rows
-- name: DeltaQueryDeltaInsert :exec
INSERT INTO delta_query_delta(example_id, id, query_key, value, description, enabled) VALUES (?,?,?,?,?, ?);
-- name: DeltaQueryDeltaUpdate :exec
UPDATE delta_query_delta SET query_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?;
-- name: DeltaQueryDeltaGet :one
SELECT query_key, value, description, enabled FROM delta_query_delta WHERE example_id = ? AND id = ?;
-- name: DeltaQueryDeltaExists :one
SELECT 1 FROM delta_query_delta WHERE example_id = ? AND id = ?;
-- name: DeltaQueryDeltaDelete :exec
DELETE FROM delta_query_delta WHERE example_id = ? AND id = ?;

-- State
-- name: DeltaQueryStateUpsert :exec
INSERT INTO delta_query_state(example_id, origin_id, suppressed, query_key, value, description, enabled, updated_at)
VALUES (?,?,?,?,?,?,?, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET
  suppressed=excluded.suppressed,
  query_key=COALESCE(excluded.query_key, delta_query_state.query_key),
  value=COALESCE(excluded.value, delta_query_state.value),
  description=COALESCE(excluded.description, delta_query_state.description),
  enabled=COALESCE(excluded.enabled, delta_query_state.enabled),
  updated_at=unixepoch();
-- name: DeltaQueryStateGet :one
SELECT suppressed, query_key, value, description, enabled FROM delta_query_state WHERE example_id = ? AND origin_id = ?;
-- name: DeltaQueryStateClearOverrides :exec
UPDATE delta_query_state SET query_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaQueryStateSuppress :exec
INSERT INTO delta_query_state(example_id, origin_id, suppressed, updated_at)
VALUES (?,?, TRUE, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch();
-- name: DeltaQueryStateUnsuppress :exec
UPDATE delta_query_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaQueryResolveExampleByDeltaID :one
SELECT example_id FROM delta_query_delta WHERE id = ? LIMIT 1;
-- name: DeltaQueryResolveExampleByOrderRefID :one
SELECT example_id FROM delta_query_order WHERE ref_id = ? LIMIT 1;

--
-- Overlay (Delta) queries: Forms
--
-- Order
-- name: DeltaFormOrderCount :one
SELECT COUNT(*) FROM delta_form_order WHERE example_id = ?;
-- name: DeltaFormOrderSelectAsc :many
SELECT ref_kind, ref_id, rank, revision FROM delta_form_order WHERE example_id = ? ORDER BY rank ASC;
-- name: DeltaFormOrderLastRank :one
SELECT rank FROM delta_form_order WHERE example_id = ? ORDER BY rank DESC LIMIT 1;
SELECT COALESCE(MAX(revision), 0) FROM delta_form_order WHERE example_id = ?;
-- name: DeltaFormOrderInsertIgnore :exec
INSERT OR IGNORE INTO delta_form_order(example_id, ref_kind, ref_id, rank, revision) VALUES (?,?,?,?,?);
-- name: DeltaFormOrderUpsert :exec
INSERT INTO delta_form_order(example_id, ref_kind, ref_id, rank, revision)
VALUES (?,?,?,?,?)
ON CONFLICT(example_id, ref_kind, ref_id) DO UPDATE SET rank=excluded.rank, revision=excluded.revision;
-- name: DeltaFormOrderDeleteByRef :exec
DELETE FROM delta_form_order WHERE example_id = ? AND ref_id = ?;
-- name: DeltaFormOrderExists :one
SELECT 1 FROM delta_form_order WHERE example_id = ? AND ref_kind = ? AND ref_id = ?;

-- Delta rows
-- name: DeltaFormDeltaInsert :exec
INSERT INTO delta_form_delta(example_id, id, body_key, value, description, enabled) VALUES (?,?,?,?,?, ?);
-- name: DeltaFormDeltaUpdate :exec
UPDATE delta_form_delta SET body_key = ?, value = ?, description = ?, enabled = ?, updated_at = unixepoch() WHERE example_id = ? AND id = ?;
-- name: DeltaFormDeltaGet :one
SELECT body_key, value, description, enabled FROM delta_form_delta WHERE example_id = ? AND id = ?;
-- name: DeltaFormDeltaExists :one
SELECT 1 FROM delta_form_delta WHERE example_id = ? AND id = ?;
-- name: DeltaFormDeltaDelete :exec
DELETE FROM delta_form_delta WHERE example_id = ? AND id = ?;

-- State
-- name: DeltaFormStateUpsert :exec
INSERT INTO delta_form_state(example_id, origin_id, suppressed, body_key, value, description, enabled, updated_at)
VALUES (?,?,?,?,?,?,?, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET
  suppressed=excluded.suppressed,
  body_key=COALESCE(excluded.body_key, delta_form_state.body_key),
  value=COALESCE(excluded.value, delta_form_state.value),
  description=COALESCE(excluded.description, delta_form_state.description),
  enabled=COALESCE(excluded.enabled, delta_form_state.enabled),
  updated_at=unixepoch();
-- name: DeltaFormStateGet :one
SELECT suppressed, body_key, value, description, enabled FROM delta_form_state WHERE example_id = ? AND origin_id = ?;
-- name: DeltaFormStateClearOverrides :exec
UPDATE delta_form_state SET body_key=NULL, value=NULL, description=NULL, enabled=NULL, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaFormStateSuppress :exec
INSERT INTO delta_form_state(example_id, origin_id, suppressed, updated_at)
VALUES (?,?, TRUE, unixepoch())
ON CONFLICT(example_id, origin_id) DO UPDATE SET suppressed=TRUE, updated_at=unixepoch();
-- name: DeltaFormStateUnsuppress :exec
UPDATE delta_form_state SET suppressed = FALSE, updated_at = unixepoch() WHERE example_id = ? AND origin_id = ?;
-- name: DeltaFormResolveExampleByDeltaID :one
SELECT example_id FROM delta_form_delta WHERE id = ? LIMIT 1;
-- name: DeltaFormResolveExampleByOrderRefID :one
SELECT example_id FROM delta_form_order WHERE ref_id = ? LIMIT 1;
