---
cli: minor
---

AI checks are now part of the engine. A request or graphql step's `expect:` block (the Stresseur syntax, unchanged) is validated on import and evaluated by `devtools flow run` after the flow's steps finish, so it never counts in step timings. Results are printed under the flow's table and written to the JSON report as each step's `checks[]` (`kind`, `passed`, `score`, `reason`, `judge_model`, `cached`, `value`, `limit`).

- **Checks:** `schema`/`schema_at`, `max_latency_ms`, the new `max_ttft_ms`, token budgets from `usage`, and a G-Eval `judge` (Anthropic or any OpenAI-compatible API). The judge uses injection-resistant prompting, logprob weighting or borderline re-sampling, and runs in parallel.
- **Judge key:** `STRESSEUR_JUDGE_API_KEY`, `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`, from the environment or `./.env`. Without one, judge checks are skipped with a notice.
- **Cache:** verdicts are cached in `--judge-cache`, default `.devtools/judge-cache.json`.
- **Exit code:** failed checks fail a flow only with `quality: { fail_below: … }`.

Server-Sent Events responses are now read as they arrive. A text/event-stream response streams automatically; a request step's `stream: openai | anthropic | vercel-ai | sse` picks the preset that assembles the text, and `stream_timeout_ms` bounds an open stream (default 30 s). Assertions, `expect:` and later steps can read `response.text`, `response.events`, `response.event_count`, `response.ttft_ms` and the streamed `response.usage`.
