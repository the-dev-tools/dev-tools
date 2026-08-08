---
cli: minor
desktop: minor
---
Load testing foundations: local load mode (`flow run --vus/--duration` and `load:` scenarios with percentile tables + `load_report` JSON), versioned yamlflow schema (`version: 2`), HTTP assertions in yamlflow files now enforced on import (previously silently dropped), `run:` blocks execute in dependency order with strict failure modes (failed dependencies skip-and-continue instead of aborting the run), `run-flows` GitHub Action for CI, and real build-version reporting in `devtools version`.
