#!/usr/bin/env bash
# Tests that the action installs and runs the CLI under its new "stresseur"
# name, while downloading the unchanged devtools-cli-<version>-<os>-<arch>
# asset and keeping a "devtoolscli" symlink on PATH.
# Pure bash, no network (curl and git are stubbed): run locally with
#   bash actions/run-flows/test/cli-name.test.sh
# and in CI by .github/workflows/action-test.yaml.
set -euo pipefail

scripts="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/scripts"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

failures=0
check() {
  local name="$1"
  shift
  if "$@"; then
    echo "ok   - ${name}"
  else
    echo "FAIL - ${name}"
    failures=$((failures + 1))
  fi
}

# --- stubs -------------------------------------------------------------------
stubs="$work/stubs"
mkdir -p "$stubs"

# git ls-remote: the cli@ tags as they exist today.
cat > "$stubs/git" <<'EOF'
#!/usr/bin/env bash
printf 'x\trefs/tags/cli@1.1.0\nx\trefs/tags/cli@1.1.1\n'
EOF

# curl: logs every URL; "downloads" a fake CLI that records how it was invoked.
cat > "$stubs/curl" <<'EOF'
#!/usr/bin/env bash
out=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -*) shift ;;
    *) echo "$1" >> "$CURL_LOG"; shift ;;
  esac
done
if [[ -n "$out" && "$out" != /dev/null ]]; then
  cat > "$out" <<'CLI'
#!/usr/bin/env bash
echo "argv0=$(basename "$0")" >> "$CLI_LOG"
echo "notice_env=${STRESSEUR_NO_RENAME_NOTICE:-}${DEVTOOLS_NO_RENAME_NOTICE:-}" >> "$CLI_LOG"
echo "args=$*" >> "$CLI_LOG"
CLI
fi
EOF
chmod +x "$stubs/git" "$stubs/curl"

export PATH="$stubs:$PATH"
export CURL_LOG="$work/curl.log" CLI_LOG="$work/cli.log"
export RUNNER_OS=Linux RUNNER_ARCH=X64 RUNNER_TEMP="$work/runner"
export GITHUB_PATH="$work/github_path" GITHUB_OUTPUT="$work/github_output"
: > "$CURL_LOG"; : > "$CLI_LOG"; : > "$GITHUB_PATH"; : > "$GITHUB_OUTPUT"

# --- download-cli.sh ---------------------------------------------------------
VERSION=latest bash "$scripts/download-cli.sh" > "$work/download.out"

bin_dir="$work/runner/devtools/bin"
bin=$(sed -n 's/^bin=//p' "$GITHUB_OUTPUT")

check 'bin output points at <bin dir>/stresseur' [ "$bin" = "$bin_dir/stresseur" ]
check 'stresseur is an executable file' [ -f "$bin" -a -x "$bin" -a ! -L "$bin" ]
check 'devtoolscli is a symlink to stresseur' [ "$(readlink "$bin_dir/devtoolscli")" = stresseur ]
check 'bin dir is added to PATH' grep -qx "$bin_dir" "$GITHUB_PATH"
check 'latest resolves to cli@1.1.1' grep -qx 'version=1.1.1' "$GITHUB_OUTPUT"
check 'downloads the unchanged asset name' \
  grep -qx 'https://github.com/the-dev-tools/dev-tools/releases/download/cli@1.1.1/devtools-cli-1.1.1-linux-x64' "$CURL_LOG"
check 'the install-time version check runs as stresseur' grep -qx 'argv0=stresseur' "$CLI_LOG"

# --- run-flow.sh -------------------------------------------------------------
: > "$CLI_LOG"
(
  unset STRESSEUR_NO_RENAME_NOTICE DEVTOOLS_NO_RENAME_NOTICE
  CLI_BIN="$bin" FILE=flows/smoke.yamlflow.yaml FLOW=fetch-user REPORT_DIR=out \
    bash "$scripts/run-flow.sh" > "$work/run.out"
)

check 'the flow runs as stresseur' grep -qx 'argv0=stresseur' "$CLI_LOG"
check 'no rename-notice silencer is needed or set' grep -qx 'notice_env=' "$CLI_LOG"
check 'arguments are unchanged' grep -qx \
  'args=flow run flows/smoke.yamlflow.yaml fetch-user --report console --report json:out/report.json --report junit:out/junit.xml' "$CLI_LOG"
check 'the echoed command names stresseur' grep -q '^+ stresseur flow run ' "$work/run.out"
check 'action.yml no longer sets a rename-notice silencer' \
  bash -c "! grep -q 'NO_RENAME_NOTICE' '$scripts/../action.yml'"

if [[ $failures -gt 0 ]]; then
  echo "${failures} test(s) failed"
  exit 1
fi
echo 'all tests passed'
