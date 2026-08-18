#!/usr/bin/env bash
set -euo pipefail
umask 077

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

bash scripts/check-edge-security.sh

docker compose version >/dev/null

TMP_ROOT="$(mktemp -d /tmp/family-finance-edge-security.XXXXXX)"
ENV_FILE="$TMP_ROOT/edge.env"
EDGE_PROJECT="${CI_EDGE_PROJECT_NAME:-family-finance-edge-${UID:-0}-$$}"

cat > "$ENV_FILE" <<'EOF'
EBK_DOMAIN=book.example.test
FINANCE_DOMAIN=finance.example.test
FINANCE_AUTH_USER=finance
FINANCE_AUTH_HASH='$2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNmpkT/5qqR7hx4IjWJPDhjvG'
POSTGRES_USER=postgres
POSTGRES_PASSWORD=test-postgres
FINANCE_DB_PASSWORD=test-finance
EBK_DB_PASSWORD=test-ebk
EBK_SECURITY_SECRET_KEY=test-secret-key
EOF

compose=(docker compose -p "$EDGE_PROJECT" --env-file "$ENV_FILE" -f compose.yaml)
cleanup() {
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

"${compose[@]}" config >/dev/null

expected='$2a$14$Zkx19XLiW6VYouLHR5NmfOFU0z2GTNmpkT/5qqR7hx4IjWJPDhjvG'
actual="$("${compose[@]}" run --rm --no-deps --entrypoint env caddy | sed -n 's/^FINANCE_AUTH_HASH=//p')"
[[ "$actual" == "$expected" ]] || {
  echo "edge runtime FINANCE_AUTH_HASH mismatch" >&2
  exit 1
}

"${compose[@]}" run --rm --no-deps \
  --entrypoint caddy caddy \
  validate --config /etc/caddy/Caddyfile --adapter caddyfile

echo "Edge security verification OK"
