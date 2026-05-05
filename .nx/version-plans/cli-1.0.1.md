---
cli: patch
---

### Bug fixes

- **Loop break condition now sees inner-node outputs.** For/ForEach break expressions are evaluated **after** each iteration's children run, so they can reference values produced during that iteration (e.g. `{{ http_1.response.body.done }}`). Previously the check ran before children, so any expression referencing a not-yet-written variable failed the entire flow on the first iteration. Missing identifiers are now treated as "don't break" (loops are still bounded by iteration count). ForEach semantics also aligned with For: an expression that evaluates true exits the loop. ([#42](https://github.com/the-dev-tools/dev-tools/issues/42))

### Other

- New `break_condition` field on `for` / `for_each` steps in YAML workspaces, so loops in CLI-driven flows can exit on a runtime predicate without needing the UI.
