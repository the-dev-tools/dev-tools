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

## Supported Steps

- `manual_start`: Entry point for flow execution.
- `request`: Execute an HTTP request.
- `js`: Execute JavaScript code.
- `if`: Conditional branching.
- `for` / `for_each`: Iteration.
