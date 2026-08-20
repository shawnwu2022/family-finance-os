#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

bash scripts/test-production-ops.sh

tag="family-finance-os:verify-${UID:-0}-$$"
cleanup() {
  docker image rm -f "$tag" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker build -t "$tag" .

echo "Container/production operations verification OK"
