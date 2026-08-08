#!/usr/bin/env bash
# Resolves the devtoolscli release for this runner's OS/arch and downloads it
# into $RUNNER_TEMP/devtools/bin. Reused by actions/run-flows/action.yml.
#
# Env in:
#   VERSION        - "latest" or a release tag, e.g. "cli@1.0.3" (also accepts a
#                    bare version like "1.0.3")
#   RUNNER_OS       - set by GitHub Actions ("Linux" / "macOS" / "Windows")
#   RUNNER_ARCH     - set by GitHub Actions ("X64" / "ARM64" / ...)
#   RUNNER_TEMP     - set by GitHub Actions; falls back to /tmp for local runs
#   GITHUB_PATH     - GitHub Actions path file (appended to)
#   GITHUB_OUTPUT   - GitHub Actions output file (appended to)
#
# Outputs (via $GITHUB_OUTPUT):
#   bin     - absolute path to the installed devtoolscli binary
#   version - resolved version number (without the "cli@" prefix)
set -euo pipefail

REPO_OWNER='the-dev-tools'
REPO_NAME='dev-tools'
REPO_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}"

version_input="${VERSION:-latest}"

case "${RUNNER_OS:-}" in
  Linux) os='linux' ;;
  macOS) os='darwin' ;;
  *)
    echo "::error::run-flows only supports Linux and macOS runners (got RUNNER_OS='${RUNNER_OS:-<unset>}'). Windows is out of scope — see actions/run-flows/README.md." >&2
    exit 1
    ;;
esac

case "${RUNNER_ARCH:-}" in
  X64) arch='x64' ;;
  ARM64) arch='arm64' ;;
  *)
    echo "::error::run-flows only supports X64 and ARM64 runners (got RUNNER_ARCH='${RUNNER_ARCH:-<unset>}')." >&2
    exit 1
    ;;
esac

platform="${os}-${arch}"

# Resolve "latest"/bare-version/tag input into a concrete release tag. Releases
# for the CLI are tagged "cli@<version>" (see tools/gha-scripts/src/cli.ts and
# .github/workflows/release-go.yaml); the repo also cuts desktop@/web@ releases
# on the same tracker, so "latest release" APIs can't be used as-is.
if [[ "$version_input" == 'latest' ]]; then
  set +e
  tag=$(git ls-remote --tags --refs "${REPO_URL}.git" 'cli@*' 2>/dev/null | sed 's#.*refs/tags/##' | sort -V | tail -n1)
  set -e
  if [[ -z "$tag" ]]; then
    echo "::error::Could not resolve the latest devtoolscli release: no cli@* tags found on ${REPO_URL} (or the network request failed)." >&2
    exit 1
  fi
elif [[ "$version_input" == cli@* ]]; then
  tag="$version_input"
else
  tag="cli@${version_input}"
fi
version_number="${tag#cli@}"

asset_name="devtools-cli-${version_number}-${platform}"
download_url="${REPO_URL}/releases/download/${tag}/${asset_name}"

echo "Resolving devtoolscli ${tag} for ${platform}..."

if ! curl -fsSI -o /dev/null "$download_url"; then
  echo "::error::devtoolscli release asset not found: ${download_url}" >&2
  echo "::error::Checked tag '${tag}' (from version input '${version_input}'). Confirm it exists at ${REPO_URL}/releases/tag/${tag} and that asset naming still matches 'devtools-cli-<version>-<os>-<arch>' (see .github/workflows/release-go.yaml)." >&2
  exit 1
fi

bin_dir="${RUNNER_TEMP:-/tmp}/devtools/bin"
bin_path="${bin_dir}/devtoolscli"
mkdir -p "$bin_dir"

if ! curl -fsSL -o "$bin_path" "$download_url"; then
  echo "::error::Failed to download ${download_url}" >&2
  exit 1
fi
chmod +x "$bin_path"

echo "Installed devtoolscli ${version_number} -> ${bin_path}"
"$bin_path" version

echo "$bin_dir" >> "$GITHUB_PATH"
{
  echo "bin=${bin_path}"
  echo "version=${version_number}"
} >> "$GITHUB_OUTPUT"
