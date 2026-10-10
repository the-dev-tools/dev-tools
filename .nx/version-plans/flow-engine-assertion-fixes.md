---
cli: patch
---

Fix four flow-engine bugs that could let a run pass when it should fail, or slow it down:

- `response.duration` in `assertions:` is now the measured request time in ms. It was always 0, so `response.duration < N` never failed.
- A failed assertion now shows each value it checked, e.g. `(got response.duration = 5120)` or `(got response.body.name = "Outdoor")`, instead of only the status code. Values under names like `token` or `password` are redacted. When a body field is compared on a non-JSON response, such as a 404 HTML page, the error now names the status and content type instead of only `invalid operation: int(string)`.
- JS steps no longer add a fixed ~3 s per run. The worker is polled every 25 ms and is ready in about 0.1 s.
- Import now rejects a step that reads another step's output, such as `{{ Ask.response.body.answer }}`, without depending on that step. The error names the step and the missing `depends_on`. Before, such a step ran at flow start with the template unfilled, and the flow still passed.
