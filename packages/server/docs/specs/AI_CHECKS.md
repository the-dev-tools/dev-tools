# AI Checks and SSE Streaming

Status: approved by the owner on 2026-10-10. Engine side of Stresseur's "AI checks". The YAML
syntax is the Stresseur CLI spec's (`stresseur-recorder--one-cli/docs/specs/2026-10-10-ai-checks-design.md`),
unchanged, so existing flow files keep working. Background: the research brief
`ai-output-testing.md` (§5 G-Eval, §6 YAML, §8 Phase 1 and Phase 1b).

## Goal

`expect:` used to be read only by the Stresseur CLI; the engine ignored it. After this change:

- The engine parses, validates and evaluates `expect:`.
- `devtools flow run` prints the results.
- The JSON report carries them per step.

The Stresseur CLI reads the results instead of evaluating them itself. Request steps can also
read Server-Sent Events streams incrementally, with time to first token.

## YAML

The keys are the CLI spec's, unchanged:

- **File level:** `judge:` and `quality:`.
- **Flow level:** `judge:` (overrides the file's key by key), `quality:` and `iterations:`.
- **Request and graphql steps:** `expect:` with `schema` (inline, or a path relative to the flow
  file), `schema_at`, `max_latency_ms`, `usage`, `max_input_tokens`, `max_output_tokens`,
  `max_total_tokens` and `judge` (`criteria`, `steps`, `input`, `output`, `min_score`).

Validation is strict. Any of these stops the import with an error naming the flow and step:

- an unknown key inside `expect:`, `judge:` or `quality:`;
- `min_score` outside 1–5;
- a negative budget;
- a provider other than anthropic or openai;
- `samples` outside 1–10;
- an `expect:` with no check;
- an unreadable or invalid schema.

The engine adds these keys (additive; the Stresseur CLI's strict parser must learn
`max_ttft_ms` before it can read files that use it):

| Key                                               | Where        | Meaning                                                              |
| ------------------------------------------------- | ------------ | -------------------------------------------------------------------- |
| `stream: openai \| anthropic \| vercel-ai \| sse` | request step | Read the response as a stream and assemble its text with this preset |
| `stream_timeout_ms`                               | request step | The longest the stream may stay open (default 30000)                 |
| `max_ttft_ms`                                     | `expect:`    | `ttft` check: time to first token ≤ limit                            |

`iterations:` and `quality.fail_below` across many runs belong to `stress ci`. The engine
keeps `iterations:` for the round trip and otherwise ignores it.

## Storage

The settings travel in `WorkspaceBundle`:

- `AIChecks` (`mexpect.Checks`) holds the file and flow settings and each step's `expect:`, keyed
  by flow node ID.
- `HTTPStreams` (`mhttp.HTTPStream`) holds `stream:` and `stream_timeout_ms` per HTTP request.

Both are stored in the workspace database (migration `01M4JSNG`), so the desktop app keeps them:

| Table              | Key            | Holds                                                     |
| ------------------ | -------------- | --------------------------------------------------------- |
| `http_stream`      | `http_id`      | `preset`, `timeout_ms` (0 = default)                      |
| `flow_node_expect` | `flow_node_id` | the step's `expect:` block as JSON                        |
| `flow_ai_checks`   | `flow_id`      | the flow's `judge:`, `quality:` and `iterations:` as JSON |

All three cascade on delete. File-level `judge:`/`quality:`/`iterations:` are folded into each
flow on import (`aicheck.FoldFileSettings`), so an export writes them on the flows.

What keeps them:

- **YAML import and export**, in the CLI (`ioworkspace.Import`/`Export`) and the desktop app
  (`rimportv2` → `rexportv2`).
- **HAR import:** see below.
- **Copy and paste of flow nodes:** the pasted nodes keep their `expect:`; requests the paste
  creates keep `stream:` (a pasted node that reuses an existing request keeps that request's).
- **Duplicating a request or a flow.**

## HAR import

An entry whose response is `text/event-stream` (the content's `mimeType` or the `Content-Type`
header) imports as a streaming request. The preset comes from the first recorded event, with
the Stresseur generator's rules, so a HAR import and a Stresseur recording agree:

- `openai`: a JSON payload with `choices`, `object: chat.completion.chunk`, or a `type` starting
  with `response.`.
- `anthropic`: an `event:` or `type` of `message_start`, `content_block_delta`, `ping`, etc.
- `vercel-ai`: a `type` of `start`, `text-delta`, `finish`, etc., or v4 data-stream lines
  (`0:"text"`).
- `sse`: anything else.

Bodies can be plain text, base64 (`encoding: base64`, as Firefox stores them) or missing (as
Chrome often does). Without a body, or if it doesn't decode, the preset is `sse`. Exporting the
imported workspace writes `stream: <preset>` on those steps.

## Evaluation (`pkg/aicheck`)

This package ports the Stresseur CLI's `stress/internal/expect` with its behavior and tests:
templates and path resolution, the four checks, the G-Eval judge, the cache, pass-rate
summaries and console lines.

- **When:** after a flow's steps and cleanup finish, and before the flow result is reported. Step
  and flow durations are already fixed by then, so checks never count in timings.
- **Order:** deterministic checks first. Judge calls then run in parallel, 8 at a time, with
  identical outputs judged once.
- **Inputs:** each step's node output (`request`, `response`) and its measured duration. A step
  that runs several times, in a loop, is checked on its last execution. Steps that did not run
  get no checks.
- **Checks:** `schema`, `latency`, `ttft`, `tokens` and `judge`. A streamed response with no
  `usage` path set falls back to the usage the stream reported (`response.usage`).
- **Judge:**
  - **Prompt:** G-Eval, with data tags carrying a per-content hash suffix and `<`/`>`
    neutralized, so the graded output cannot close or forge them. The reply is strict JSON
    (`reason`, then `score`).
  - **Scoring:** OpenAI-compatible providers are asked for logprobs, and the score is weighted
    over 1–5. Otherwise a borderline score is re-judged with `samples − 1` more calls at
    temperature 1, and the result is the mean.
  - **Retries:** 429, 5xx and network errors are retried 3 times with backoff; after that the
    check is skipped with the reason.
- **Keys:**
  - **Where:** `api_key_env` alone when set; otherwise `STRESSEUR_JUDGE_API_KEY`, then
    `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`. Both the process environment and `./.env` count.
  - **No key:** judge checks are skipped, and `flow run` prints once:
    `Judge checks skipped: set ANTHROPIC_API_KEY or STRESSEUR_JUDGE_API_KEY.`
  - **Never stored:** the key is not written to the report or the cache.
- **Cache:** a JSON file keyed on sha256 of (provider, model, base_url, samples, criteria, steps,
  min_score, input, output).
  - **Location:** `--judge-cache <path>`, default `.devtools/judge-cache.json`; `--judge-cache off`
    disables it. Stresseur passes `.stresseur/judge-cache.json` to keep its documented path.
  - **Contents:** at most 5,000 entries, the oldest dropped first. Errors are never cached.
- **Exit code:** with `quality.fail_below` set (flow, else file), a check whose pass rate falls
  below it fails the flow (`AI checks failed: …`). Without it, checks only report.
- **Not covered:** load runs (`--scenario`, `--vus`) do not evaluate `expect:`.

## Output

Each step result in the JSON report (`--report json:<path>`) gets `checks[]`:

`{kind, passed, score, reason, judge_model, cached, value, limit}`, plus `skipped`,
`judge_calls`, `judge_input_tokens` and `judge_output_tokens` when they apply.

Reasons are one line, at most 300 characters. The flow result gets `checks_status`
(`passed | warn | failed | skipped`) and `judge {calls, cached, elapsed_ms}`.

The console prints, after the flow's table:

```
Checks
  ✓ AskAssistant schema
  ✓ AskAssistant latency 59 ms ≤ 4 s
  ✗ AskAssistant judge 2 (min 4): Answer omits where to cancel.
Judge: 1 call, 1.3 s
```

## SSE (`pkg/httpclient`)

- **When it applies:** a response streams when its content type is `text/event-stream` or the step
  sets `stream:`. A text/event-stream response without `stream:` uses the raw `sse` preset.
- **Reading:** the body is read incrementally and parsed per the SSE spec (`data:`, `event:`,
  `id:`, comments, CRLF). `vercel-ai` also accepts the v4 data-stream lines (`0:"text"`).
- **Assembling text, by preset:**
  - `openai`: Chat Completions `choices[0].delta.content` and Responses API
    `response.output_text.delta`;
  - `anthropic`: `content_block_delta` `text_delta`;
  - `vercel-ai`: `text-delta` (`delta`, or `textDelta` in earlier versions);
  - `sse`: every data payload except `[DONE]`.
- **Usage:** OpenAI's final chunk, Anthropic's `message_start` and `message_delta`, and the
  Vercel `finish` part are merged into `response.usage`.
- **What assertions and `expect:` see:**
  - `response.text`, `response.events` (`{event, data, id}`, with `data` parsed when it is
    JSON);
  - `response.event_count`;
  - `response.ttft_ms`: time to the first event that adds text, or to the first data event
    for raw `sse`;
  - `response.usage`;
  - `response.body`, which stays the raw stream text.
- **Limits:**
  - At most 10,000 events are kept, though `event_count` counts them all.
  - Lean (load) mode keeps the text and the counts but not the events.
- **Timeout:** a stream still open after `stream_timeout_ms` fails the step:
  `stream did not end within 30s (N events received)`.

## Desktop

The desktop app's request runs get SSE reading and the `response.text`, `response.events` and
`response.ttft_ms` bindings in assertions through the same HTTP client. It stores `stream:` and
`expect:` (see Storage) so they survive import, export, copy/paste and duplicates, but it doesn't
show them, run the checks, or apply a stored preset: a text/event-stream response is read with
the raw `sse` preset. Showing them would need new RPC fields.

## Tests

- **Port of the CLI tests:** parse/validation, schema, latency, tokens, render, prompt injection,
  verdict parsing, Anthropic/OpenAI judge, logprob weighting, borderline sampling, retries,
  cache bound, evaluation with cache and dedupe, no-key skip, never judging an unfilled
  template.
- **YAML:** round trip and strict errors.
- **Storage:** CLI and desktop YAML import → DB → export, HAR import → export (one fixture per
  preset plus base64 and empty bodies), copy/paste, request and flow duplicates, the migration.
- **SSE:** a local test server per preset (openai, anthropic, vercel-ai SSE and v4 lines, raw
  sse), TTFT, the event cap, and a stream that never closes (timeout).
- **CLI end to end:** `flow run` with `expect:` against a local app and a fake judge: console
  lines, the JSON `checks[]`, and the `fail_below` exit code.
