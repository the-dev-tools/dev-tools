## 0.3.2 (2026-03-03)

### 🩹 Fixes

- Fix startup reliability and data migration for upgrading users. Migrate database from old data directories (DevTools, DevTools Studio). Catch protocol handler errors during server startup. Cap health check retry backoff. Add branded loading screen and error recovery UI. Fix migration FK reference to prevent folder hierarchy flattening. Make telemetry non-blocking. ([a7de9ad6](https://github.com/the-dev-tools/dev-tools/commit/a7de9ad6))

### ❤️ Thank You

- moosebay

## 0.3.1 (2026-03-02)

### 🩹 Fixes

- Fix CodeMirror crash caused by duplicate @codemirror/state instances. Fix server lint by broadening dbtest build tag and removing dead code. ([05ca2a7a](https://github.com/the-dev-tools/dev-tools/commit/05ca2a7a))

### ❤️ Thank You

- Claude Opus 4.6
- moosebay

## 0.3.0 (2026-02-26)

### 🚀 Features

- Add dark mode ([73c1cb75](https://github.com/the-dev-tools/dev-tools/commit/73c1cb75))

### 🩹 Fixes

- Fix AI node export and credential env var name sanitization ([36fd3671](https://github.com/the-dev-tools/dev-tools/commit/36fd3671))
- Optimise package size ([0da13cbe](https://github.com/the-dev-tools/dev-tools/commit/0da13cbe))

### ❤️ Thank You

- ElecTwix @ElecTwix
- Tomas Zaluckij @Tomaszal

## 0.2.3 (2026-02-10)

### 🩹 Fixes

- Version snapshots with is_snapshot column and deterministic test sync ([e6c2b883](https://github.com/the-dev-tools/dev-tools/commit/e6c2b883))
- Add uuid() and ulid() built-in expression functions with v4/v7 support ([ecfa35df](https://github.com/the-dev-tools/dev-tools/commit/ecfa35df))

### ❤️ Thank You

- ElecTwix @ElecTwix

## 0.2.2 (2026-02-09)

### 🩹 Fixes

- Version snapshots with is_snapshot column and deterministic test sync ([e6c2b883](https://github.com/the-dev-tools/dev-tools/commit/e6c2b883))

### 🧱 Updated Dependencies

- Updated cli to 0.2.1

### ❤️ Thank You

- ElecTwix @ElecTwix

## 0.2.1 (2026-02-09)

### 🩹 Fixes

- Fix auto-update not working correctly ([764e4384](https://github.com/the-dev-tools/dev-tools/commit/764e4384))

### ❤️ Thank You

- Tomas Zaluckij @Tomaszal

## 0.1.6 (2026-01-12)

### 🩹 Fixes

- Fix Windows binary path ([2f37717b](https://github.com/the-dev-tools/dev-tools/commit/2f37717b))

### ❤️ Thank You

- ElecTwix @ElecTwix

## 0.1.5 (2026-01-11)

### 🩹 Fixes

- Revert to manual ASAR binary unpacking and execution due to issues on Windows and MacOS ([6646917a](https://github.com/the-dev-tools/dev-tools/commit/6646917a))
- Fix Windows RPC issues by switching to named pipes ([e72a8a65](https://github.com/the-dev-tools/dev-tools/commit/e72a8a65))

### ❤️ Thank You

- Tomas Zaluckij @Tomaszal

## 0.1.4 (2026-01-10)

### 🩹 Fixes

- Fix Go binary linking ([64a948af](https://github.com/the-dev-tools/dev-tools/commit/64a948af))

### ❤️ Thank You

- ElecTwix @ElecTwix

## 0.1.3 (2026-01-09)

### 🩹 Fixes

- Add a 'diagnostics' command to help debug Electron issues ([e9a962ed](https://github.com/the-dev-tools/dev-tools/commit/e9a962ed))

### ❤️ Thank You

- Tomas Zaluckij @Tomaszal

## 0.1.2 (2026-01-09)

### 🩹 Fixes

- Manually unpack Go binaries from desktop Electron ASAR archive ([#15](https://github.com/the-dev-tools/dev-tools/pull/15))

### ❤️ Thank You

- DANG QUOC SON
- quocson95 @quocson95
- Tomas Zaluckij @Tomaszal

## 0.1.1 (2026-01-08)

### 🩹 Fixes

- Allow creating new nodes by draging from the node handle and the plus button ([d74275dc](https://github.com/the-dev-tools/dev-tools/commit/d74275dc))
- Fix desktop binary execution from ASAR archives ([9f3b1602](https://github.com/the-dev-tools/dev-tools/commit/9f3b1602))

### ❤️ Thank You

- Tomas Zaluckij @Tomaszal