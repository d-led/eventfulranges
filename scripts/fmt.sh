#!/usr/bin/env bash
# Formats all Go files in place with gofumpt (the same formatter the linter
# enforces) and fails if any file still needs formatting afterwards.
set -euo pipefail
cd "$(dirname "$0")/.."

gofumpt=(go run mvdan.cc/gofumpt@v0.9.0)

"${gofumpt[@]}" -w .

# Any file still listed by -l was unformattable or is checked in unformatted.
if unformatted=$("${gofumpt[@]}" -l .); then
  if [ -n "$unformatted" ]; then
    echo "gofumpt: the following files are not formatted:" >&2
    echo "$unformatted" >&2
    exit 1
  fi
fi

