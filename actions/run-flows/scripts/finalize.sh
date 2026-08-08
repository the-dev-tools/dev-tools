#!/usr/bin/env bash
# Sets the action's outputs (json-report, junit-report, success) and enforces
# the fail-on-error input. Runs with `if: always()` in action.yml so outputs
# are always set, even when the flow run failed.
#
# Env in:
#   REPORT_DIR     - directory containing report.json/junit.xml (inputs.report-dir)
#   RUN_OUTCOME    - outcome of the "Run flow" step ("success"/"failure"/"")
#   FAIL_ON_ERROR  - inputs.fail-on-error ("true"/"false")
set -uo pipefail

json_report="${REPORT_DIR:-.devtools-reports}/report.json"
junit_report="${REPORT_DIR:-.devtools-reports}/junit.xml"

[[ -f "$json_report" ]] || json_report=''
[[ -f "$junit_report" ]] || junit_report=''

success='false'
[[ "${RUN_OUTCOME:-}" == 'success' ]] && success='true'

{
  echo "json-report=${json_report}"
  echo "junit-report=${junit_report}"
  echo "success=${success}"
} >> "$GITHUB_OUTPUT"

if [[ "$success" == 'false' && "${FAIL_ON_ERROR:-true}" == 'true' ]]; then
  echo "::error::devtoolscli flow run did not complete successfully (run step outcome: ${RUN_OUTCOME:-unknown}) and fail-on-error is 'true'." >&2
  exit 1
fi

exit 0
