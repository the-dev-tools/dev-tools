---
cli: patch
---

AI checks: with no judge provider configured, the judge uses OpenAI when `OPENAI_API_KEY` is the only key set (a configured `provider`, `model`, `base_url` or `api_key_env` still decides), and the no-key notice names both keys. A failing token check reports the budget that failed, not the first one, and `ci` summaries print `>` instead of `≤` when the p95 latency or max tokens is over its limit.
