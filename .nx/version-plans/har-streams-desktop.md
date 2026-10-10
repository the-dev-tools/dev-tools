---
desktop: minor
---

Importing YAML or HAR keeps AI checks and streaming settings. A step's `expect:` block, the flow's `judge:`, `quality:` and `iterations:`, and a request's `stream:` and `stream_timeout_ms` are stored with the workspace, so exporting writes them back, and they follow copy/paste of nodes and duplicating requests or flows. HAR entries with text/event-stream responses get a `stream:` preset (`openai`, `anthropic`, `vercel-ai` or `sse`) detected from the recorded events. The app doesn't show or run these settings yet.
