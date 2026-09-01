#!/usr/bin/env bash
# Static analysis for the core library and the demo module, each in its own
# module so the Go toolchain resolves its dependency graph independently.
# File-tree checks (gofmt, gocyclo, jscpd) run once over the whole repository.
#
# Usage:
#   scripts/lint.sh             # run every check
#   scripts/lint.sh --install   # install missing tools first, then run
#
# Checks (in order): gofmt, go vet, staticcheck, golangci-lint, gocyclo,
# revive, gosec, govulncheck, jscpd.
set -euo pipefail

# --- tunable thresholds -----------------------------------------------------
GOCYCLO_OVER="${GOCYCLO_OVER:-15}"       # cyclomatic complexity that fails the build
JSCPD_THRESHOLD="${JSCPD_THRESHOLD:-0.5}" # duplication % that fails the build

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# The workspace's two Go modules. Every module-scoped check runs in each.
MODULES=("." "demo")

if [[ "${1:-}" == "--install" ]]; then
  echo "installing missing tools…"
  command -v staticcheck   >/dev/null || go install honnef.co/go/tools/cmd/staticcheck@latest
  command -v gocyclo       >/dev/null || go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
  command -v revive        >/dev/null || go install github.com/mgechev/revive@latest
  command -v gosec         >/dev/null || go install github.com/securego/gosec/v2/cmd/gosec@latest
  command -v govulncheck   >/dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
  command -v golangci-lint >/dev/null || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
  echo "done."
fi

RED=$'\033[31m'; GREEN=$'\033[32m'; BOLD=$'\033[1m'; RESET=$'\033[0m'

section() { printf '\n%s\n' "${BOLD}==> $*${RESET}"; }
fail()    { printf '%serror:%s %s\n' "$RED" "$RESET" "$*" >&2; exit 1; }
need()    { command -v "$1" >/dev/null 2>&1 || fail "$1 is not installed — run scripts/lint.sh --install (or: $2)"; }

# each runs the given command in every module, from that module's directory.
each() {
  local m
  for m in "${MODULES[@]}"; do
    (cd "$ROOT/$m" && "$@")
  done
}

section "gofmt"
files="$(gofmt -l .)"
[[ -z "$files" ]] || { printf '%s\n' "$files" >&2; fail "gofmt: the files above need formatting (gofmt -w .)"; }

section "go vet"
each go vet ./...

section "staticcheck"
need staticcheck "go install honnef.co/go/tools/cmd/staticcheck@latest"
each staticcheck ./...

section "golangci-lint"
need golangci-lint "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"
each golangci-lint run --timeout=5m ./...

section "gocyclo (complexity > ${GOCYCLO_OVER})"
need gocyclo "go install github.com/fzipp/gocyclo/cmd/gocyclo@latest"
complex="$(gocyclo -over "${GOCYCLO_OVER}" . || true)"
[[ -z "$complex" ]] || { printf '%s\n' "$complex" >&2; fail "gocyclo: functions above cyclomatic complexity ${GOCYCLO_OVER}"; }

section "revive"
need revive "go install github.com/mgechev/revive@latest"
each revive -config "$ROOT/revive.toml" ./...

section "gosec"
need gosec "go install github.com/securego/gosec/v2/cmd/gosec@latest"
each gosec -quiet -exclude-generated ./...

section "govulncheck"
need govulncheck "go install golang.org/x/vuln/cmd/govulncheck@latest"
each govulncheck ./...

section "jscpd (duplication ≤ ${JSCPD_THRESHOLD}%)"
need jscpd "npm install -g jscpd"
jscpd . --config .jscpd.json --threshold "${JSCPD_THRESHOLD}"

printf '\n%sall checks passed%s\n' "$GREEN" "$RESET"
