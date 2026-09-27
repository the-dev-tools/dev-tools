# Rename inventory: DevTools Studio → Stresseur Studio (Step 0)

Status: inventory only. Nothing has been renamed. Steps 1–7 are waiting for go-ahead.
Base commit: `e968d04c` (main, "Merge pull request #46"). Scanned 1,356 tracked files (`git ls-files`, so
`node_modules`, `dist`, `out` and other ignored build output are excluded) for: `DevTools`, `devtools`, `dev-tools`,
`dev.tools`, `DEVTOOLS_`, `the-dev-tools`, `dev tools` (case-insensitive). There were 4,392 matching lines in 927 files.

**Bucket totals** (one row = one table entry below; bulk usages are rolled up with counts):

| Bucket | Entries | Notes |
|---|---|---|
| Display (rename) | 38 | 9 of them marked **AMBIGUOUS** |
| Identifier (keep) | 36 | plus rolled-up usages: 3,147 Go import lines in 694 files, 512 `@the-dev-tools/*` TS imports in 128 files, 85 `devtoolsdb` refs in 26 files, 131 import paths in `sqlc.yaml` |
| Contract (keep working) | 30 | 2 marked **AMBIGUOUS** (C2 cobra `Use`, C3 version line, which conflicts with Step 4) |
| Not our name (false positives) | n/a | Chrome DevTools / TanStack / React / Electron devtools, listed at the end so nobody renames them |

---

## A. Pre-flight findings

### A1. Desktop stack and packager: confirmed Electron + electron-builder (with electron-vite)

- `apps/desktop/package.json:9-10`: `"build": "electron-vite build && node build.ts"`, `"dev": "electron-vite dev"`.
- `apps/desktop/package.json` devDependencies: `electron`, `electron-builder` (26.8.1 in the store), `electron-updater` (6.8.3), `electron-vite`.
- `apps/desktop/build.ts`: the electron-builder config is **programmatic** (`build({ config, publish: 'never' })`). There is no
  `electron-builder.yml` and no `build` key in package.json.
- Targets: mac `dmg`+`zip` (defaults, `hardenedRuntime`, notarized in CI), Windows NSIS (`oneClick: false`,
  `allowToChangeInstallationDirectory: true`, Azure Key Vault signing), Linux `AppImage`.
- CI: `.github/workflows/release-electron-builder.yaml` → `pnpm nx run desktop:build` → `gha-scripts upload-electron-release-assets`.
- **No** explicit `appId`, `productName`, `executableName`, `nsis.guid`, `win.publisherName`, `protocols`, or `publish.channel` anywhere.

### A2. How the Electron userData path is derived today

There is no `app.setName()` or `app.setPath('userData', …)` anywhere in `apps/desktop/src`. Electron derives the path from the
**packaged** `package.json`:

1. `apps/desktop/build.ts:10-12` sets `extraMetadata: { name: 'DevTools-Studio' }`.
2. electron-builder merges `extraMetadata` into the packaged `package.json` (`app-builder-lib/out/fileTransformer.js:87-90`).
   The source `package.json` has no `productName`, and electron-builder does **not** write `config.productName` into the
   packaged `package.json`.
3. So at runtime `app.getName()` = `productName ?? name` = **`DevTools-Studio`**, and userData is:
   - macOS `~/Library/Application Support/DevTools-Studio`
   - Windows `%APPDATA%\DevTools-Studio`
   - Linux `~/.config/DevTools-Studio`
   - dev mode (`electron-vite dev`, unpackaged): `…/@the-dev-tools/desktop`
4. The server gets `DB_PATH: app.getPath('userData')` and `DB_NAME: 'state'` (`apps/desktop/src/main/index.ts:191-193`), so the
   workspace DB is `<userData>/state.db` (+ `-wal`/`-shm`). Agent logs are in `<userData>/logs/agent`.
5. Precedent: this has moved twice before (`DevTools` → `DevTools Studio` in `b3d51d13`, then `DevTools-Studio` in `764e4384`).
   `migrateDataDir()` (`index.ts:141-170`) copies `state.db*` from `DevTools Studio` and `DevTools` only. It does **not** list
   `DevTools-Studio`, so a new move would not be caught by it and existing users would see an empty workspace.

What this means for Step 1:
- Adding a `productName` to `extraMetadata` or to `package.json` **moves userData**.
- Changing `extraMetadata.name` **moves userData and changes appId**. appId feeds the bundle ID, the NSIS GUID and the AUMID
  (see I1), which means a second install on Windows.
- Setting only `config.productName` in `build.ts` does **not** move userData, but it does change artifact names, `.app`/`.exe`
  names, and the NSIS install directory and shortcut names (see A3 and I1).

### A3. How release artifact names are derived

- **Desktop:** `apps/desktop/build.ts:9` `artifactName: '${productName}-${version}-${platform}-${arch}.${ext}'`.
  - `productName` falls back to `metadata.name` = `DevTools-Studio` (`app-builder-lib/out/appInfo.js:54`).
  - `${platform}` = Node `process.platform` (`macroExpander.js:34-35`).
  - Verified on the live releases (`gh api repos/the-dev-tools/dev-tools/releases`), e.g. desktop@1.1.0:
    `DevTools-Studio-1.1.0-darwin-arm64.dmg` / `.zip` (+ `.blockmap`), `DevTools-Studio-1.1.0-darwin-x64.dmg` / `.zip`,
    `DevTools-Studio-1.1.0-win32-x64.exe` (+ `.blockmap`), `DevTools-Studio-1.1.0-linux-x86_64.AppImage`.
  - Update metadata is renamed on upload to `latest-${process.platform}-${process.arch}.yml`
    (`tools/gha-scripts/src/cli.ts`, `upload-electron-release-assets`).
  - **Discrepancy:** the brief and `README.md:88-90` both say `DevTools-{version}-…`. The real prefix is **`DevTools-Studio-`**.
    README is stale.
- **CLI:** `apps/cli/taskfile.yaml:110` → `dist/devtools-cli-{VERSION}-{PLATFORM}{.exe}`. PLATFORM comes from the
  `release-go.yaml` matrix: `darwin-x64`, `darwin-arm64`, `linux-x64`, `linux-arm64`, `win32-x64`, `win32-ia32`.
  `upload-go-release-assets` uploads everything in `dist/` under its file name. Verified for cli@1.1.1: `devtools-cli-1.1.1-darwin-arm64`,
  `-darwin-x64`, `-linux-arm64`, `-linux-x64`, `-win32-ia32.exe`, `-win32-x64.exe`. **No `checksums.txt` is published**, so the
  checksum step in `install.sh` (lines 147-175) silently does nothing.
- Tags are `desktop@X.Y.Z` and `cli@X.Y.Z` (nx release, `tools/gha-scripts/src/repository.ts` splits on `@`).

### A4. Would a GitHub **pre-release** be picked up? **NOT SAFE as the release process works today**

No consumer in this repo reads GitHub's `prerelease` or "latest" flag. They all resolve "latest" by other means:

| Consumer | How it resolves "latest" | Picks up a pre-release? |
|---|---|---|
| Desktop auto-updater (`apps/desktop/src/main/update.ts:24-60`, wired at `index.ts:236-245`) | Reads `version` from **`apps/desktop/package.json` on `main`** via `raw.githubusercontent.com/…/refs/heads/main/…`, then `GET /repos/…/releases/tags/desktop@<version>` (this endpoint returns pre-releases too), then downloads `latest-<platform>-<arch>.yml`. Uses a `CustomUpdateProvider`, so electron-updater's `allowPrerelease` (GitHub provider only) does not apply. `AppUpdater.isUpdateAvailable` is a plain `semver.gt`. | **Yes, if** main's `apps/desktop/package.json` version equals the pre-release version. The flag doesn't matter. |
| Renderer "Update available" screen (`apps/desktop/src/renderer/main.tsx:37`) | Same tag lookup, to show the release body | Same as above |
| `apps/cli/install.sh` (`get_version`, lines 77-114) | Reads `version` from **`apps/cli/package.json` on `main`**, then checks that tag `cli@<version>` exists | **Yes, if** main's `apps/cli/package.json` points at it |
| GitHub Action `actions/run-flows` with `version: latest` (**the default**, `action.yml:15`; `scripts/download-cli.sh:49-61`) | `git ls-remote --tags 'cli@*' \| sort -V \| tail -n1` | **Yes, immediately**, as soon as any `cli@*` tag is pushed, whatever its prerelease flag and whatever main says. `sort -V` also ranks `cli@1.2.0-rc.1` **above** `cli@1.2.0` (verified locally), so an rc tag would stay "latest" even after the final release ships. |
| `install.ps1` | Not a CLI installer. It is the vendored upstream **Scoop installer** (header lines 1-40), used for the Windows CI toolchain (`.github/actions/dependencies-windows/action.yaml`). | n/a |
| `scoop.json` | Not a CLI manifest. It is a `scoop export` of CI build dependencies (7zip, gcc, go, jq…), refreshed by `.github/workflows/update-scoop.yaml`. | n/a |
| `flake.nix` | Dev shells only (`devShells.default`, `devShells.runner`). It exposes no CLI package. | n/a |

Why the normal release path exposes the pre-release to everyone:
- `release.yaml` → `gha-scripts release` → nx `releaseVersion` + `releaseChangelog` with `createRelease: "github"` (`nx.json` `release.changelog.projectChangelogs`).
- nx forces `git push` whenever `createRelease` is set (`nx/src/command-line/release/config/config.js:106-107`, "We have to perform a git push in order to create a release"). The version bump is committed and pushed to the ref the workflow was dispatched on.
- Dispatched on `main` (the default), that bump is exactly what the updater and install.sh read.
- nx *does* set `prerelease: true` on the GitHub release for semver pre-release versions (`remote-release-client.js:78`), but nothing reads that flag.

What would actually hide a week-2 pre-release (for the user to decide; nothing done):
- **Desktop:** the version on main's `apps/desktop/package.json` must never equal the pre-release version. For example,
  dispatch from a non-main branch and don't merge the bump back, or create the release by hand.
- **CLI:** *any* pushed `cli@*` tag is picked up by every Action user on `version: latest`. Either don't cut a `cli@` pre-release
  tag, or first change `download-cli.sh` to skip pre-release tags or resolve via the GitHub "latest release" API.
  That change is a behaviour change to a public contract, so it needs the user's call (hard rule 7).
- Also note `latest-*.yml`: the updater channel is always `latest`, because `publish` is `{ provider: 'custom' }` with no
  `channel` (`app-builder-lib/out/publish/updateInfoBuilder.js:37`). A `-beta` version does not get a separate channel file.

### A5. Items the brief assumes that don't match this repo

1. The desktop artifact prefix is `DevTools-Studio-`, not `DevTools-`.
2. There is no CLI `install.ps1`, no scoop bucket manifest for the CLI, and no CLI package in `flake.nix` (see A4 table).
   Step 4's alias work for those three has no target in this repo. If a scoop bucket or a Windows installer exists, it lives
   elsewhere. `sh.dev.tools/install.sh` is hosted outside the repo. Presumably it proxies `apps/cli/install.sh`, but that
   can't be verified from here.
3. The CLI already has **two** installed names: `install.sh` installs it as **`devtools`** (`install.sh:17`), while the Action
   and cobra use **`devtoolscli`** (`download-cli.sh:76`, `root.go:13`). Both are contracts.
4. The CLI has **no `--version` flag**. There is a `version` subcommand that prints `DevToolsCLI v1.1.1` (`version.go:25`).
   Step 4's "version text says Stresseur CLI" **conflicts** with hard rule 3 ("keep the `--version` format"). Needs a decision (C3).
5. JSON/JUnit reports contain **no** tool name or DevTools string (`apps/cli/internal/reporter/*.go`: `<testsuites>`/`<testsuite>`/`<testcase>`,
   no `name="DevTools…"`). Nothing to protect there beyond keeping the code unchanged.
6. Chrome extension: the build is **disabled** (`apps/api-recorder-extension/package.disabled.json`, `project.disabled.json`,
   whose build target just `echo`s: "Plasmo … doesn't work with Tailwind CSS v4"). The manifest has **no `key`**, so the
   extension ID is assigned by the Chrome Web Store and isn't in the repo. The extension does **not** talk to Studio: it
   downloads `postman-collection.json` (`popup.tsx:276`) and signs in via magic.link. There is no "Open in Studio"
   hook to rename. Its current `displayName` is `API Recorder`, with no old name in it.
7. No keychain/credential service names, no OS protocol handler (`server://` at `index.ts:58-66` is an in-process privileged
   scheme), no single-instance lock, no explicit AppUserModelId, and no container image names in this repo. The DB file is
   `state.db`, which doesn't contain the old name.

### A6. Hard-rule-7 risks (would move user data, break auto-update, or change a public contract)

These would be triggered by a naive Step 1/4/5, not by this inventory:

1. **Setting a desktop productName via `extraMetadata`/`package.json`, or changing `extraMetadata.name`** → userData moves
   (workspaces vanish; `migrateDataDir` doesn't know `DevTools-Studio`). Changing `name` also changes appId
   `com.electron.devtools-studio`, which changes the macOS bundle ID, the Windows NSIS GUID (UUIDv5 of appId), the uninstall
   registry key and the AUMID. Windows would then install a second copy instead of upgrading, and pinned taskbar shortcuts break.
2. **Setting only `config.productName` in `build.ts`** → changes artifact names (breaks README, dev.tools/download, and
   `latest-*.yml` file references for anyone who hard-links), the `.app`/`.exe` file names, the NSIS install directory
   (`getWindowsInstallationDirName`), and the Start-menu/desktop shortcut names. It needs a pinned `executableName`, a
   hardcoded `artifactName` prefix `DevTools-Studio-`, and a test that the Windows installer's old-uninstall path still finds
   `DevTools-Studio.exe`.
3. **Changing `CLI version` output** (`version.go:25`) → changes machine-readable output (C3).
4. **Changing Action `default: '.devtools-reports'`** or any input, output or asset name → breaks consumers' workflows.
5. **Cutting a `cli@*-rc` tag** → every Action user on `version: latest` gets it, and keeps getting it after the final release (A4).
6. **Letting the pre-release version land in main's `apps/desktop/package.json` or `apps/cli/package.json`** → every installed
   Studio and every `install.sh` user gets it (A4).

---

## B. Display (rename)

User-visible strings. **AMBIGUOUS** = displayed, but also doubles as something else; decide before changing.

| # | file:line | Exact string | Reason |
|---|---|---|---|
| D1 | apps/desktop/src/main/index.ts:43 | `'The x64 (Intel) build of DevTools Studio is running under Rosetta 2 on an Apple Silicon Mac. '` | Native dialog text |
| D2 | apps/desktop/src/main/index.ts:76 | `title: 'DevTools Studio',` | BrowserWindow title |
| D3 | apps/desktop/src/renderer/index.html:5 | `<title>DevTools Studio</title>` | Renderer document title (overrides window title once loaded) |
| D4 | apps/desktop/src/renderer/main.tsx:65 | `DevTools Studio` | "Update available" screen heading |
| D5 | apps/desktop/src/renderer/main.tsx:112 | `Starting DevTools Studio...` | Loading screen |
| D6 | apps/desktop/package.json:3 | `"description": "DevTools Studio is a powerful API testing tool that records …"` | Package description; electron-builder uses it for the Linux desktop-entry comment and installer metadata. Not an identifier |
| D7 | apps/desktop/package.json:4 | `"author": "DevTools",` | **AMBIGUOUS**: becomes Windows CompanyName / Publisher in Add/Remove Programs and the default copyright. DevTools stays the umbrella brand, so the recommendation is to **keep** |
| D8 | packages/client/index.html:5 | `<title>DevTools</title>` | Client (web build) page title |
| D9 | packages/client/src/pages/dashboard/routes/index.tsx:69 | `Welcome to DevTools 👋` | Dashboard empty state |
| D10 | packages/client/src/pages/user/routes/signIn.tsx:43 | `Welcome to DevTools` | Sign-in page |
| D11 | packages/client/src/pages/user/routes/signUp.tsx:43 | `Sign Up to DevTools` | Sign-up page |
| D12 | packages/client/src/pages/workspace/routes/workspace/$workspaceIdCan/index.tsx:26 | `Discover what you can do in DevTools` | Workspace onboarding |
| D13 | packages/client/src/pages/workspace/routes/workspace/$workspaceIdCan/route.tsx:126 | `DevTools v{…Config.string('VERSION')…}` | Sidebar version label (display only) |
| D14 | packages/client/src/features/file-system/index.tsx:624 | `YAML (DevTools)` | **AMBIGUOUS**: export menu item that names the *file format*. The format is unchanged, so renaming the label could suggest a new format. Decide: keep, or e.g. "YAML (DevTools / Stresseur)" |
| D15 | packages/client/src/features/file-system/index.tsx:789 | `YAML (DevTools)` | **AMBIGUOUS**: same as D14 |
| D16 | packages/client/src/features/file-system/index.tsx:924 | `Export YAML (DevTools)` | **AMBIGUOUS**: same as D14 (flow context menu) |
| D17 | packages/client/src/features/file-system/index.tsx:1077 | `YAML (DevTools)` | **AMBIGUOUS**: same as D14 |
| D18 | packages/client/src/features/file-system/index.tsx:1246 | `YAML (DevTools)` | **AMBIGUOUS**: same as D14 |
| D19 | packages/client/src/features/file-system/index.tsx:1388 | `YAML (DevTools)` | **AMBIGUOUS**: same as D14 |
| D20 | apps/api-recorder-extension/src/popup.tsx:63 | `>DevTools</h1>` | Extension sign-in heading (extension build currently disabled) |
| D21 | apps/api-recorder-extension/src/popup.tsx:99 | `>DevTools</h1>` | Extension recorder header |
| D22 | apps/cli/cmd/root.go:14 | `Short: "DevTools is a powerful API testing tool",` | CLI help text |
| D23 | apps/cli/cmd/root.go:15 | ``Long: `DevTools is a powerful API testing tool that records your browser interactions,`` | CLI help text |
| D24 | apps/cli/cmd/import.go:46 | `and HAR files into your DevTools workspace using modern v2 translation services.` | CLI help text |
| D25 | apps/cli/cmd/version.go:22-23 | `Short: "Print the version number of DevToolsCLI",` / ``Long: `All software has versions. This is DevToolsCLI's` `` | Help text for `version` (the *output* line is C3) |
| D26 | apps/cli/install.sh:5-6 | `# DevTools CLI Installer Script` / `# This script downloads and installs the DevTools CLI from GitHub releases` | Comments in a contract file (safe to edit text) |
| D27 | apps/cli/install.sh:127, 213, 266 | `"Downloading DevTools CLI ${version} for ${platform}..."`, `"DevTools CLI installed successfully to $install_path"`, `"DevTools CLI Installer"` | Installer stderr messages (text only; the file itself is a contract) |
| D28 | actions/run-flows/action.yml:1 | `name: 'DevTools Run Flows'` | Action display name (Step 5) |
| D29 | actions/run-flows/action.yml:2 | `description: 'Run a DevTools YAML flow with the released devtoolscli CLI and publish JSON/JUnit reports plus a job summary.'` | Action description. Keep the `devtoolscli` mention accurate |
| D30 | actions/run-flows/action.yml:13 | `description: "devtoolscli release to install: 'latest' or a release tag, e.g. cli@1.0.3."` | Input *description* only (the input name and default are C-rows) |
| D31 | actions/run-flows/scripts/write-summary.sh:18, 20 | `heading='DevTools flow run — success'` / `heading='DevTools flow run — failed'` | **AMBIGUOUS**: job-summary Markdown that people read, but downstream scripts could scrape `$GITHUB_STEP_SUMMARY`. Low risk |
| D32 | actions/run-flows/scripts/download-cli.sh:2-3, 15, 54, 67, 70-71, 85; finalize.sh:32; run-flow.sh:2-3, 8, 32 | e.g. `"::error::Could not resolve the latest devtoolscli release…"`, `echo "Installed devtoolscli ${version_number} -> ${bin_path}"`, `echo "+ devtoolscli ${args[*]}"` | Log/comment text naming the real binary. Must keep naming `devtoolscli` accurately, or say "Stresseur CLI (devtoolscli)" |
| D33 | README.md:7, 26, 37, 43, 62, 96, 104 | `<h1 align="center">DevTools</h1>`, `DevTools gives developers complete control…`, `DevTools combines…`, `The DevTools interface…`, `Install the DevTools CLI…`, `The DevTools API Recorder extension…`, `…the main DevTools application…` | README prose (Step 6). Install commands on lines 65/71 are C-rows |
| D34 | actions/run-flows/README.md:3-4, 7, 24, 36, 103-104, 108 | `runs a DevTools .yamlflow.yaml file with the released devtoolscli binary…` | Action docs. The prose is display; the embedded names (`devtoolscli`, `.devtools-reports`, `devtools-cli-<version>-<os>-<arch>`, `$RUNNER_TEMP/devtools/bin`) must stay accurate |
| D35 | docs/cli.md:1, 5, 84 | `# DevTools CLI Guide`, `The DevTools CLI (devtoolscli) is…`, `no repo checkout of DevTools itself` | CLI docs prose. Command examples on lines 17-191 are contract refs (C-rows), to be updated per Step 4 with an "old name still works" note |
| D36 | CLAUDE.md:77, AGENTS.md:77, GEMINI.md:77 | `DevTools is a local-first, open-source API testing platform (Postman alternative)…` | Agent docs (Step 6 Naming note). GEMINI.md is a third copy the brief doesn't mention |
| D37 | LICENSE:189 | `Copyright 2026 DevTools` | **AMBIGUOUS**: legal notice, not UI. DevTools stays the umbrella/legal name, so recommend **keep** |
| D38 | tools/storybook/.storybook/Introduction.mdx:1; packages/server/docs/specs/GRAPHQL.md:5, 9, 32; HTTP.md:5 | `# DevTools Storybook`, `…first-class GraphQL request support to DevTools…`, `The HTTP system in DevTools…` | Internal developer docs. Optional, low priority |

Umbrella-brand references to **keep** (not old-product-name hits): `https://dev.tools/` (README.md:2; `.github/ISSUE_TEMPLATE/config.yml:4`),
`help@dev.tools` (`.github/ISSUE_TEMPLATE/bug-report.yaml:9`, `config.yml:5`, `docs/CODE-OF-CONDUCT.md:63`, `docs/CONTRIBUTING.md:34`),
README images on `dev.tools/_next/…` (README.md:41, 47), Figma link (`docs/CONTRIBUTING.md:145`).

Historical or planning text to **leave alone**: `apps/desktop/CHANGELOG.md` (27 lines) and `apps/cli/CHANGELOG.md` (15 lines,
nx-generated, mostly commit links to `the-dev-tools/dev-tools`); `docs/superpowers/specs/2026-08-08-load-testing-design.md`
(23 lines) and `docs/superpowers/plans/2026-08-08-stresseur-phase0-1.md` (5 lines); test fixtures
`packages/server/pkg/flow/node/nai/integration_providers_test.go:61,74` (`"Hello from DevTools!"`) and
`integration_tools_http_test.go:40` (`jdoe@devtools.local`).

---

## C. Identifier (keep)

These follow hard rule 2. Keep them byte-for-byte. The riskiest ones are flagged ⚠.

### C.1 Desktop packaging, userData, update feed

| # | file:line | Exact string | Reason |
|---|---|---|---|
| I1 ⚠ | apps/desktop/build.ts:11 | `name: 'DevTools-Studio',` (inside `extraMetadata`) | **Root identifier.** It is the packaged `package.json` `name`, and it drives: `app.getName()` → **userData dir**; the default **appId `com.electron.devtools-studio`** (`appInfo.js:~100`, prefix `com.electron.` from `ElectronFramework.js:88`) → macOS **CFBundleIdentifier**, Windows **NSIS GUID** (`UUID.v5(appId, 50e065bc-…)`, `NsisTarget.js:156`), uninstall registry key and AUMID; the default **productName** → `.app`/`.exe` names, NSIS install dir, artifact names; Linux executable `devtools-studio` (`linuxPackager.js:16`); updater cache dir `devtools-studio-updater` (`appInfo.js` `updaterCacheDirName`) |
| I2 ⚠ | (implicit, no literal) | appId `com.electron.devtools-studio` | Not written anywhere. It is derived from I1. If Step 1 adds an explicit `appId`, it must be exactly this string |
| I3 ⚠ | (implicit, no literal) | executable `DevTools-Studio.exe` / `DevTools-Studio.app` / `devtools-studio` (Linux) | Derived from productName/name. Step 1 must pin `executableName` if productName changes |
| I4 ⚠ | apps/desktop/src/main/index.ts:150-151 | `// 0.2.0 used "DevTools Studio" (space), 0.1.x used "DevTools".` / `const oldDirs = [nodePath.join(appData, 'DevTools Studio'), nodePath.join(appData, 'DevTools')];` | Legacy userData dirs for migration. Must stay. Note that `DevTools-Studio` (the current dir) is **not** in this list |
| I5 ⚠ | apps/desktop/src/main/index.ts:242 | `repo: 'the-dev-tools/dev-tools',` | **Update feed**: the repo slug the custom update provider reads (`update.ts:24-60, 88-97`) |
| I6 ⚠ | apps/desktop/src/main/update.ts:28, 34, 51, 94 (no old name in literal) | `` `https://raw.githubusercontent.com/${options.repo}/refs/heads/main/${options.project.path}/package.json` ``, `` `…/releases/tags/${options.project.name}@${version}` ``, `` `latest-${process.platform}-${process.arch}.yml` ``, `` `…/releases/download/${…name}@${updateInfo.version}/` `` | **Update feed/channel contract**. Listed because it defines what the pre-release question hinges on (A4) |
| I7 | apps/desktop/src/renderer/main.tsx:37 | `` `https://api.github.com/repos/the-dev-tools/dev-tools/releases/tags/desktop@${version}` `` | Fetches release notes for the update screen. Same repo slug |
| I8 | apps/desktop/build.ts:13 | `...libFiles('@the-dev-tools/server'), ...libFiles('@the-dev-tools/worker-js')` | Package names that decide which files are bundled into the asar |
| I9 | apps/desktop/src/main/index.ts:179, 212, 382 | `import.meta.resolve('@the-dev-tools/server')` / `('@the-dev-tools/worker-js')` | Runtime resolution of the bundled Go server and worker by package name |

### C.2 IPC endpoints and internal hostnames (shared between Electron, Go server, JS worker, auth)

| # | file:line | Exact string | Reason |
|---|---|---|---|
| I10 | apps/desktop/src/main/index.ts:248 | `path.join(os.tmpdir(), 'the-dev-tools', 'server.socket')` | UDS path. Must match the server |
| I11 | apps/desktop/src/main/index.ts:249 | `'\\\\.\\pipe\\the-dev-tools_server.socket'` | Windows named pipe. Must match the server |
| I12 | apps/desktop/src/main/index.ts:262 | `'http://the-dev-tools:0/'` | Virtual host for `server://` proxying |
| I13 | packages/server/internal/api/api_unix.go:17, 22 | `filepath.Join(os.TempDir(), "the-dev-tools", "server.socket")` / `"worker-js.socket"` | Server-side socket paths |
| I14 | packages/server/internal/api/api_windows.go:17, 22 | `` `\\.\pipe\the-dev-tools_server.socket` `` / `` `\\.\pipe\the-dev-tools_worker-js.socket` `` | Server-side pipes |
| I15 | packages/server/internal/api/api.go:74, 89 | `Addr: "the-dev-tools:0",` / `defaults to /tmp/the-dev-tools/server.socket` | Server listener host / doc comment |
| I16 | packages/server/cmd/serverrun/serverrun.go:445 | `jsBaseURL = "http://the-dev-tools:0"` | Server → worker base URL |
| I17 | packages/worker-js/src/main.ts:34, 42 | `'\\\\.\\pipe\\the-dev-tools_worker-js.socket'` / `path.join(os.tmpdir(), 'the-dev-tools')` | Worker listener |
| I18 | packages/auth/src/adapter.ts:21 | `baseUrl: 'http://the-dev-tools:0',` | Auth adapter → server |
| I19 | packages/auth/src/auth-effect.ts:36 | `path.resolve(os.tmpdir(), 'the-dev-tools', 'server.socket')` | Auth socket default |
| I20 | packages/auth/src/adapter.test.ts:23 | `path.resolve(os.tmpdir(), 'the-dev-tools', 'test.auth-adapter.server.socket')` | Test socket |
| I21 | apps/cli/internal/runner/jsrunner.go:43, 55, 85 | `"devtools-worker-*.cjs"`, `"%s/devtools-cli-worker-%d.sock"`, `"http://devtools-cli:0"` | CLI temp file, socket and virtual host for the embedded JS worker |

### C.3 Auth, tokens, internal MIME types

| # | file:line | Exact string | Reason |
|---|---|---|---|
| I22 ⚠ | packages/server/pkg/stoken/stoken.go:33, 35 | `Issuer: "devtools-server",` / `Audience: jwt.ClaimStrings{"devtools-server"},` | JWT iss/aud. Changing them invalidates tokens already issued or stored. (Test mirror at `internal/api/middleware/mwauth/mwauth_test.go:172, 174`) |
| I23 | packages/auth/src/adapter.ts:90-91 | `adapterId: '@the-dev-tools/auth-adapter',` / `adapterName: 'DevTools Auth Adapter',` | Better Auth adapter id. The name only shows in logs, but it sits next to the id: keep both |
| I24 | packages/client/src/features/expression/code-mirror/drop-extension.ts:9; packages/client/src/features/expression/reference.tsx:93 | `'application/x-devtools-reference'` | Internal drag-and-drop MIME key. Both sides must match |

### C.4 Package names (hard rule 2: package `name` fields)

| # | file:line | Exact string | Reason |
|---|---|---|---|
| I25 | package.json:2 | `"name": "the-dev-tools",` | Workspace root name. Also referenced by `syncpack.config.mjs:23` `packages: ['!the-dev-tools']` |
| I26 | apps/desktop/package.json:2; apps/cli/package.json:2; apps/api-recorder-extension/package.disabled.json:2 | `"@the-dev-tools/desktop"`, `"@the-dev-tools/cli"`, `"@the-dev-tools/api-recorder-extension"` | App package names (nx project resolution, version plans) |
| I27 | packages/auth/package.json:2; packages/client/package.json:2; packages/server/package.json:2; packages/spec/package.json:2; packages/ui/package.json:2; packages/worker-js/package.json:2 | `"@the-dev-tools/auth"` … `"@the-dev-tools/worker-js"` | Library package names |
| I28 | tools/eslint/package.json:2; tools/gha-scripts/package.json:2; tools/spec-lib/package.json:2; tools/storybook/package.json:2 | `"@the-dev-tools/eslint-config"`, `…/gha-scripts`, `…/spec-lib`, `…/storybook` | Tool package names |
| I29 | all `"@the-dev-tools/*": "workspace:^"` dependency lines (package.json:6, 16; apps/desktop/package.json:14-15, 28-30; apps/cli/package.json:8-9; packages/*/package.json; tools/*/package.json; package.disabled.json:27-28) | `"@the-dev-tools/…": "workspace:^"` | Workspace links |
| I30 | 512 import lines in 128 `.ts/.tsx/.tsp/.mjs/.mdx` files; `tools/eslint/config.ts:123` `internalPattern: ['^@the-dev-tools/.*', …]`; `packages/spec/tspconfig.yaml:4-9`; `tools/spec-lib/src/*/lib.ts` `name: '@the-dev-tools/spec-lib/…'`; `tools/spec-lib/src/ai-tools/emitter.tsx:279` | `'@the-dev-tools/…'` | npm scope in imports, emitter names and codegen output |

### C.5 Go module paths and Go identifiers (hard rule 2: Go module paths)

| # | file:line | Exact string | Reason |
|---|---|---|---|
| I31 ⚠ | apps/cli/go.mod:1; packages/db/go.mod:1; packages/server/go.mod:1; packages/spec/go.mod:1; packages/auth-lib/go.mod:1; tools/benchmark/go.mod:1; tools/go-tool/go.mod:1; tools/modmigrate/go.mod:1; tools/norawsql/go.mod:1; tools/notxread/go.mod:1 | `module github.com/the-dev-tools/dev-tools/<path>` | Go module paths. Also every `require`/`replace` line in those go.mod files (apps/cli/go.mod:10-12, 84-86; packages/db/go.mod:9, 37; packages/server/go.mod:21-23, 86-88; tools/go-tool/go.mod:9-10, 207-208, 260-261) |
| I32 | 3,147 import lines in 694 `.go` files | `"github.com/the-dev-tools/dev-tools/…"` | Go imports (rolled up) |
| I33 ⚠ | apps/cli/taskfile.yaml:110 | `-ldflags "-X github.com/the-dev-tools/dev-tools/apps/cli/cmd.version=v{{.VERSION_RESOLVED}}"` | Functional: if the module path changes, the version injection silently stops working |
| I34 | packages/spec/tspconfig.yaml:11; packages/db/pkg/sqlc/sqlc.yaml (131 lines, e.g. :29); packages/server/.golangci.yml:44, 46, 48; packages/server/project.json:82-83 | `goPackage: 'github.com/the-dev-tools/dev-tools/packages/spec/dist/buf/go'`, `import: 'github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap'`, `- pkg: github.com/…/mhttp`, `go tool norawsql github.com/the-dev-tools/dev-tools/packages/server/...` | Codegen config, lint config and lint targets that embed the module path. 12 of the 13 tracked generated files under `packages/db/pkg/sqlc/gen/` carry these import paths |
| I35 | packages/db/db.go:1 (+ ai_test.go:1, flow_node_http_test.go:1, verification_test.go:1); 85 refs in 26 files (e.g. `devtoolsdb.TxnRollback`); CLAUDE.md/AGENTS.md/GEMINI.md:87 | `package devtoolsdb` | Go package name |
| I36 | tools/spec-lib/src/core/main.tsp:3; protobuf/main.tsp:3; tanstack-db/main.tsp:4, 15-16; ai-tools/main.tsp:4; core/index.tsx:24, 30, 43; protobuf/lib.ts:26, 29; protobuf/emitter.tsx:119-124, 178, 499, 569, 594, 603, 694; tanstack-db/lib.ts:26, 69; ai-tools/lib.ts:10; `using DevTools;` in 14 files under packages/spec/api/*.tsp; packages/spec/api/main.tsp:37 `@DevTools.project` | `namespace DevTools…`, `'DevTools.Protobuf.Validate.Field.Required'`, … | TypeSpec library namespace, resolved by string. It is **not** on the wire: the API namespace is `Api` (main.tsp:38), so proto packages are `api.*` |

### C.6 Dev/test-only identifiers, lockfile and generated output

| Where | String | Reason |
|---|---|---|
| packages/server/pkg/mutation/replay_dev.go:28, 30 (doc: packages/server/docs/specs/MUTATION.md:410-462) | `os.Getenv("DEVTOOLS_REPLAY_DIR")`, `"devtools-replay"` | Dev-build-only env var and temp dir. Keep |
| apps/cli/project.json:69-70, 73; apps/cli/test/yamlflow/integration_yamlflow_test.go:13, 19, 23, 30, 45, 47, 51 | `DEVTOOLS_CLI_BIN`, `devtools-cli-test`, `DEVTOOLS_MODE=cli` | Integration-test harness. Keep |
| .gitignore:19 | `apps/cli/devtoolscli` | Local build output name. Keep |
| packages/server/docs/specs/BACKEND_ARCHITECTURE_V2.md:43-44, 73-74; packages/server/testing.md:147; tools/modmigrate/main.go:2 | `"the-dev-tools/db/pkg/sqlc/gen"` etc. | Old short import paths in docs and the migration tool. Keep |
| packages/server/pkg/translate/tpostmanv2/tpostmanv2.go:1111 (+ real_world_test.go:48) | `Value: "https://dev.tools/",` | Default placeholder value written into imported Postman data. It is persisted user data and points at the umbrella URL. Keep |
| packages/client/src/shared/ui/dashboard.tsx:49-50 | `href='https://github.com/the-dev-tools/dev-tools'`, `img.shields.io/github/stars/the-dev-tools/dev-tools` | Repo link and badge. Keep (the repo slug is unchanged) |
| packages/server/pkg/flow/node/nfor/nfor_test.go:284 | `https://github.com/the-dev-tools/dev-tools/issues/42` | Issue link in a comment |
| **pnpm-lock.yaml** | 26 `@the-dev-tools/*` workspace importer/link entries | Lockfile carries the package names. It regenerates only if the names change, which they must not. No old-name hits in `go.sum`/`go.work.sum`. `go.work` uses relative paths only |
| **Generated, not tracked** | `packages/spec/dist/**` (buf Go/TS, tanstack-db) | Embeds `goPackage` (I34) and `@the-dev-tools/*` imports. Regenerated from spec; no manual edits |
| **Generated at build (not in repo)** | packaged `app-update.yml`, `latest-*.yml`, NSIS script defines | Carry `DevTools-Studio` artifact file names, updater cache dir `devtools-studio-updater`, `APP_GUID` and `PRODUCT_NAME` |

---

## D. Contract (keep working)

| # | file:line | Exact string | Reason |
|---|---|---|---|
| C1 ⚠ | apps/cli/install.sh:17 | `BINARY_NAME="devtools"` | Installed CLI name for every `curl … install.sh \| bash` user |
| C2 ⚠ | apps/cli/cmd/root.go:13 | `Use: "devtoolscli",` | **AMBIGUOUS**: cobra's root command name. It is shown in usage/help and used by `completion` script generation. It is not argv dispatch, so the binary works under any file name |
| C3 ⚠ | apps/cli/cmd/version.go:25 | `fmt.Printf("DevToolsCLI %s\n", version)` | **AMBIGUOUS / CONFLICT**: this is the machine-readable version line (`DevToolsCLI v1.1.1`). Brief Step 4 ("version text says Stresseur CLI") conflicts with hard rule 3. The CLI has no `--version` flag |
| C4 ⚠ | apps/cli/cmd/root.go:29, 44, 64, 72; apps/cli/.devtools.yaml (tracked, `{}`) | `ConfigFileName = ".devtools"`, `"config file (default is $HOME/.devtools.yaml)"`, `viper.SetConfigName(".devtools")`, `home + "/.devtools.yaml"` | CLI config file. It is **auto-created in $HOME on first run**, so existing users have it |
| C5 ⚠ | apps/cli/main.go:12, 21, 35, 44 | `EnvDevToolsMode = "DEVTOOLS_MODE"` | The only user-facing `DEVTOOLS_*` env var (documented in docs/cli.md:124-162). Also the Electron → binary switch: apps/desktop/src/main/index.ts:194 `DEVTOOLS_MODE: 'server'`, :394 `DEVTOOLS_MODE: 'cli'` |
| C6 ⚠ | apps/cli/taskfile.yaml:110 | `-o {{.BIN_DIR}}/devtools-cli-{{.VERSION_RESOLVED}}-{{.OUTPUT_PLATFORM}}{{.SUFFIX}}` | **CLI release asset name** (`devtools-cli-<ver>-<platform>[.exe]`) |
| C7 ⚠ | apps/cli/install.sh:123 | `local binary_name="devtools-cli-${version}-${platform}${binary_suffix}"` | Consumer of C6 |
| C8 ⚠ | actions/run-flows/scripts/download-cli.sh:64 | `asset_name="devtools-cli-${version_number}-${platform}"` | Consumer of C6 |
| C9 ⚠ | apps/desktop/build.ts:9 | `artifactName: '${productName}-${version}-${platform}-${arch}.${ext}',` | **Desktop asset name** → `DevTools-Studio-<ver>-<platform>-<arch>.<ext>`. It depends on productName, so the prefix must be hardcoded before any productName change |
| C10 | tools/gha-scripts/src/cli.ts (`upload-electron-release-assets`) | `` name: `latest-${process.platform}-${process.arch}.yml` `` | Update-metadata asset name that the updater reads (I6) |
| C11 | README.md:88-90 | `DevTools-{version}-darwin-{arch}.dmg` / `DevTools-{version}-win32-{arch}.exe` / `DevTools-{version}-linux-{arch}.AppImage` | Documented asset names. **Already wrong** (real: `DevTools-Studio-…`, Linux arch `x86_64`) |
| C12 ⚠ | apps/cli/install.sh:15-16 | `REPO_OWNER="the-dev-tools"` / `REPO_NAME="dev-tools"` | Release source for the installer (URLs at :82, :93, :102, :124, :151) |
| C13 | apps/cli/install.sh:152 | `local temp_checksum="/tmp/devtools-checksums.txt"` | Temp file (the checksum asset is never published; see A3) |
| C14 ⚠ | README.md:65, 71 | `curl -fsSL https://sh.dev.tools/install.sh \| bash` / `wget -qO- https://sh.dev.tools/install.sh \| bash` | Public install one-liners. Hosted outside this repo |
| C15 | docs/cli.md:12, 115 | `curl -fsSL https://raw.githubusercontent.com/the-dev-tools/dev-tools/main/apps/cli/install.sh \| bash` | Public install one-liner (raw path) |
| C16 | README.md:82, 86 | `https://github.com/the-dev-tools/dev-tools/releases` | Download URL |
| C17 | apps/desktop/src/main/index.ts:48 | `shell.openExternal('https://dev.tools/download')` | Download page URL (plain link, allowed under rule 5) |
| C18 ⚠ | actions/run-flows/action.yml:3 | `author: 'the-dev-tools'` | Action metadata. Low risk, but it identifies the owner |
| C19 ⚠ | actions/run-flows/action.yml:19 | `default: '.devtools-reports'` | **Action input default.** Users' later steps read reports from this path |
| C20 | actions/run-flows/scripts/run-flow.sh:24; finalize.sh:16-17; write-summary.sh:13 | `report_dir="${REPORT_DIR:-.devtools-reports}"` etc. | Script-side copies of C19 |
| C21 | actions/run-flows/action.yml:5-34 (no old name) | inputs `file`, `flow`, `version`, `report-dir`, `fail-on-error`; outputs `json-report`, `junit-report`, `success` | Action interface. Keep exactly |
| C22 ⚠ | actions/run-flows/README.md:12, 65, 82; docs/cli.md:94 | `uses: the-dev-tools/dev-tools/actions/run-flows@main` | Action path and ref consumers use |
| C23 ⚠ | actions/run-flows/scripts/download-cli.sh:19-20, 49-61 | `REPO_OWNER='the-dev-tools'`, `REPO_NAME='dev-tools'`, `git ls-remote --tags --refs "${REPO_URL}.git" 'cli@*' \| … \| sort -V \| tail -n1` | Release source, and the "latest" resolution that ignores the pre-release flag (A4) |
| C24 | actions/run-flows/scripts/download-cli.sh:75-76 | `bin_dir="${RUNNER_TEMP:-/tmp}/devtools/bin"` / `bin_path="${bin_dir}/devtoolscli"` | Installed path and binary name inside the Action. `bin` is written to the step output |
| C25 | actions/run-flows/scripts/run-flow.sh (whole) | `devtoolscli flow run <file> [flow] --report console --report json:… --report junit:…` | The CLI command the Action calls (Step 5: keep identical) |
| C26 | docs/cli.md:17, 24, 53, 66, 68, 78, 106-107, 116, 130-131, 149-155, 184, 191 | `devtoolscli version`, `devtoolscli flow run …`, `devtools flow run …`, `go build -tags cli -o devtools .`, `DEVTOOLS_MODE=cli ./devtools …` | Documented commands. Step 4 adds `stresseur` alongside them; don't delete these |
| C27 | (no old name) `desktop@<ver>` / `cli@<ver>` tag scheme; `tools/gha-scripts/src/repository.ts` | tag split on `@` | Release tags consumed by the updater, install.sh and the Action |
| C28 | (no old name) YAML flow format / `.yamlflow.yaml` | none | No DevTools string inside the export format (no hits under `packages/server/pkg/translate/yamlflow*` or the exporter) |
| C29 | (no old name) `apps/cli/internal/reporter/reporter.go:249-264` | `<testsuites>`, `<testsuite>`, `<testcase>`, JSON report | No tool name in JSON/JUnit. Leave the reporter untouched |
| C30 | `install.ps1`, `scoop.json`, `flake.nix` | none | Scanned: **zero** old-name hits, and none of them is a CLI installer (see A4/A5). There is no contract to preserve here, and nothing to add aliases to |

---

## E. Not our name (do not touch)

These match the search, but they refer to Chrome/React/TanStack/Electron developer tools:

- `electron-devtools-installer`, `REACT_DEVELOPER_TOOLS`, `mainWindow.webContents.openDevTools()`, `// Open the DevTools.`
  (apps/desktop/src/main/index.ts:104-115, 134; apps/desktop/package.json).
- `@tanstack/react-query-devtools`, `@tanstack/react-router-devtools`, `@tanstack/router-devtools-core`, `@tanstack/query-devtools`,
  `@hookform/devtools`, `ReactQueryDevTools*`, `TanStackRouterDevTools*`, `ReactScanDevTools`, `ShowDevToolsContext`,
  `DevToolsProvider`, `window.toggleDevTools` (packages/client/src/app/dev-tools.tsx and `__root.tsx:16-17`, plus their
  `pnpm-lock.yaml` entries). The file name `packages/client/src/app/dev-tools.tsx` belongs here too.
- `devtools-protocol`, `Devtools.Protocol.Network.*` (Chrome DevTools Protocol types in the extension).
