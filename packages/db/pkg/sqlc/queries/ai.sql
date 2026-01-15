-- name: GetCredential :one
SELECT
  id,
  workspace_id,
  name,
  kind
FROM
  credential
WHERE
  id = ?
LIMIT 1;

-- name: GetCredentialsByWorkspaceID :many
SELECT
  id,
  workspace_id,
  name,
  kind
FROM
  credential
WHERE
  workspace_id = ?;

-- name: CreateCredential :exec
INSERT INTO
  credential (id, workspace_id, name, kind)
VALUES
  (?, ?, ?, ?);

-- name: UpdateCredential :exec
UPDATE credential
SET
  name = ?,
  kind = ?
WHERE
  id = ?;

-- name: DeleteCredential :exec
DELETE FROM credential
WHERE
  id = ?;

-- name: GetCredentialOpenAI :one
SELECT
  credential_id,
  token,
  base_url,
  encryption_type
FROM
  credential_openai
WHERE
  credential_id = ?
LIMIT 1;

-- name: CreateCredentialOpenAI :exec
INSERT INTO
  credential_openai (credential_id, token, base_url, encryption_type)
VALUES
  (?, ?, ?, ?);

-- name: UpdateCredentialOpenAI :exec
UPDATE credential_openai
SET
  token = ?,
  base_url = ?,
  encryption_type = ?
WHERE
  credential_id = ?;

-- name: DeleteCredentialOpenAI :exec
DELETE FROM credential_openai
WHERE
  credential_id = ?;

-- name: GetCredentialGemini :one
SELECT
  credential_id,
  api_key,
  base_url,
  encryption_type
FROM
  credential_gemini
WHERE
  credential_id = ?
LIMIT 1;

-- name: CreateCredentialGemini :exec
INSERT INTO
  credential_gemini (credential_id, api_key, base_url, encryption_type)
VALUES
  (?, ?, ?, ?);

-- name: UpdateCredentialGemini :exec
UPDATE credential_gemini
SET
  api_key = ?,
  base_url = ?,
  encryption_type = ?
WHERE
  credential_id = ?;

-- name: DeleteCredentialGemini :exec
DELETE FROM credential_gemini
WHERE
  credential_id = ?;

-- name: GetCredentialAnthropic :one
SELECT
  credential_id,
  api_key,
  base_url,
  encryption_type
FROM
  credential_anthropic
WHERE
  credential_id = ?
LIMIT 1;

-- name: CreateCredentialAnthropic :exec
INSERT INTO
  credential_anthropic (credential_id, api_key, base_url, encryption_type)
VALUES
  (?, ?, ?, ?);

-- name: UpdateCredentialAnthropic :exec
UPDATE credential_anthropic
SET
  api_key = ?,
  base_url = ?,
  encryption_type = ?
WHERE
  credential_id = ?;

-- name: DeleteCredentialAnthropic :exec
DELETE FROM credential_anthropic
WHERE
  credential_id = ?;

-- name: GetFlowNodeAI :one
SELECT
  flow_node_id,
  prompt,
  max_iterations
FROM
  flow_node_ai
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeAI :exec
INSERT INTO
  flow_node_ai (flow_node_id, prompt, max_iterations)
VALUES
  (?, ?, ?);

-- name: UpdateFlowNodeAI :exec
UPDATE flow_node_ai
SET
  prompt = ?,
  max_iterations = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeAI :exec
DELETE FROM flow_node_ai
WHERE
  flow_node_id = ?;

-- NodeModel (Model Node) queries
-- name: GetFlowNodeModel :one
SELECT
  flow_node_id,
  credential_id,
  model,
  temperature,
  max_tokens
FROM
  flow_node_model
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeModel :exec
INSERT INTO
  flow_node_model (flow_node_id, credential_id, model, temperature, max_tokens)
VALUES
  (?, ?, ?, ?, ?);

-- name: UpdateFlowNodeModel :exec
UPDATE flow_node_model
SET
  credential_id = ?,
  model = ?,
  temperature = ?,
  max_tokens = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeModel :exec
DELETE FROM flow_node_model
WHERE
  flow_node_id = ?;

-- NodeMemory (Memory Node) queries
-- name: GetFlowNodeMemory :one
SELECT
  flow_node_id,
  memory_type,
  window_size
FROM
  flow_node_memory
WHERE
  flow_node_id = ?
LIMIT 1;

-- name: CreateFlowNodeMemory :exec
INSERT INTO
  flow_node_memory (flow_node_id, memory_type, window_size)
VALUES
  (?, ?, ?);

-- name: UpdateFlowNodeMemory :exec
UPDATE flow_node_memory
SET
  memory_type = ?,
  window_size = ?
WHERE
  flow_node_id = ?;

-- name: DeleteFlowNodeMemory :exec
DELETE FROM flow_node_memory
WHERE
  flow_node_id = ?;
