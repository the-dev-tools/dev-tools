# Simplified YAML Flow Format (V2)

This directory implements the V2 Simplified YAML Flow format, designed for human readability and "Parallel by Default" execution.

## Core Principles

1.  **Parallel by Default**: Steps listed without dependencies run in parallel, implicitly depending on the `Start` node.
2.  **Explicit Dependencies**: Serial execution must be explicitly defined using `depends_on`.
3.  **Unified Control Flow**: Control flow logic (`if`, `for`) uses standard `depends_on` with dot-notation (`Node.handle`) rather than nested or special fields.

## Execution Model

### Parallel Execution (Default)

Steps A and B run simultaneously.

```yaml
steps:
  - manual_start:
      name: Start
  - js:
      name: A
      # No depends_on -> Depends on Start
  - js:
      name: B
      # No depends_on -> Depends on Start
```

### Serial Execution

Step B waits for Step A.

```yaml
steps:
  - manual_start:
      name: Start
  - js:
      name: A
      depends_on: [Start]
  - js:
      name: B
      depends_on: [A]
```

### Reading another step's output requires depending on it

A step that reads another step's output, such as `{{ Login.response.body.token }}` in a
field or `Login.response.status` in an assertion or condition, must depend on that step,
directly or through steps in between. Otherwise the import fails with an error that names
the step and the missing `depends_on`. Without this check the step would run in parallel
with the step it reads, see the reference unfilled, and the flow could still pass.

The converter rejects the flow instead of adding the edge itself. `depends_on` is the
explicit ordering contract: it decides what runs in parallel, and with an explicit
`manual_start` a step that has no `depends_on` intentionally never runs. Inferring edges
would silently change both, along with the graph that is exported back.

These are exempt from the check:

- steps that never run;
- a loop's `break_condition`, which is evaluated after each iteration and so may read
  the loop body;
- JS `code`, which reads steps through its context argument rather than templates.

## Control Flow

Control flow nodes (`if`, `for`) emit signals (handles) that other nodes listen to.

### Conditional (If/Else)

```yaml
steps:
  - if:
      name: Check
      condition: response.status == 200

  - js:
      name: OnSuccess
      depends_on: [Check.then] # Runs if condition is true

  - js:
      name: OnFailure
      depends_on: [Check.else] # Runs if condition is false
```

### Loops (For/ForEach)

```yaml
steps:
  - for_each:
      name: Loop
      items: [1, 2, 3]

  - js:
      name: ProcessItem
      depends_on: [Loop.loop] # Runs for each iteration
```

## Cleanup Steps

A flow's optional `cleanup:` list runs after its `steps:` reach a terminal
state, whether they passed or failed, so data a run creates is removed even
when a step in the middle fails:

```yaml
flows:
  - name: Catalog
    steps:
      - request:
          name: PostProducts
          method: POST
          url: '{{ baseUrl }}/products'
    cleanup:
      - request:
          name: DeleteProduct
          method: DELETE
          url: '{{ baseUrl }}/products/{{ PostProducts.response.body.id }}'
```

- Only `request` and `graphql` steps are allowed in `cleanup:`.
- Cleanup steps run one at a time in listed order; `depends_on` may name only
  other cleanup steps and reorders them. Every step is attempted, even after
  another cleanup step fails.
- A cleanup step whose templates read a step that produced no output (it never
  ran) is skipped, as is one whose cleanup dependency did not succeed.
- A failing cleanup step fails a flow that otherwise passed. When the flow
  already failed, the original failure stays the reported error.
- Cleanup steps run in the CLI (`flow run`, per iteration in load runs). They
  do not run when Ctrl-C interrupts a run, when the flow runs as a sub-flow,
  or in the desktop app, which does not store them yet.

## Load Scenarios

The optional top-level `load:` block describes load profiles for flows that
already exist in `flows:`. A flow is never edited to be load-tested; a scenario
names its flow and adds a schedule, and `devtoolscli flow run <file>
--scenario <name>` runs it. The same entries can live in a separate file (a
`load:` block, or a bare list of entries) passed with `--load-file`.

```yaml
load:
  - name: checkout-ramp
    flow: Checkout
    executor: ramping-vus
    start_vus: 0
    stages:
      - { duration: 30s, target: 50 }
      - { duration: 5m, target: 50 }
      - { duration: 30s, target: 0 }
    think_time: { min: 1s, max: 3s }
    thresholds:
      p95: <300ms
      errors: <1%
      steps:
        PostLogin:
          p95: <500ms
    abort:
      - errors>20%
      - when: p95>2s
        window: 1m
        delay: 30s
```

### Executors

`executor` defaults to `constant-vus`. A key that does not apply to the chosen
executor is rejected rather than ignored.

| Executor                | Model  | Keys                                                                                          |
| ----------------------- | ------ | --------------------------------------------------------------------------------------------- |
| `constant-vus`          | closed | `vus`, plus `duration` and/or `iterations`                                                    |
| `ramping-vus`           | closed | `stages`, `start_vus` (0), `graceful_ramp_down` (30s), `graceful_stop` (30s)                  |
| `constant-arrival-rate` | open   | `rate`, `duration`, `pre_allocated_vus`, `time_unit` (1s), `max_vus`, `graceful_stop` (30s)   |
| `ramping-arrival-rate`  | open   | `stages`, `pre_allocated_vus`, `start_rate` (0), `time_unit` (1s), `max_vus`, `graceful_stop` |

- **Closed models** loop each VU: a VU starts its next iteration when the
  previous one ends, so a slower target means fewer iterations.
- **`ramping-vus`** moves the number of looping VUs linearly through
  `stages` (`target` is a whole number of VUs). A VU that is ramped away may
  finish its iteration for up to `graceful_ramp_down`; after that, and after
  `graceful_stop` once the last stage ends, the iteration is interrupted.
  Interrupted iterations and their canceled requests are reported separately,
  never as errors.
- **Open models** start iterations on schedule regardless of response time:
  `rate` iterations per `time_unit`, or a rate that moves linearly through
  `stages` (`target` is a rate). Each start takes an idle VU, growing the pool
  from `pre_allocated_vus` up to `max_vus` (default: `pre_allocated_vus`).
  When every VU is busy and the pool is full the start is **dropped** and
  counted as a dropped iteration in the report.
- `think_time` pauses a VU after each iteration: a duration (`think_time: 1s`)
  or a uniform range (`think_time: { min: 1s, max: 3s }`). It applies to every
  executor.
- Durations are Go durations (`500ms`, `30s`, `2m`, `1h30m`) and are exported
  in canonical form (`2m0s`).

### Thresholds

`thresholds` are pass/fail conditions checked once the run ends. Any failure
makes `flow run` exit with code 99, and every verdict is printed under the
results table and written to the JSON report. Two spellings are accepted, and
export always uses the map form:

```yaml
# map form
thresholds:
  p95: <300ms # whole run
  errors: <1%
  steps:
    PostLogin: # one request step
      p95: <500ms
```

```yaml
# list form
thresholds:
  - p95<300ms
  - p95(PostLogin)<500ms
  - errors<1%
```

- Metrics: `p50`, `p90`, `p95`, `p99`, `max` (compared with a duration),
  `errors` (alias `error_rate`; a percentage like `1%` or a ratio like `0.01`)
  and `rps` (requests per second).
- Operators: `<`, `<=`, `>`, `>=`. Quote a value that starts with `>`
  (`rps: '>100'`), since YAML reads a bare `>` as a block scalar.
- A threshold on a step that recorded no requests fails with `no data`.

### Abort rules

`abort` rules are evaluated on every metrics frame (every 5s by default, see
`--frame-interval`) over a trailing `window` (default 30s), ignoring the first
`delay`. The first rule whose condition holds stops the run: no new iterations
start, in-flight ones get the executor's graceful stop, the report is still
written, and `flow run` exits with code 108. A rule uses the threshold grammar,
but it fires when the condition is true: `errors>20%` aborts once more than 20%
of requests in the window failed. The shorthand `- errors>20%` is a rule with
the default window and no delay.

## AI Checks (`expect:`)

A `request` or `graphql` step can carry an `expect:` block. `assertions:` are evaluated inline
as the step runs. `expect:` is evaluated after the flow's steps (and cleanup) finish, so it
never adds to step or flow timings. The syntax is the one the Stresseur CLI reads; the
full design is in `docs/specs/AI_CHECKS.md`.

```yaml
judge: # file level, optional; a flow's judge: overrides it key by key
  provider: anthropic # anthropic (default) | openai (any OpenAI-compatible API)
  model: claude-haiku-4-5-20251001
  api_key_env: ANTHROPIC_API_KEY # optional
  samples: 3 # borderline re-judging without logprobs; 1 turns it off
quality:
  fail_below: 90% # optional; without it, failed checks only report
flows:
  - name: AssistantAnswersBilling
    steps:
      - request:
          name: AskAssistant
          method: POST
          url: '{{ BASE_URL }}/api/assistant'
          expect:
            schema: # inline JSON Schema, or a path relative to the flow file
              type: object
              required: [answer, citations]
            schema_at: response.body # default
            max_latency_ms: 4000
            usage: response.body.usage # default; a stream's response.usage when unset
            max_output_tokens: 400 # also max_input_tokens, max_total_tokens
            judge:
              criteria: Answers how to cancel, using only facts from the cited docs.
              steps:
                - Check the answer says where cancellation happens (Settings > Billing).
              input: '{{ AskAssistant.request.body.question }}'
              output: '{{ response.body.answer }}'
              min_score: 4 # 1-5
```

Checks are `schema`, `latency`, `ttft`, `tokens` and `judge`:

- **Judge:** a G-Eval rubric scored 1–5. The output is passed as delimited data, never as
  instructions. OpenAI-compatible judges are logprob-weighted; otherwise a borderline score
  is re-judged `samples − 1` times and averaged.
- **Judge key:** `api_key_env`, else `STRESSEUR_JUDGE_API_KEY`, else `ANTHROPIC_API_KEY` or
  `OPENAI_API_KEY`, read from the environment or `./.env`. Without a key, judge checks are
  skipped with a notice.
- **Cache:** verdicts are cached in `--judge-cache` (default `.devtools/judge-cache.json`).
- **Results:** `devtools flow run` prints the checks under the flow's table and writes them to
  the JSON report as each step's `checks[]`.
- **Validation:** strict. An unknown key, a `min_score` outside 1–5, a negative budget or an
  unreadable schema stops the import with an error naming the step.

## Streaming responses (`stream:`)

A response with `Content-Type: text/event-stream`, or any response of a step with
`stream:`, is read as it arrives. The step's assertions, its `expect:` block and later steps
can then read:

- `response.text`, the assembled message;
- `response.events`, the parsed events;
- `response.event_count`;
- `response.ttft_ms`, the time to the first token;
- `response.usage`, when the stream reports it.

`response.body` stays the raw stream.

```yaml
- request:
    name: Chat
    method: POST
    url: '{{ BASE_URL }}/api/chat'
    stream: openai # openai | anthropic | vercel-ai | sse (raw)
    stream_timeout_ms: 30000 # default; a stream still open after this fails the step
    assertions:
      - response.text contains "Billing"
    expect:
      max_ttft_ms: 800
      judge:
        criteria: Names where to cancel.
        output: '{{ response.text }}'
```

The presets assemble the text as follows:

- `openai`: Chat Completions deltas and Responses API `output_text` deltas.
- `anthropic`: `text_delta` events.
- `vercel-ai`: AI SDK `text-delta` parts, including the v4 data-stream lines.
- `sse`: every data payload except `[DONE]`. This is the default for a text/event-stream
  response without `stream:`.

A HAR import sets `stream:` on each request whose recorded response is text/event-stream,
picking the preset from the first event (`sse` when the browser didn't keep the body), so
exporting the imported workspace writes it.

`stream:`, `stream_timeout_ms`, `expect:` and the flow's `judge:`, `quality:` and `iterations:`
are stored with the workspace, so they survive importing into the desktop app and exporting
again, copying and pasting nodes, and duplicating requests or flows. File-level settings are
written back on each flow. The desktop app keeps them but doesn't show or run them yet.

## Supported Steps

- `manual_start`: Entry point for flow execution.
- `request`: Execute an HTTP request.
- `js`: Execute JavaScript code.
- `if`: Conditional branching.
- `for` / `for_each`: Iteration.
