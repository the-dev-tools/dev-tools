#!/usr/bin/env bash
# write-summary.sh ends the job summary with the Stresseur upgrade line, whether or not a
# report exists.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
expected='Load-test these flows at scale on [Stresseur](https://stresseur.com).'

for case in with-report no-report; do
  : > "$tmp/summary"
  mkdir -p "$tmp/$case"
  [[ "$case" == with-report ]] && echo '[{"flow_name":"a","status":"success","duration":1000000000}]' > "$tmp/$case/report.json"
  GITHUB_STEP_SUMMARY="$tmp/summary" REPORT_DIR="$tmp/$case" RUN_OUTCOME=success \
    bash "$here/../scripts/write-summary.sh"
  last="$(grep -v '^$' "$tmp/summary" | tail -1)"
  if [[ "$last" != "_${expected}_" ]]; then
    echo "FAIL ($case): last line was: $last" >&2; exit 1
  fi
done
# upgrade-line: 'false' (set by workflows that print the line once themselves) omits it.
: > "$tmp/summary"
GITHUB_STEP_SUMMARY="$tmp/summary" REPORT_DIR="$tmp/with-report" RUN_OUTCOME=success UPGRADE_LINE=false \
  bash "$here/../scripts/write-summary.sh"
if grep -q 'stresseur.com' "$tmp/summary"; then
  echo "FAIL (UPGRADE_LINE=false): upgrade line was still written" >&2; exit 1
fi

echo "PASS write-summary upgrade line"
