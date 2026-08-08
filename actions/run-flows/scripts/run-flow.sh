#!/usr/bin/env bash
# Runs the resolved devtoolscli binary against the given yamlflow file.
# Exits with devtoolscli's own exit code (0 on success, non-zero if any flow
# failed) — the calling step uses continue-on-error so a failing run doesn't
# skip the summary/output steps that follow it.
#
# Env in:
#   CLI_BIN     - absolute path to the devtoolscli binary (from download-cli.sh)
#   FILE        - path to the .yamlflow.yaml file (inputs.file)
#   FLOW        - optional single flow name (inputs.flow)
#   REPORT_DIR  - directory to write json/junit reports into (inputs.report-dir)
set -uo pipefail

if [[ -z "${CLI_BIN:-}" || ! -x "$CLI_BIN" ]]; then
  echo "::error::run-flow.sh: CLI_BIN ('${CLI_BIN:-<unset>}') is not an executable file." >&2
  exit 1
fi

if [[ -z "${FILE:-}" ]]; then
  echo "::error::run-flow.sh: the 'file' input is required." >&2
  exit 1
fi

report_dir="${REPORT_DIR:-.devtools-reports}"

args=(flow run "$FILE")
if [[ -n "${FLOW:-}" ]]; then
  args+=("$FLOW")
fi
args+=(--report console --report "json:${report_dir}/report.json" --report "junit:${report_dir}/junit.xml")

echo "+ devtoolscli ${args[*]}"
"$CLI_BIN" "${args[@]}"
