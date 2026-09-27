#!/usr/bin/env bash
# Tests for scripts/select-latest-cli-tag.sh, the `version: latest` resolver.
# Pure bash, no network: run locally with
#   bash actions/run-flows/test/select-latest-cli-tag.test.sh
# and in CI by .github/workflows/action-test.yaml (Linux GNU sort and macOS
# BSD sort both run it, since `sort -V` is the part most likely to differ).
set -euo pipefail

select_tag="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/scripts/select-latest-cli-tag.sh"

failures=0

check() {
  local name="$1" expected="$2" input="$3" actual
  actual=$(printf '%s' "$input" | "$select_tag")
  if [[ "$actual" == "$expected" ]]; then
    echo "ok   - ${name}"
  else
    echo "FAIL - ${name}: expected '${expected}', got '${actual}'"
    failures=$((failures + 1))
  fi
}

# The cli@ tags that exist on the-dev-tools/dev-tools today.
current_tags='cli@0.1.0
cli@0.2.0
cli@0.2.1
cli@0.2.2
cli@1.0.0
cli@1.0.1
cli@1.0.2
cli@1.0.3
cli@1.1.0
cli@1.1.1
'

check 'current tags resolve to the newest stable release' 'cli@1.1.1' "$current_tags"

check 'a numeric pre-release (nx preminor) is ignored' 'cli@1.1.1' "${current_tags}cli@1.2.0-0
"

check 'rc/beta/alpha pre-releases are ignored' 'cli@1.1.1' "${current_tags}cli@1.2.0-rc.1
cli@1.2.0-beta.2
cli@2.0.0-alpha.0
"

check 'stable release wins once published, even though sort -V ranks its rc higher' 'cli@1.2.0' "${current_tags}cli@1.2.0-rc.1
cli@1.2.0-0
cli@1.2.0
"

check 'numeric ordering, not lexical (1.10.0 > 1.9.0)' 'cli@1.10.0' 'cli@1.9.0
cli@1.10.0
cli@1.2.0
'

check 'input order does not matter' 'cli@1.1.1' 'cli@1.2.0-rc.1
cli@1.1.1
cli@0.1.0
cli@1.0.3
'

check 'non-cli tags are ignored' 'cli@1.1.1' "${current_tags}desktop@1.1.0
desktop@9.9.9
"

check 'only pre-releases present resolves to nothing' '' 'cli@1.2.0-rc.1
cli@1.2.0-0
'

check 'empty input resolves to nothing' '' ''

if [[ "$failures" -ne 0 ]]; then
  echo "${failures} test(s) failed"
  exit 1
fi
echo 'all select-latest-cli-tag tests passed'
