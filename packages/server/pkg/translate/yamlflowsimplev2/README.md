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

## Supported Steps

- `manual_start`: Entry point for flow execution.
- `request`: Execute an HTTP request.
- `js`: Execute JavaScript code.
- `if`: Conditional branching.
- `for` / `for_each`: Iteration.
