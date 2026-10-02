#!/usr/bin/env bash
# Tests for apps/cli/install.sh (Windows asset names, checksum verification)
# and apps/cli/scripts/write-checksums.sh. No network: uname and curl are
# stubbed, and install.sh runs unmodified against the stubs. Run with
#   bash apps/cli/scripts/install.test.sh
set -euo pipefail

cli_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
install_sh="$cli_dir/install.sh"
write_checksums="$cli_dir/scripts/write-checksums.sh"

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

# Fake release: one asset per platform, as named by release-go.yaml.
version='1.1.1'
release="$work/release"
mkdir -p "$release"
for p in darwin-arm64 darwin-x64 linux-arm64 linux-x64 win32-ia32.exe win32-x64.exe; do
  echo "binary for ${p}" > "$release/devtools-cli-${version}-${p}"
done

stubs="$work/stubs"
mkdir -p "$stubs"

# uname: -s -> $FAKE_OS, -m -> $FAKE_ARCH.
cat > "$stubs/uname" <<'EOF'
#!/usr/bin/env bash
case "$1" in -s) echo "$FAKE_OS" ;; -m) echo "$FAKE_ARCH" ;; *) echo "$FAKE_OS" ;; esac
EOF

# curl: serves main's package.json, release checks and release downloads from
# $RELEASE_DIR; unknown files are a 404 (exit 22 with -f). Logs every URL.
cat > "$stubs/curl" <<'EOF'
#!/usr/bin/env bash
out="" url="" write=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o) out="$2"; shift 2 ;;
    -w) write="$2"; shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
echo "$url" >> "$CURL_LOG"
case "$url" in
  */apps/cli/package.json) echo "  \"version\": \"$FAKE_VERSION\"," ;;
  */releases/tags/*) [[ -n "$write" ]] && printf '200' ;;
  */releases/download/*)
    file="$RELEASE_DIR/${url##*/}"
    [[ -f "$file" ]] || exit 22
    cp "$file" "$out"
    ;;
  *) exit 22 ;;
esac
EOF
chmod +x "$stubs/uname" "$stubs/curl"

export CURL_LOG="$work/curl.log" RELEASE_DIR="$release" FAKE_VERSION="$version"

# run_install <os> <arch> [PATH]: installs into a fresh dir, prints the exit code.
run_install() {
  local path="${3:-$stubs:$PATH}"
  rm -rf "${work:?}/bin" "${work:?}/out"
  mkdir -p "$work/bin"
  : > "$CURL_LOG"
  set +e
  FAKE_OS="$1" FAKE_ARCH="$2" INSTALL_DIR="$work/bin" PATH="$path" \
    bash "$install_sh" > "$work/out" 2>&1
  echo $?
  set -e
}
downloaded() { grep -qx "https://github.com/the-dev-tools/dev-tools/releases/download/cli@${version}/$1" "$CURL_LOG"; }
said() { sed 's/\x1b\[[0-9;]*m//g' "$work/out" | grep -qF "$1"; }
installed() { [ -f "$work/bin/devtools" ] && [ "$(cat "$work/bin/devtools")" = "binary for $1" ]; }

# --- Windows asset names (no checksums.txt yet, like cli@1.1.1) -------------
rc=$(run_install MINGW64_NT-10.0-26100 x86_64)
check 'Git Bash x64 exits 0' [ "$rc" = 0 ]
check 'Git Bash x64 downloads the win32-x64.exe asset' downloaded "devtools-cli-${version}-win32-x64.exe"
check 'Git Bash x64 installs it' installed win32-x64.exe
check 'missing checksums.txt is skipped with a note' said 'No checksums.txt published for cli@1.1.1'

rc=$(run_install MSYS_NT-10.0 i686)
check 'MSYS ia32 downloads the win32-ia32.exe asset' downloaded "devtools-cli-${version}-win32-ia32.exe"
check 'MSYS ia32 installs it' installed win32-ia32.exe

rc=$(run_install CYGWIN_NT-10.0 x86_64)
check 'Cygwin x64 maps to win32-x64.exe' downloaded "devtools-cli-${version}-win32-x64.exe"

rc=$(run_install Linux x86_64)
check 'Linux x64 is unchanged' downloaded "devtools-cli-${version}-linux-x64"
rc=$(run_install Darwin arm64)
check 'macOS arm64 is unchanged' downloaded "devtools-cli-${version}-darwin-arm64"

# --- write-checksums.sh ------------------------------------------------------
"$write_checksums" "$release" > "$work/checksums.txt"
check 'write-checksums lists every asset' [ "$(wc -l < "$work/checksums.txt" | tr -d ' ')" = 6 ]
check 'write-checksums output passes sha256sum -c' \
  bash -c "cd '$release' && sha256sum -c --quiet '$work/checksums.txt' 2>/dev/null || shasum -a 256 -c --quiet '$work/checksums.txt'"
cp "$work/checksums.txt" "$release/checksums.txt"
"$write_checksums" "$release" > "$work/checksums2.txt"
check 'write-checksums ignores an existing checksums.txt' cmp -s "$work/checksums.txt" "$work/checksums2.txt"
check 'write-checksums fails on an empty dir' bash -c "! '$write_checksums' '$(mktemp -d)' 2>/dev/null"

# --- checksum verification against the published file ------------------------
rc=$(run_install Linux x86_64)
check 'matching checksum installs' [ "$rc" = 0 ]
check 'matching checksum is reported as verified' said 'Checksum verified'
rc=$(run_install MINGW64_NT-10.0 x86_64)
check 'Windows asset checksum is verified too' said 'Checksum verified'

# Tampered binary: checksum no longer matches.
cp "$release/devtools-cli-${version}-linux-x64" "$work/orig"
echo tampered > "$release/devtools-cli-${version}-linux-x64"
rc=$(run_install Linux x86_64)
check 'mismatch exits non-zero' [ "$rc" != 0 ]
check 'mismatch is reported' said 'Checksum verification failed for devtools-cli-1.1.1-linux-x64'
check 'mismatch installs nothing' [ ! -e "$work/bin/devtools" ]
check 'mismatch removes the download' [ ! -e "/tmp/devtools-cli-${version}-linux-x64" ]
cp "$work/orig" "$release/devtools-cli-${version}-linux-x64"

# checksums.txt without an entry for this asset: skipped, not failed.
grep -v 'darwin-arm64' "$work/checksums.txt" > "$release/checksums.txt"
rc=$(run_install Darwin arm64)
check 'missing entry still installs' [ "$rc" = 0 ]
check 'missing entry is reported' said 'checksums.txt has no entry for devtools-cli-1.1.1-darwin-arm64'
cp "$work/checksums.txt" "$release/checksums.txt"

# No sha256sum on PATH (stock macOS): falls back to shasum -a 256.
if command -v shasum > /dev/null; then
  tools="$work/tools"
  mkdir -p "$tools"
  for t in bash env awk basename cat chmod cp grep head mkdir mv rm sed sort tail tr ln perl shasum; do
    src=$(command -v "$t" || true)
    [[ -n "$src" ]] && ln -sf "$src" "$tools/$t"
  done
  rc=$(run_install Darwin arm64 "$stubs:$tools")
  check 'without sha256sum, shasum verifies the checksum' said 'Checksum verified'
  echo tampered > "$release/devtools-cli-${version}-darwin-arm64"
  rc=$(run_install Darwin arm64 "$stubs:$tools")
  check 'without sha256sum, a mismatch still fails' [ "$rc" != 0 ]
fi

if [[ $failures -gt 0 ]]; then
  echo "${failures} test(s) failed"
  exit 1
fi
echo 'all install.sh tests passed'
