#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

bash scripts/ci/contract-test.sh
docker compose version >/dev/null

make verify-go
make verify-mcp-security
make verify-web
make verify-edge-security
make verify-container

echo "Repository-native verification OK"
