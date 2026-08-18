#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

bash scripts/ci/contract-test.sh
docker compose version >/dev/null

COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-family-finance-ci-${UID:-0}-$$}"
export COMPOSE_PROJECT_NAME
CI_COMPOSE_FILE="${CI_COMPOSE_FILE:-compose.ci.yaml}"
export CI_COMPOSE_FILE
compose=(docker compose -p "$COMPOSE_PROJECT_NAME" -f "$CI_COMPOSE_FILE")
verify_image="${COMPOSE_PROJECT_NAME}-finance-core:verify"

cleanup() {
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  docker image rm -f "$verify_image" >/dev/null 2>&1 || true
}
trap cleanup EXIT

"${compose[@]}" build go
"${compose[@]}" up -d --wait postgres

db_url='postgres://finance_app:test-secret@postgres:5432/finance?sslmode=disable'
"${compose[@]}" run --rm go goose -dir /src/db/migrations postgres "$db_url" up
"${compose[@]}" run --rm go goose -dir /src/db/migrations postgres "$db_url" down
"${compose[@]}" run --rm go goose -dir /src/db/migrations postgres "$db_url" up

bash scripts/ci/restore-verify.sh
"${compose[@]}" run --rm go bash /src/scripts/ci/go-verify.sh
"${compose[@]}" run --rm go bash /src/scripts/ci/mcp-security.sh
"${compose[@]}" run --rm web bash /src/scripts/ci/web-verify.sh

CI_EDGE_PROJECT_NAME="${COMPOSE_PROJECT_NAME}-edge" bash scripts/ci/edge-security.sh

docker build -t "$verify_image" .

echo "Repository-native verification OK"
