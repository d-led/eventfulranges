#!/usr/bin/env bash
# Merge-strategy microbenchmarks: the cost of one full materialization, of the
# append path alone, and of folding n ops then materializing once. Informational
# only; not part of the quality gate.
set -euo pipefail
cd "$(dirname "$0")/.."

benchtime="${BENCHTIME:-300ms}"

echo "== strategy: one full materialization"
go test -run '^$' -bench 'BenchmarkMaterialize$' -benchmem -benchtime="$benchtime" ./space/strategy/

echo
echo "== engine: append path (lazy, no materialization)"
go test -run '^$' -bench 'BenchmarkApplyIncremental$' -benchmem -benchtime="$benchtime" ./space/engine/

echo
echo "== engine: append then materialize once"
go test -run '^$' -bench 'BenchmarkApplyThenMaterialize$' -benchmem -benchtime="$benchtime" ./space/engine/
