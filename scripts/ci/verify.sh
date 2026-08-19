#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

bash scripts/ci/contract-test.sh
docker compose version >/dev/null

bash scripts/ci/go-stack-verify.sh
bash scripts/ci/mcp-security.sh
bash scripts/ci/web-verify.sh
bash scripts/ci/edge-security.sh
bash scripts/ci/container-verify.sh

echo "Repository-native verification OK"
