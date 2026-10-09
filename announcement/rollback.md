# Rename-day rollback runbook

For the Oct 13, 2026 rename of DevTools Studio / DevTools CLI to **Stresseur
Studio** / **Stresseur CLI** (desktop currently `1.1.3`, CLI currently
`1.2.2`, API Recorder extension currently `0.4.10` as of Oct 9, 2026 — see
pre-flight step 1 below for how to get the exact numbers on the day).

**Owner's rule:** if something breaks, delete the new release or set it back
to draft so nothing serves it, then ship a fix. This runbook turns that into
exact steps per distribution surface, because "delete the release" does not
mean the same thing on every surface — on some it is sufficient by itself, on
others it does nothing at all.

This is a document for the human owner to execute by hand. Nothing in it was
run as part of writing it: no release, tag, draft state, or workflow in this
repository was touched to produce this runbook.

Every claim below is cited to the file/line it was verified against in this
checkout. Anything that depends on code that does not exist yet (the rename
PR itself) or on a third-party dashboard this checkout cannot see is marked
**UNVERIFIED**.

## Pre-flight: find what "stable" means right now

Before touching anything, capture the versions and commit you are rolling
back *to*. Do this first — every section below refers back to it.

```bash
# The commit that bumped versions for the rename release is titled "Version
# projects" (tools/gha-scripts/src/cli.ts:112, the gha-scripts release command's
# hard-coded gitCommitMessage). Find it and the commit right before it:
git fetch origin main
git log --oneline -n 10 origin/main
# Last-good versions, i.e. the parent of the "Version projects" commit:
git show <version-projects-commit>^:apps/desktop/package.json | jq -r .version
git show <version-projects-commit>^:apps/cli/package.json | jq -r .version
git show <version-projects-commit>^:apps/api-recorder-extension/package.json | jq -r .version
```

As of Oct 9, 2026 on `main` (before any rename commit), these are
`1.1.3` / `1.2.2` / `0.4.10` respectively (`apps/desktop/package.json:5`,
`apps/cli/package.json:4`, `apps/api-recorder-extension/package.json:5`).
Treat those three numbers as placeholders below (`<old-desktop>`,
`<old-cli>`, `<old-ext>`) and substitute the real parent-commit values on the
day, in case another patch release lands between now and Oct 13.

---

## 1. Desktop auto-update (`apps/desktop/src/main/update.ts`, `index.ts`)

**Mechanism, verified from code:**

1. `CustomUpdateProvider.getLatestVersion` (`update.ts:89-105`) calls
   `getUpdateInfo`, which:
   - Fetches `apps/desktop/package.json` straight from `main` via
     `raw.githubusercontent.com` (`update.ts:30-35`) to get a version string.
   - Fetches `https://api.github.com/repos/<repo>/releases/tags/desktop@<that
     version>` (`update.ts:37-52`).
   - If that second call fails for any reason (including a 404), the whole
     `Effect` fails, and `getLatestVersion`'s `onFailure` branch returns
     `{ ..., version: this.updater.currentVersion.raw }` — i.e. "no update"
     (`update.ts:95-104`).
2. So the updater needs **both** a version string on `main` **and** a
   publicly-visible release at tag `desktop@<that version>`. Either one being
   wrong is enough to stop the offer.
3. A GitHub release that has been edited back to `draft` is not returned by
   the unauthenticated `releases/tags/<tag>` endpoint this code calls (GitHub
   serves drafts only to callers with push access to the repo) — so **yes,
   setting the release to draft is sufficient by itself** to stop
   `update.ts` from offering it to any app that hasn't already updated.
4. This does **not** help anyone who already updated. There is no downgrade
   path in this code: `getLatestVersion` only ever compares against
   `main`'s version, and electron-updater does not offer an "update" to a
   version lower than the one currently installed. Confirms the existing
   note in `docs/release/prerelease.md` ("If a pre-release reaches `main` by
   mistake", step 3): those users must reinstall the stable build by hand.

**Does reverting the version-bump commit on `main` also matter?** Yes, for a
different reason than stopping the update: as long as `apps/desktop/package.json`
on `main` still says the new (broken) version, every *future* patch you ship
has to version itself above that number, and the "Latest release" GitHub
badge / `releases/latest` API (used by the `dev.tools` download page outside
this repo — **UNVERIFIED**, that page's code is not in this repository) may
still point at it until a newer non-draft release exists. Revert it so `main`
matches reality.

**Commands (run by the owner, not by this session):**

```bash
REPO=the-dev-tools/dev-tools
BROKEN_TAG=desktop@<new-version>        # e.g. desktop@1.2.0

# 1. Stop the updater from offering it to anyone who hasn't updated yet.
gh release edit "$BROKEN_TAG" -R "$REPO" --draft=true

# 2. Put main's version back so new clones/badges/download pages agree with
#    what's actually being served. Prefer a revert of the exact "Version
#    projects" commit so the CHANGELOG.md entry and tag bump both roll back
#    together; fall back to a manual edit only if other commits were stacked
#    on top of it.
git fetch origin main
git switch -c fix/desktop-rollback origin/main
git revert --no-edit <version-projects-commit-sha>
git push -u origin fix/desktop-rollback
gh pr create -R "$REPO" --base main --title "Revert desktop version bump (rename rollback)" --body "..."
```

**Verify:**

```bash
# (a) The release no longer resolves anonymously - expect 404:
curl -s -o /dev/null -w '%{http_code}\n' \
  "https://api.github.com/repos/the-dev-tools/dev-tools/releases/tags/desktop@<new-version>"

# (b) main's package.json is back to the old version, once the revert PR merges:
curl -s "https://raw.githubusercontent.com/the-dev-tools/dev-tools/main/apps/desktop/package.json" | jq -r .version
# expect <old-desktop>

# (c) gh's own view of the release confirms draft:
gh release view desktop@<new-version> -R the-dev-tools/dev-tools --json isDraft,isPrerelease
```

---

## 2. `install.sh` (CLI)

Confirmed: `https://sh.dev.tools/install.sh` is a 301 to
`raw.githubusercontent.com/the-dev-tools/dev-tools/main/apps/cli/install.sh`,
so `main`'s copy is always what every user runs, immediately, with no
caching layer of its own.

**Mechanism, verified from code (`apps/cli/install.sh`):**

- With no `-v`, `get_version` reads the version straight from `main`'s
  `apps/cli/package.json` (`install.sh:106-108`), then — **only if that
  string contains a `-` pre-release suffix** — falls back to the highest
  stable `cli@*` tag (`install.sh:119-128`). A stable-looking broken version
  does **not** trigger that fallback.
- It then checks the release exists via `releases/tags/cli@<version>`
  (`install.sh:131-137`). If that returns anything but `200` — which is what
  a draft release returns to this unauthenticated call — it prints
  `"Release cli@<version> not found. It may not be published yet."` and
  calls `exit 1`.
- That `exit 1` is inside `get_version`, which `main()` calls via command
  substitution (`install.sh:328`), i.e. in a subshell. The script's own
  comment at `install.sh:329-330` is correct: the subshell's `exit 1` cannot
  stop the parent script by itself, but `get_version` echoes nothing before
  it, so `version` comes back empty and `main()`'s own `if [ -z "$version"
  ]; then exit 1; fi` (`install.sh:331-333`) stops the install right after,
  with the error already printed.

**Conclusion: drafting the release is not sufficient here — it breaks the
installer outright for every user with no fallback**, because the pre-release
dash-suffix branch is the only fallback path and a normal stable version
string never takes it. **Reverting `apps/cli/package.json` on `main` to the
last good version is required**, not optional, so that `get_version` resolves
to a version whose release is published (not draft) and the script keeps
working at all.

**Commands:**

```bash
REPO=the-dev-tools/dev-tools
BROKEN_TAG=cli@<new-version>             # e.g. cli@1.2.0

# 1. Draft the broken release too (stops `-v <new-version>` installs and any
#    other consumer reading releases/tags/cli@<new-version> directly).
gh release edit "$BROKEN_TAG" -R "$REPO" --draft=true

# 2. Required: revert apps/cli/package.json on main so install.sh resolves
#    back to the last published stable release.
git fetch origin main
git switch -c fix/cli-rollback origin/main
git revert --no-edit <version-projects-commit-sha>   # same commit as desktop if they were bumped together
git push -u origin fix/cli-rollback
gh pr create -R "$REPO" --base main --title "Revert CLI version bump (rename rollback)" --body "..."
```

**Verify:**

```bash
# After the revert PR merges, main's package.json must point at a published
# release:
V=$(curl -s https://raw.githubusercontent.com/the-dev-tools/dev-tools/main/apps/cli/package.json | jq -r .version)
echo "$V"   # expect <old-cli>
curl -s -o /dev/null -w '%{http_code}\n' \
  "https://api.github.com/repos/the-dev-tools/dev-tools/releases/tags/cli@$V"
# expect 200

# End-to-end:
curl -fsSL https://sh.dev.tools/install.sh | bash
devtools version   # expect <old-cli>
```

---

## 3. `actions/run-flows` with `version: latest`

**Mechanism, verified from code:**

- `download-cli.sh` resolves `latest` with
  `git ls-remote --tags --refs <repo>.git 'cli@*' | select-latest-cli-tag.sh`
  (`actions/run-flows/scripts/download-cli.sh:48-50`) — **git tags**, not the
  GitHub Releases API.
- `select-latest-cli-tag.sh` filters to tags matching `^cli@`, drops any with
  a `-` (pre-release) suffix, and takes the highest by `sort -V`
  (`actions/run-flows/scripts/select-latest-cli-tag.sh:17`).

**Setting a release to draft does not remove its git tag.** `gh release
edit --draft=true` only changes the Release object; the underlying
`refs/tags/cli@<new-version>` ref stays on the remote (`gh release delete`
removes the tag too, but only when `--cleanup-tag` is passed — see
`docs/release/prerelease.md:199-205` for that exact flag in the existing
cleanup procedure). Since this tag has no `-` suffix, `select-latest-cli-tag.sh`
has no reason to skip it, and `sort -V` will keep ranking it as the newest
stable tag for as long as it exists — **draft alone changes nothing on this
surface.**

**The tag must be deleted** (or superseded by a higher stable tag from the
fix release) for `version: latest` to stop resolving to it.

**Commands:**

```bash
REPO=the-dev-tools/dev-tools
BROKEN_TAG=cli@<new-version>

# Removes both the release and the tag in one step:
gh release delete "$BROKEN_TAG" -R "$REPO" --cleanup-tag --yes

# If the release was already deleted/drafted earlier and only the tag is left:
git push origin ":refs/tags/$BROKEN_TAG"
```

**Verify:**

```bash
# Tag must be gone:
git ls-remote --tags https://github.com/the-dev-tools/dev-tools.git 'cli@*'
# <new-version> must not appear in the output

# Re-resolve latest the same way the action does:
git ls-remote --tags --refs https://github.com/the-dev-tools/dev-tools.git 'cli@*' \
  | sed 's#.*refs/tags/##' \
  | ./actions/run-flows/scripts/select-latest-cli-tag.sh
# must print <old-cli>'s tag, e.g. cli@1.2.2

# Or just run a workflow using version: latest and check the step's
# `version=` output / its own `devtoolscli version` print.
```

Note this is the one surface where deleting the tag is the only fix — a
`gh workflow run` re-trigger or any change to `release.yaml`/`check.yaml`
itself is out of scope per this runbook's own hard limits (never touch
workflows), and isn't needed anyway: `download-cli.sh` and
`select-latest-cli-tag.sh` need no code change, only the tag removed.

---

## 4. Studio data folder migration (`DevTools-Studio` → `Stresseur Studio`)

**What exists in this checkout today (verified):** `apps/desktop/src/main/index.ts:147-177`,
`migrateDataDir()`. On startup, if the *current* `userData` directory
(named after `extraMetadata.name` in `apps/desktop/build.ts:10`, currently
`DevTools-Studio`) has no `state.db` yet, it looks for one in older
directories (today: `DevTools Studio` with a space, then `DevTools`) and
**copies** `state.db` (+ `-wal`/`-shm`) into the new directory with
`copyFileSync` (`index.ts:166-171`). It is a one-time, one-directional copy,
guarded by `existsSync(newDb)` so it never overwrites a database that already
exists at the destination, and it never deletes or modifies the source.

**UNVERIFIED — the actual rename migration is not in this repo yet.** The
rename-day change that adds the Stresseur Studio folder as a new migration
target (presumably appending `DevTools-Studio` to the `oldDirs` list in
`index.ts:158`, and changing `extraMetadata.name` in `build.ts:10` to
whatever the new folder is named) has not landed on `main` as of Oct 9, 2026
— only the cosmetic heads-up banner has (`83d720b`,
`announcement/pinned-issue.md`, `announcement/release-notes-snippet.md`,
`apps/desktop/src/renderer/main.tsx`). Everything below assumes the rename
PR reuses this exact `migrateDataDir` pattern (copy-only, skip-if-destination-exists);
if it instead *moves* or *deletes* the old folder, every claim in this
section is wrong and must be re-checked against that PR's actual diff before
telling any user anything.

**Assuming the pattern holds, what rollback means for users:**

- A user who upgraded to Stresseur Studio has `state.db` copied into the new
  folder; their **original `DevTools-Studio` folder and its `state.db` are
  untouched** (the function only ever reads from old dirs and writes to the
  new one — it never writes back). Reinstalling the old `1.1.x` DevTools
  Studio build will find that original file waiting for it, exactly as it
  was at the moment they upgraded.
- **What they lose:** anything they did *while running* Stresseur Studio —
  new requests, flows, environment edits — since that only ever got written
  to the new folder, which the old build never reads. There is no merge; it
  is an all-or-nothing point-in-time split.
- **What they do not lose:** anything from before the upgrade. The old copy
  was never touched.
- **Do not** tell users to manually copy the new folder's `state.db` back
  over the old one to "recover" post-upgrade work before reinstalling 1.1.x.
  If the new build ran any schema migration on its copy, that file may not
  be readable by the older code at all — **UNVERIFIED**, no such migration
  exists yet to check, so this must be re-verified against whatever the
  rename PR (or any migration shipped alongside it) actually does to
  `state.db`'s schema before giving users that instruction.

**Suggested user-facing text once a rollback happens:**

> We've rolled Stresseur Studio back to DevTools Studio 1.1.x while we fix an
> issue. Reinstalling 1.1.x restores exactly what you had before you
> upgraded — nothing from before the upgrade was touched or deleted. Anything
> you created or changed *after* upgrading to Stresseur Studio won't appear
> until we ship the fix and you upgrade again; please don't delete or move
> anything in your Stresseur Studio data folder in the meantime, in case we
> need it to recover that work.

**Rollback step for this surface specifically:** reinstalling the old
installer is enough (per the desktop section above) — there is no database
or folder command to run. The point of this section is the *message*, not an
action against GitHub.

---

## 5. Chrome extension (API Recorder / `apps/api-recorder-extension`)

**Important scope correction:** per `apps/api-recorder-extension/src/brand.ts:1-9`,
the rename does **not** rename this extension's own Chrome Web Store
listing — its comment is explicit: *"the extension's own name ('API
Recorder') stays as is"*; the only change is the `STUDIO_NAME` string it
displays (`'DevTools Studio'` → presumably `'Stresseur Studio'`) and the
`manifest.json`'s `name`/`version`/`author` are generated from
`package.json` at build time (`apps/api-recorder-extension/build.ts:79-88`).
So this is an ordinary content/string change shipped as a normal version
bump, not an identity migration — much lower blast radius than the desktop
app or CLI.

**Publish pipeline, verified from code:**
`.github/workflows/release-chrome-extension.yaml` builds the extension
(`pnpm nx run <project>:build`), uploads `dist/chrome-mv3-prod.zip`, then the
`publish` job runs `PlasmoHQ/bpp@v3` with a `BPP_KEYS` secret against that
zip — i.e. the moment that workflow's `publish` job succeeds, the new
version is submitted to the Chrome Web Store directly from CI. Dispatched
only by `workflow_dispatch` (manual), and also by `gha-scripts release
api-recorder-extension` (`tools/gha-scripts/src/cli.ts:82`,
`ReleaseWorkflows['api-recorder-extension']`).

**Can a Chrome Web Store rollback be done from the dashboard? UNVERIFIED.**
This repository has no code for the Web Store dashboard — it is a Google
product surface this checkout cannot inspect. Best current public
understanding (not verified against the dashboard itself, flagged here
rather than stated as fact):
- The Web Store serves installed users the latest *published* version via
  its own update ping; there is no self-service "revert users to an older
  version" button for a listing that has already finished rolling out to
  100%.
- If a **staged rollout** is still in progress (the dashboard's rollout
  percentage has not reached 100%), the owner may be able to pause or lower
  that percentage from the dashboard before more users receive the broken
  build — **UNVERIFIED, confirm the current state of that version in the
  dashboard directly.**
- The reliable fix, regardless of rollout state, is the same one the owner's
  rule already describes: ship a new, higher-numbered version through the
  normal pipeline with the regression reverted. Because this extension's
  identity isn't changing, that "fix" can be a plain revert of whatever
  commit changed `STUDIO_NAME` (or whatever else broke), bumped and released
  the same way as any other patch.
- Unpublishing the item (dashboard: set to "Unlisted"/"Draft") stops *new*
  installs from the Web Store listing page but, per the same caveat, does
  not downgrade anyone already on the bad version — **UNVERIFIED**, confirm
  in the dashboard; stated here by analogy with how Chrome extension updates
  are known to work (pull, not push, and only ever to the latest), not from
  anything in this repository.

**What the owner must do by hand (not executable from this repo or by this
session):**
1. Open the Chrome Web Store Developer Dashboard for this item.
2. Check whether the bad version is still in staged rollout; if so, consider
   pausing/lowering it there.
3. Decide whether to unlist the current version while a fix is prepared.
4. Prepare the revert in this repo (see below) and let the normal Release
   workflow publish the new version through `PlasmoHQ/bpp`, the same way
   every other extension release goes out — there is no "rollback" button to
   press that's different from "ship a fix," per the owner's own rule.

**Repo-side commands for the fix release** (this *will* touch a workflow run
and a release if actually executed — flagging that it is the owner's call,
not something this session or this PR performs):

```bash
git fetch origin main
git switch -c fix/api-recorder-rollback origin/main
git revert --no-edit <commit-that-broke-it>
git push -u origin fix/api-recorder-rollback
gh pr create -R the-dev-tools/dev-tools --base main \
  --title "Revert API Recorder rename string change" --body "..."
# After merge, cut the patch release the normal way (CLAUDE.md → Releasing):
task version-plan project=api-recorder-extension
git push
gh workflow run release.yaml -f api-recorder-extension=true
```

**Verify:** `gh run watch` on the dispatched `release-chrome-extension.yaml`
run; once `publish` succeeds, check the listing's "Version history" in the
dashboard for the new version number — **UNVERIFIED** whether/how fast that
reflects for already-installed users (Web Store update checks are
client-initiated on their own interval, not push).

---

## Quick reference

| Surface | Does `--draft=true` alone stop it? | Also required | Verified against |
|---|---|---|---|
| Desktop auto-update | Yes | Revert `apps/desktop/package.json` on `main` (hygiene, not strictly required to stop the offer) | `update.ts:30-104` |
| `install.sh` | No — breaks the installer outright | **Required:** revert `apps/cli/package.json` on `main` | `install.sh:106-137,328-333` |
| `actions/run-flows` `version: latest` | No — tags survive draft | **Required:** `gh release delete --cleanup-tag` or delete the tag directly | `download-cli.sh:48-50`, `select-latest-cli-tag.sh:17` |
| Already-updated desktop users | N/A | Tell them to reinstall the stable build; no auto-downgrade exists | `update.ts:89-105`, `docs/release/prerelease.md` "If a pre-release reaches main by mistake" |
| Studio data folder | N/A | Nothing to run — the copy-only migration (if it follows the existing pattern) already protects old data; see messaging above | `index.ts:147-177` (current code; rename-day version is **UNVERIFIED**) |
| Chrome extension (API Recorder) | **UNVERIFIED** (dashboard, not this repo) | Ship a reverted patch version through the normal pipeline | `release-chrome-extension.yaml`, `brand.ts:1-9` |

## Hard limits on this runbook itself

This document only describes commands for the owner to run. Producing it
did not create, edit, delete, or draft any release; did not create, delete,
or push any tag; did not dispatch, edit, or modify any workflow; and did not
open any issue.
