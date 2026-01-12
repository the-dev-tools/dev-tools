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
  token TEXT NOT NULL,
  base_url TEXT,
  FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
);

CREATE TABLE credential_gemini (
  credential_id BLOB NOT NULL PRIMARY KEY,
  api_key TEXT NOT NULL,
  base_url TEXT,
  FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
);

CREATE TABLE credential_anthropic (
  credential_id BLOB NOT NULL PRIMARY KEY,
  api_key TEXT NOT NULL,
  base_url TEXT,
  FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
);

CREATE TABLE flow_node_ai (
  flow_node_id BLOB NOT NULL PRIMARY KEY,
  model INT8 NOT NULL, -- AiModel enum: 0=Gpt52Instant, 1=Gpt52Thinking, 2=Gpt52Pro, 3=Gpt52Codex, 4=O3, 5=O4Mini, 6=ClaudeOpus45, 7=ClaudeSonnet45, 8=ClaudeHaiku45, 9=Gemini3Pro, 10=Gemini3Flash
  credential_id BLOB NOT NULL,
  prompt TEXT NOT NULL,
  FOREIGN KEY (flow_node_id) REFERENCES flow_node (id) ON DELETE CASCADE,
  FOREIGN KEY (credential_id) REFERENCES credential (id) ON DELETE CASCADE
);
