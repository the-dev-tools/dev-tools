---
desktop: patch
---

### Bug fixes

- **Workspaces no longer go blank after many file reorders.** File append/reorder used the midpoint between the last position and float32 MAX to generate `display_order`, converging to the float32 limit after a few dozen inserts — at which point every file in the workspace disappeared from the UI and new flows could not be created. Order generation now uses fixed steps, and a startup migration repacks existing `display_order` values to small sequential numbers (preserving your current order), so previously broken workspaces recover automatically. ([#44](https://github.com/the-dev-tools/dev-tools/issues/44))
