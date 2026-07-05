---
desktop: patch
---

### Bug fixes

- **Windows builds restored.** A stale `@nx/eslint` patch made dependency installation fail hard on Windows CI, so 1.0.2 shipped without Windows (and, due to an expired Apple agreement, macOS) artifacts. The dead patch is removed; this release ships installers for all platforms again. Includes everything from 1.0.2, notably the fix for workspaces going blank after many file reorders ([#44](https://github.com/the-dev-tools/dev-tools/issues/44)).
