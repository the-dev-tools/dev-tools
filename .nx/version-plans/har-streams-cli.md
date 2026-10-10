---
cli: minor
---

`devtools import har` detects streaming responses. An entry whose response is text/event-stream becomes a streaming request, with its `stream:` preset (`openai`, `anthropic`, `vercel-ai` or `sse`) picked from the first recorded event, as Stresseur recordings do. Plain-text, base64 and missing bodies are handled (no body means `sse`), the import prints a per-preset summary, and exporting the workspace writes `stream: <preset>`.

`stream:`, `stream_timeout_ms`, `expect:` and the flow's `judge:`, `quality:` and `iterations:` are now stored with the workspace instead of being dropped on import, so they survive import → export. File-level settings are written back on each flow.

WebSocket flows no longer lose their connection: a `ws_connection` stayed open only until its step finished, so a later `ws_send` could fail with "use of closed network connection" and messages stopped being read. The connection now lasts for the whole flow, and its handshake has its own 30 s limit.
