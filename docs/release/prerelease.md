# Private pre-release (desktop + CLI)

How to put a pre-release build of DevTools Studio (desktop) and the CLI on the
owner's machines without any user, CI pipeline or installer picking it up, and
how to clean up afterwards so the next real release from `main` is unaffected.

Written for the Oct 2026 rename (pre-release around Oct 5, real release Oct 13),
but the procedure is general. File/line references are to `main` after the
"Keep pre-releases away from users" change; Nx references are to the pinned
`nx@22.5.4` in `node_modules/nx/src/command-line/release/`.

## What "private" means here

The repo is public. Nothing below makes a GitHub release secret: while a
published pre-release exists, anyone can see it on the Releases page, and
GitHub notifies people who watch the repo for releases. What this procedure
guarantees is that **nothing installs or offers it automatically**:

| Consumer                                                             | How it picks a version                                                                                               | Why a pre-release is ignored                                                                                                                                                                                                               |
| -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Installed desktop apps (auto-update)                                 | Version in `main`'s `apps/desktop/package.json`, then tag `desktop@<that>` (`apps/desktop/src/main/update.ts:32-38`) | The pre-release version never lands on `main` (step 1). New builds also refuse pre-releases unless the running app is one (`update.ts:59`). **Apps already installed do not have that guard**, so the first reason is the one that counts. |
| `install.sh` (no `-v`)                                               | Version in `main`'s `apps/cli/package.json` (`apps/cli/install.sh:105`)                                              | Same as above. Also falls back to the newest stable `cli@` tag if `main` ever carries a pre-release (`install.sh:118`).                                                                                                                    |
| `actions/run-flows` with `version: latest`                           | Highest `cli@*` git tag (`actions/run-flows/scripts/download-cli.sh:54`)                                             | Pre-release tags are skipped (`scripts/select-latest-cli-tag.sh:17`). **Old action refs (`@cli@1.1.0`, `@cli@1.1.1`, `@desktop@1.1.0`, older SHAs) do not skip them**, so the CLI pre-release gets no `cli@` tag at all (step 6).          |
| GitHub "Latest release" badge / `releases/latest`                    | GitHub's latest non-pre-release                                                                                      | Nx creates the release with `prerelease: true` for any semver pre-release (`utils/remote-release-clients/remote-release-client.js:78`, `github.js:271`).                                                                                   |
| `install.ps1`                                                        | n/a                                                                                                                  | It is the vendored Scoop installer; it never resolves a DevTools release.                                                                                                                                                                  |
| `update-scoop.yaml`, `release-*.yaml`                                | n/a                                                                                                                  | All are `workflow_dispatch`/`schedule` only; none triggers on tags or releases. Only `release.yaml` dispatches them.                                                                                                                       |
| dev.tools download page (`devtools-website` repo, outside this repo) | Newest `desktop@*` entry from the GitHub releases API                                                                | **Not safe today**: it does not skip pre-releases. See pre-flight step 0.                                                                                                                                                                  |

## Pre-flight

0. **Website.** `devtools-website/components/ui/DownloadButton.tsx` (used by
   `/download`) takes the first `desktop@*` release from
   `GET /repos/the-dev-tools/dev-tools/releases`, which includes pre-releases.
   Before cutting a desktop pre-release, make it skip `rel.prerelease` (and
   `rel.draft`) and deploy that. If that can't happen in time, step 5 (draft it
   right after the build) is the only protection, and the page serves the
   pre-release while assets upload (tens of minutes).
1. This change is merged to `main`. It adds two guards that make the main
   mistake fail loudly:
   - `release.yaml` refuses to run from `main` when a version plan asks for a
     `pre*` bump (`.github/workflows/release.yaml:39`).
   - `check.yaml` fails any PR to (or push on) `main` whose
     `apps/*/package.json` version has a pre-release suffix
     (`.github/workflows/check.yaml:65`), so the pre-release branch can't be
     merged by accident.
2. Decide the bump for the **real** Oct 13 release (for example `minor`:
   desktop `1.1.0 -> 1.2.0`). The pre-release uses the matching `pre*` bump so
   it sorts just below the real version.
3. Back up your own desktop data. The pre-release uses the same app identity
   and data directory as your stable install (`userData` is named after
   `extraMetadata.name` in `apps/desktop/build.ts:10`, so by default `~/Library/Application Support/DevTools-Studio`,
   `%APPDATA%\DevTools-Studio`, `~/.config/DevTools-Studio`). Any database
   migration it runs on `state.db` may not be readable by 1.1.0 afterwards.

## Desktop

1. **Create the pre-release branch.** Never use `main`.

   ```bash
   git fetch origin
   git switch -c prerelease/rename origin/<branch-or-commit-with-the-rename>
   ```

   Why this is safe: `gha-scripts release` runs `releaseVersion` then
   `releaseChangelog` (`tools/gha-scripts/src/cli.ts:111-119`). Because
   `createRelease` is set in `nx.json`, Nx forces a git push
   (`config/config.js:106-107`), and it runs `git push --follow-tags --no-verify
--atomic` with no refspec (`utils/git.js:373-382`), which pushes only the
   checked-out branch (the one you dispatch on) plus the new tag. `main` is
   untouched.

2. **Add exactly one version plan**, on the branch only:

   ```bash
   ls .nx/version-plans/ 2>/dev/null   # must contain no other desktop/cli plans
   mkdir -p .nx/version-plans
   cat > .nx/version-plans/rename-prerelease.md <<'EOF'
   ---
   desktop: preminor
   ---

   Pre-release build for internal testing.
   EOF
   git add .nx/version-plans/rename-prerelease.md
   git commit -m "chore: desktop pre-release plan"
   git push -u origin prerelease/rename
   ```

   - Use `preminor` if Oct 13 is a `minor`, `premajor` for `major`,
     `prepatch` for `patch`. Version plans only accept semver release types
     (`config/version-plans.js:222-224`).
   - `gha-scripts` passes no `--preid` (`cli.ts:109`), so the version is
     `semver.inc('1.1.0', 'preminor', '')` = **`1.2.0-0`** (verified), not
     `-rc.N`. Nx reads the current version from `package.json` on the branch
     (`currentVersionResolver` defaults to `disk`, `utils/release-graph.js:515-519`).
   - Tag: **`desktop@1.2.0-0`** (independent projects use
     `{projectName}@{version}`, `config/config.js:115`).
   - The message becomes the public release notes. Keep it neutral.
   - Remove any other plan on this branch (for example a `desktop: minor` plan
     from the rename PR). Otherwise the bump can come out stable (`1.2.0`),
     which would publish a normal release and take the tag the real release
     needs.

3. **Run the Release workflow from the branch, desktop only:**

   ```bash
   gh workflow run release.yaml --ref prerelease/rename -f desktop=true
   ```

   Leave `cli`, `web` and `api-recorder-extension` unset. What happens:
   commit "Version projects" plus tag `desktop@1.2.0-0` pushed to the branch.
   A GitHub release is created with `prerelease: true` and `make_latest: legacy`,
   so it never becomes "Latest" (`github.js:266-274`). `release-electron-builder.yaml`
   is dispatched at the tag (`cli.ts:121-135`) and uploads the signed and
   notarized installers plus `latest-*.yml` to that release (`cli.ts:161`).
   Those `.yml` files are only read by the updater after it has already picked
   a version from `main`'s `package.json`, so they don't matter here.

4. **Watch it:** `gh run list --workflow release-electron-builder.yaml -L 1`, then
   `gh run watch <id>`.

5. **Download, then hide the release** (as soon as the build is green):

   ```bash
   V=1.2.0-0
   mkdir -p ~/devtools-prerelease && cd ~/devtools-prerelease
   gh release download "desktop@$V" -R the-dev-tools/dev-tools \
     -p "DevTools-Studio-$V-darwin-arm64.dmg" \
     -p "DevTools-Studio-$V-win32-x64.exe" \
     -p "DevTools-Studio-$V-linux-x86_64.AppImage"
   gh release edit "desktop@$V" -R the-dev-tools/dev-tools --draft=true
   ```

   A draft doesn't show up for anyone without push access, including the
   anonymous releases API the website uses. Asset names follow
   `artifactName` (`apps/desktop/build.ts:9`). Use `darwin-x64` on an Intel Mac.

6. **Install** (after the backup in pre-flight step 3):
   - **macOS:** open the `.dmg` and drag DevTools Studio to Applications,
     replacing the stable app. It is signed and notarized.
   - **Windows:** run `DevTools-Studio-1.2.0-0-win32-x64.exe`. It is the
     signed NSIS installer and upgrades the existing install.
   - **Linux:** `chmod +x DevTools-Studio-1.2.0-0-linux-x86_64.AppImage && ./DevTools-Studio-1.2.0-0-linux-x86_64.AppImage`.

   Auto-update on your pre-release build: `allowPrerelease` is on because the
   app version is a pre-release (electron-updater `AppUpdater.js:218`). It still
   reads `main`'s version (`1.1.0`), and that is lower, so nothing is offered
   (no downgrades). On Oct 13, `main` becomes `1.2.0`, which is higher than
   `1.2.0-0`, so you get the normal update. If the real version ends up lower
   than the pre-release, reinstall by hand.

Another iteration: on the same branch, add a plan with `desktop: prerelease`
(`1.2.0-0 -> 1.2.0-1`) and repeat steps 3–6.

## CLI (no tag, no GitHub release)

Don't run the Release workflow with `cli=true` for a pre-release. Any
`cli@*` tag is visible to `git ls-remote`, and every `actions/run-flows` ref
cut before this change (`cli@1.1.0`, `cli@1.1.1`, `desktop@1.1.0`, and the
SHAs users pin to, as the action README suggests) still resolves
`version: latest` with plain `sort -V`, which ranks `cli@1.2.0-rc.1` above
`cli@1.2.0`. Build the CLI yourself from the same branch instead. It is pure Go
with `CGO_ENABLED=0` and cross-compiles (`apps/cli/taskfile.yaml`, `build:release`):

```bash
cd <repo checkout on prerelease/rename>
direnv allow
for p in darwin-arm64 linux-x64 win32-x64; do
  VERSION=1.2.0-rc.1 PLATFORM=$p pnpm nx run cli:build:release --skip-nx-cache
done
ls apps/cli/dist/
# devtools-cli-1.2.0-rc.1-darwin-arm64  devtools-cli-1.2.0-rc.1-linux-x64  devtools-cli-1.2.0-rc.1-win32-x64.exe
```

(Verified: this produces a Mach-O arm64, an ELF x86-64 and a PE32+ x86-64
binary, and `devtools-cli-1.2.0-rc.1-darwin-arm64 version` prints
`DevToolsCLI v1.2.0-rc.1`.) Any `VERSION` string works because nothing is
published. Pick one that makes it obvious in `devtools version` output.

Install it next to the stable CLI, not over it:

- **macOS / Linux:** `install -m 755 apps/cli/dist/devtools-cli-1.2.0-rc.1-<platform> ~/.local/bin/devtools-next`
  (a binary built on your own Mac isn't quarantined. If you copy it from
  another machine, run `xattr -d com.apple.quarantine ~/.local/bin/devtools-next`.)
- **Windows:** copy `devtools-cli-1.2.0-rc.1-win32-x64.exe` to a folder on
  `PATH` as `devtools-next.exe`, then `Unblock-File .\devtools-next.exe`. The
  binary is unsigned, so SmartScreen may ask once.

## Cleanup (after the Oct 13 release is out)

1. **Don't merge `prerelease/rename`.** Its "Version projects" commit sets
   `apps/desktop/package.json` to `1.2.0-0`, and `check.yaml` will fail the PR.
   If the pre-release found bugs, cherry-pick the fixes onto `main` but leave
   out the version plan, the "Version projects" commit and the `CHANGELOG.md` entry.
2. Make the real release from `main` as usual (`CLAUDE.md` → Releasing).
   The pre-release doesn't change it: Nx computes the next version from
   `main`'s `package.json` (disk resolver), and `desktop@1.2.0` doesn't collide
   with `desktop@1.2.0-0`.
3. Delete the pre-release release(s) and tag(s):

   ```bash
   gh release delete desktop@1.2.0-0 -R the-dev-tools/dev-tools --cleanup-tag --yes
   # if the release was already removed but the tag remains:
   git push origin :refs/tags/desktop@1.2.0-0
   ```

4. Delete the branch: `git push origin --delete prerelease/rename`.
5. Verify:

   ```bash
   git fetch origin --prune --prune-tags
   git show origin/main:apps/desktop/package.json | jq -r .version   # stable, e.g. 1.2.0
   git show origin/main:apps/cli/package.json | jq -r .version       # stable
   git ls-remote --tags origin 'desktop@*-*' 'cli@*-*'               # prints nothing
   gh release list -R the-dev-tools/dev-tools -L 10                  # no Pre-release / Draft rows left
   ```

6. Move your machine back to stable builds: accept the desktop auto-update to
   the real release (or reinstall from it) and remove `devtools-next`.

## If a pre-release reaches `main` by mistake

Act right away. Every installed desktop app checks `main` on startup.

1. Push a commit to `main` that restores the previous stable versions in
   `apps/*/package.json`. This is what installed apps and `install.sh` read.
2. `gh release edit <tag> --draft=true` (or delete it with `--cleanup-tag`).
3. Anyone who already updated has the pre-release. Because they won't be
   downgraded automatically, tell them to reinstall the stable build.
