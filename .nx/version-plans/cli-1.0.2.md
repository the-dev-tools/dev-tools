---
cli: patch
---

### Bug fixes

- **Startup migration repairs pathological file ordering.** The embedded server now repacks `files.display_order` values that converged to float32 MAX (a bug in desktop order generation) back to small sequential numbers, preserving relative order. Databases shared with a broken desktop workspace recover automatically. ([#44](https://github.com/the-dev-tools/dev-tools/issues/44))
