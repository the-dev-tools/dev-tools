---
cli: minor
---

Flows can now declare `cleanup:` steps that always run after the flow's normal steps, whether they passed or failed, so data a test creates is deleted even when a step in the middle fails and the next run starts from the same state. Cleanup steps are `request` or `graphql` steps that can read earlier outputs (`{{ PostProducts.response.body.id }}`); they run in listed order (`depends_on` may reorder them among themselves), a step that reads a step which never ran is reported as skipped, and a failing cleanup step fails an otherwise passing flow while an earlier failure stays the reported error. Cleanup steps appear after the normal steps in the console table and in JSON and JUnit reports, and run after every iteration in load runs without being measured.
