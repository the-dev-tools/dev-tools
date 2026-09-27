#!/usr/bin/env bash
# Reads tag names (one per line, e.g. "cli@1.0.3") on stdin and prints the
# highest stable devtoolscli release tag. Used by download-cli.sh to resolve
# `version: latest`.
#
# Pre-release tags (anything with a "-" pre-release suffix, e.g.
# "cli@1.2.0-rc.1" or "cli@1.2.0-0") are skipped. Without this, `sort -V`
# ranks "cli@1.2.0-rc.1" *above* "cli@1.2.0", so a single pushed pre-release
# tag would become every consumer's `latest` immediately. An explicit
# `version:` input (e.g. cli@1.2.0-rc.1) still resolves to that exact tag,
# because download-cli.sh only calls this for `latest`.
#
# Prints nothing (and exits 0) if no stable cli@ tag is present; the caller
# turns that into an error.
set -euo pipefail

grep -E '^cli@' | grep -v -e '-' | sort -V | tail -n1 || true
