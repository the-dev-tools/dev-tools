---
cli: minor
---

Onboarding fixes for flows run against local apps:

- `Secure` cookies are now sent to `http://localhost`, `127.0.0.1`, `::1` and `*.localhost`, as browsers do, so a local dev server's Secure session cookie keeps the session alive. Other `http://` hosts still never get them.
- `follow_redirects: false` on a request step returns the 30x as is, so the redirect itself can be asserted. Every response now has `response.url` (the final URL after redirects) and `response.redirects`, e.g. `'!(response.url contains "/login")'`. The option survives YAML export; the desktop app doesn't store it yet.
- `{{ now() }}` now writes an RFC 3339 timestamp (`2026-10-11T15:59:15.978+03:00`) instead of Go's internal time format. `uuid()`, `ulid()`, `now().Unix()` and `faker.*` are documented in the YAML README.
- `flow run` reads `./.env` and the flow file's directory's `.env` for `{{ #env:NAME }}` (the process environment wins), and before running lists every missing `#env:` name in one error instead of failing at the first step that reads one.
