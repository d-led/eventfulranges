#!/usr/bin/env bash
# Formats all Go files and fails if any file needed formatting.
set -euo pipefail
cd "$(dirname "$0")/.."

mapfile -t files < <(find . -name '*.go' -type f -not -path './build/*')
gofmt -w "${files[@]}"
