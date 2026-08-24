#!/usr/bin/env bash
# Stops the local KurrentDB.
set -euo pipefail
cd "$(dirname "$0")/.."

docker compose down
