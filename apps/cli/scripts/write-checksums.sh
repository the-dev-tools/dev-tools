#!/usr/bin/env bash
# Prints a sha256sum-compatible checksums file ("<sha256>  <name>", sorted by
# name) for every file in <dir> except checksums.txt itself.
# Used by .github/workflows/release-go.yaml to publish checksums.txt with every
# CLI release, which apps/cli/install.sh verifies downloads against.
#
# Usage: write-checksums.sh <dir> > checksums.txt
set -euo pipefail

dir="${1:?usage: write-checksums.sh <dir>}"
cd "$dir"

files=()
while IFS= read -r -d '' file; do
  files+=("${file#./}")
done < <(find . -maxdepth 1 -type f ! -name checksums.txt -print0 | LC_ALL=C sort -z)

if [[ ${#files[@]} -eq 0 ]]; then
  echo "write-checksums.sh: no files to checksum in ${dir}" >&2
  exit 1
fi

if command -v sha256sum > /dev/null; then
  sha256sum -- "${files[@]}"
else
  shasum -a 256 -- "${files[@]}"
fi
