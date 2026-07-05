---
cli: patch
---

### Bug fixes

- **Windows binaries restored.** A stale `@nx/eslint` patch made dependency installation fail hard on Windows CI, so 1.0.2 shipped without `win32-x64`/`win32-ia32` binaries. The dead patch is removed; this release ships all platforms again. Includes the file `display_order` repair migration from 1.0.2 ([#44](https://github.com/the-dev-tools/dev-tools/issues/44)).
