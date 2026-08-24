#!/usr/bin/env bash
# Mutation testing with gremlins. Fails if the mutant kill rate drops below 80%.
#
# --timeout-coefficient 30: the default (3x the baseline test time) is too
# short once property tests are in the suite, which makes every mutant "time
# out" and hides the real signal. 30x gives mutants enough time to actually
# run, so KILLED/LIVED counts are meaningful.
#
# The gate still passes with a small number of equivalent mutants alive
# (capacity-hint arithmetic and strict comparisons guarded by !=), so the
# efficacy threshold is the quality bar rather than demanding 100%.
set -euo pipefail
cd "$(dirname "$0")/.."

if ! command -v gremlins >/dev/null 2>&1; then
  echo "installing gremlins"
  go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0
fi

mkdir -p build/reports
# Note: gremlins mutates the module in the current directory; passing a
# package pattern here makes it find nothing. The demo programs are
# illustrations, not library code, so they are excluded from mutation.
gremlins unleash \
  --threshold-efficacy 0.8 \
  --threshold-mcover 0.8 \
  --timeout-coefficient 30 \
  --exclude-files '^demo/' \
  --output build/reports/gremlins.json
