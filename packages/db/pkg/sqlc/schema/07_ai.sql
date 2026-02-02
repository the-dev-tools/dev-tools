/*
 *
 * AI & CREDENTIALS
 *
 */

CREATE TABLE credential (
  id BLOB NOT NULL PRIMARY KEY,
  workspace_id BLOB NOT NULL,
  name TEXT NOT NULL,
  kind INT8 NOT NULL, -- 0 = OpenAI, 1 = Gemini, 2 = Anthropic
  FOREIGN KEY (workspace_id) REFERENCES workspaces (id) ON DELETE CASCADE
);

CREATE INDEX credential_workspace_idx ON credential (workspace_id);

CREATE TABLE credential_openai (
  credential_id BLOB NOT NULL PRIMARY KEY,
  token BLOB NOT NULL, -- Encrypted or plaintext depending on encryption_type
  base_url TEXT,
  encryption_type INT8 NOT NULL DEFAULT 0, -- 0=None, 1=XChaCha20-Poly1305, 2=AES-256-GCM
  FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
);

CREATE TABLE credential_gemini (
  credential_id BLOB NOT NULL PRIMARY KEY,
  api_key BLOB NOT NULL, -- Encrypted or plaintext depending on encryption_type
  base_url TEXT,
  encryption_type INT8 NOT NULL DEFAULT 0, -- 0=None, 1=XChaCha20-Poly1305, 2=AES-256-GCM
  FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
);

CREATE TABLE credential_anthropic (
  credential_id BLOB NOT NULL PRIMARY KEY,
  api_key BLOB NOT NULL, -- Encrypted or plaintext depending on encryption_type
  base_url TEXT,
  encryption_type INT8 NOT NULL DEFAULT 0, -- 0=None, 1=XChaCha20-Poly1305, 2=AES-256-GCM
  FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_ai (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  prompt TEXT NOT NULL,
  max_iterations INT NOT NULL DEFAULT 5
);

-- AI Provider Node: LLM configuration that can be connected to AI Agent nodes via HandleAiProvider edge
CREATE TABLE flow_node_ai_provider (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  credential_id BLOB, -- Optional: NULL means no credential set yet
  model INT8 NOT NULL, -- AiModel enum
  temperature REAL, -- Optional: 0.0-2.0, NULL means use provider default
  max_tokens INT -- Optional: max output tokens, NULL means use provider default
);

-- Memory Node: Conversation memory configuration that can be connected to AI Agent nodes via HandleAiMemory edge
CREATE TABLE flow_node_memory (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  memory_type INT8 NOT NULL, -- AiMemoryType enum: 0 = WindowBuffer
  window_size INT NOT NULL -- For WindowBuffer: number of messages to retain
);
