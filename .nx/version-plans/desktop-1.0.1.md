---
desktop: patch
---

### Bug fixes

- **Loop break condition now sees inner-node outputs.** For/ForEach break expressions are evaluated **after** each iteration's children run, so they can reference values produced during that iteration (e.g. `{{ http_1.response.body.done }}`). Previously the check ran before children, so any expression referencing a not-yet-written variable failed the entire flow on the first iteration. Missing identifiers are now treated as "don't break" (loops are still bounded by iteration count). ForEach semantics also aligned with For: an expression that evaluates true exits the loop. ([#42](https://github.com/the-dev-tools/dev-tools/issues/42))
- **YAML imports no longer fail with a foreign-key error.** Workspace YAML imports were storing HTTP requests before their parent folder file in `StoreUnifiedResults`, so SQLite rejected the row with `FOREIGN KEY constraint failed (787)` whenever `GenerateFiles=true`. Files are now stored first, and HTTP `folder_id` references are remapped before insertion.
- **Imported workspaces show up in the UI immediately.** The import path now publishes mutation events for newly created flows, flow nodes, per-type node configs (For, ForEach, JS, Condition, AI), edges, flow variables, and HTTP requests, so the desktop's TanStack DB collections refresh in real time instead of waiting for a manual reload. Previously, imported For nodes appeared with `Iterations: 0` and an empty break expression until you closed and reopened the workspace.

### Other

- New `break_condition` field on `for` / `for_each` steps in YAML workspaces, mirroring the desktop UI's "Break If" setting.
