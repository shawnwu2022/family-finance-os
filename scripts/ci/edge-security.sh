#!/usr/bin/env bash
set -euo pipefail
umask 077

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

bash scripts/check-edge-security.sh
bash scripts/ci/application-auth-edge-contract.sh
bash scripts/ci/test-finance-frame-protection.sh

docker compose version >/dev/null

TMP_ROOT="$(mktemp -d /tmp/family-finance-edge-security.XXXXXX)"
ENV_FILE="$TMP_ROOT/edge.env"
EDGE_PROJECT="${CI_EDGE_PROJECT_NAME:-family-finance-edge-${UID:-0}-$$}"
mkdir -p "$TMP_ROOT/secrets"
for name in finance-auth-key finance-admin-password ezbookkeeping-api-token ezbookkeeping-secret-key; do
  printf '%s' 'edge-security-secret-material' >"$TMP_ROOT/secrets/$name"
  chmod 0600 "$TMP_ROOT/secrets/$name"
done

cat > "$ENV_FILE" <<EOF_ENV
EBK_DOMAIN=book.example.test
FINANCE_DOMAIN=finance.example.test
POSTGRES_USER=postgres
POSTGRES_PASSWORD=test-postgres
FINANCE_DB_PASSWORD=test-finance
EBK_DB_PASSWORD=test-ebk
FINANCE_AUTH_KEY_HOST_FILE=$TMP_ROOT/secrets/finance-auth-key
FINANCE_ADMIN_PASSWORD_HOST_FILE=$TMP_ROOT/secrets/finance-admin-password
EBK_API_TOKEN_HOST_FILE=$TMP_ROOT/secrets/ezbookkeeping-api-token
EBK_SECURITY_SECRET_KEY_HOST_FILE=$TMP_ROOT/secrets/ezbookkeeping-secret-key
EBK_USER_ENABLE_REGISTER=false
EOF_ENV

compose=(docker compose -p "$EDGE_PROJECT" --env-file "$ENV_FILE" -f compose.yaml)
cleanup() {
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

"${compose[@]}" config >/dev/null
"${compose[@]}" run --rm --no-deps \
  --entrypoint caddy caddy \
  validate --config /etc/caddy/Caddyfile --adapter caddyfile

echo "Edge security verification OK"
