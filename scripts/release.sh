#!/usr/bin/env bash
# Build release artifacts locally.
#
#   ./scripts/release.sh snapshot  # cross-platform snapshot build into .goreleaser-dist/
#   ./scripts/release.sh check     # validate .goreleaser.yaml only
#   ./scripts/release.sh release   # full release (requires GITHUB_TOKEN + a version tag)
#
# The real release is driven by .github/workflows/goreleaser.yml on a v*.*.*
# tag push; use the local commands to smoke-test the config before tagging.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v goreleaser >/dev/null 2>&1; then
  echo "goreleaser not found — install it with:" >&2
  echo "  go install github.com/goreleaser/goreleaser/v2@latest" >&2
  exit 1
fi

case "${1:-snapshot}" in
  snapshot)
    goreleaser build --snapshot --clean
    ;;
  check)
    goreleaser check
    ;;
  release)
    goreleaser release --clean
    ;;
  *)
    echo "usage: $0 [snapshot|check|release]" >&2
    exit 2
    ;;
esac
