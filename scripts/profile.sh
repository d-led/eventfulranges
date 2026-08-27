#!/usr/bin/env bash
# Profiles the hot paths: runs benchmarks with CPU and allocation profiles and
# prints a readable top-N report for each. Informational only; not part of the
# quality gate.
#
# Usage:
#   ./scripts/profile.sh                        # profile every benchmark
#   BENCH='Apply|Tick' ./scripts/profile.sh     # profile matching benchmarks
#   PKG=./clock ./scripts/profile.sh            # profile one package
set -euo pipefail
cd "$(dirname "$0")/.."

bench="${BENCH:-.}"
pkg="${PKG:-./...}"
nodes="${NODES:-30}"
out="build/profile"
mkdir -p "$out"

echo "== benchmarking -bench '$bench' (cpu + mem profiles)"
go test -run '^$' -bench "$bench" -benchmem -count=1 \
  -cpuprofile "$out/cpu.out" -memprofile "$out/mem.out" "$pkg"

echo
echo "== CPU hot paths (top $nodes)"
go tool pprof -top -nodecount="$nodes" "$out/cpu.out"

echo
echo "== allocation hot paths (top $nodes, alloc_space)"
go tool pprof -top -alloc_space -nodecount="$nodes" "$out/mem.out"

echo
echo "profiles written to $out/{cpu,mem}.out"
