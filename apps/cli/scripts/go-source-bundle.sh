#!/usr/bin/env bash
# Writes the Go source of the DevTools CLI and every module it imports, including the generated
# code that isn't in git (packages/spec/dist/buf/go, the embedded JS worker), as one tar.gz.
# Programs that embed the CLI as a library (Stresseur's `stress`) build against this bundle,
# pinned by release version, instead of a checkout plus the TypeSpec/buf toolchain.
#
# Usage: apps/cli/scripts/go-source-bundle.sh <out.tar.gz>   (run from the repo root, after
# `pnpm nx run spec:build` and `pnpm nx run cli:copy-worker`)
set -euo pipefail

out=$1
modules=(apps/cli packages/server packages/db packages/spec packages/auth-lib)
generated=(packages/spec/dist/buf/go apps/cli/embedded/embeddedJS/worker.cjs.embed)

for path in "${generated[@]}"; do
  [ -e "$path" ] || { echo "missing generated $path: run spec:build and cli:copy-worker first" >&2; exit 1; }
done

list=$(mktemp)
trap 'rm -f "$list"' EXIT
git ls-files -- "${modules[@]}" | grep -v '/node_modules/' > "$list"
find "${generated[@]}" -type f >> "$list"
tar -czf "$out" -T "$list"
echo "wrote $out ($(wc -l < "$list") files)"
