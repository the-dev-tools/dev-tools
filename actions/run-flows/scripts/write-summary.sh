#!/usr/bin/env bash
# Appends a flows/status/duration table to $GITHUB_STEP_SUMMARY from the JSON
# report. Runs with `if: always()` in action.yml so it still produces a
# summary when the flow run step failed. Deliberately does not use `set -e`:
# a malformed report should degrade the summary, not abort the composite
# action before outputs get set by finalize.sh.
#
# Env in:
#   REPORT_DIR   - directory containing report.json (inputs.report-dir)
#   RUN_OUTCOME  - outcome of the "Run flow" step ("success"/"failure"/"" )
set -uo pipefail

report_json="${REPORT_DIR:-.devtools-reports}/report.json"
summary_file="${GITHUB_STEP_SUMMARY:-/dev/stdout}"

run_outcome="${RUN_OUTCOME:-unknown}"
if [[ "$run_outcome" == 'success' ]]; then
  heading='DevTools flow run — success'
else
  heading='DevTools flow run — failed'
fi

{
  echo "### ${heading}"
  echo

  if [[ ! -f "$report_json" ]]; then
    echo "_No JSON report was produced at \`${report_json}\`. The \"Run flow\" step outcome was **${run_outcome}** — check its logs above._"
    exit 0
  fi

  if ! total=$(jq 'length' "$report_json" 2>/dev/null); then
    echo "_Could not parse \`${report_json}\` as JSON. The \"Run flow\" step outcome was **${run_outcome}**._"
    exit 0
  fi
  passed=$(jq '[.[] | select(.status == "success")] | length' "$report_json")

  echo "**Flows:** ${passed}/${total} passed"
  echo
  echo '| Flow | Status | Duration |'
  echo '| --- | --- | --- |'
  jq -r '
    .[]
    | "| " + .flow_name
      + " | " + (if .status == "success" then "✅ Success" else "❌ Failed" end)
      + " | " + (((.duration / 1000000000 * 100 | round) / 100 | tostring) + "s")
      + " |"
  ' "$report_json"
} >> "$summary_file"
