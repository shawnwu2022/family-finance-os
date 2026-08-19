#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "OpenClaw ephemeral release acceptance failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

for command_name in docker curl openssl jq sha256sum sed grep awk getent; do
  command -v "$command_name" >/dev/null || fail "$command_name is required"
done
docker compose version >/dev/null || fail "Docker Compose v2 is required"

live_smoke="scripts/acceptance/openclaw-mcp-live-smoke.sh"
audit_table="agent_tool_audits"
[[ -f "$live_smoke" ]] || fail "live smoke helper is missing"
[[ -n "$audit_table" ]] || fail "audit table contract is missing"

workdir="$(mktemp -d /tmp/family-finance-openclaw-acceptance.XXXXXX)"
chmod 0700 "$workdir"
secrets_dir="$workdir/secrets"
mkdir -m 0700 "$secrets_dir"
env_file="$workdir/acceptance.env"
caddy_root="$workdir/caddy-root.crt"
project="family-finance-openclaw-${GITHUB_RUN_ID:-$$}-${RANDOM}"
hosts_tag="family-finance-openclaw-${GITHUB_RUN_ID:-$$}-${RANDOM}"
hosts_added=0

export FINANCE_ACCEPTANCE_SECRETS_DIR="$secrets_dir"

compose=(
  docker compose
  --env-file "$env_file"
  -p "$project"
  -f compose.yaml
  -f compose.openclaw-acceptance.yaml
)

cleanup() {
  set +e
  "${compose[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  if [[ "$hosts_added" == "1" ]]; then
    sudo sed -i "/# ${hosts_tag}$/d" /etc/hosts >/dev/null 2>&1 || true
  fi
  rm -rf "$workdir"
}
trap cleanup EXIT INT TERM

random_hex() {
  openssl rand -hex "${1:-24}"
}

postgres_password="$(random_hex 24)"
finance_db_password="$(random_hex 24)"
ebk_db_password="$(random_hex 24)"
finance_auth_password="$(random_hex 18)"
ebk_user_password="$(random_hex 18)"
ebk_security_secret="$(openssl rand -base64 32 | tr -d '\n')"
mcp_token="$(random_hex 32)"
printf '%s' "$mcp_token" >"$secrets_dir/finance-mcp-token"
chmod 0600 "$secrets_dir/finance-mcp-token"

finance_auth_hash="$(docker run --rm caddy:2.11.4-alpine caddy hash-password --algorithm bcrypt --plaintext "$finance_auth_password")"
[[ -n "$finance_auth_hash" ]] || fail "could not generate Caddy Basic Auth hash"

cat >"$env_file" <<EOF_ENV
EBK_DOMAIN=ebk.localhost
FINANCE_DOMAIN=finance.localhost
FINANCE_AUTH_USER=acceptance
FINANCE_AUTH_HASH='$finance_auth_hash'
POSTGRES_USER=postgres
POSTGRES_PASSWORD=$postgres_password
POSTGRES_DB=postgres
FINANCE_DB_NAME=finance
FINANCE_DB_USER=finance_app
FINANCE_DB_PASSWORD=$finance_db_password
EBK_DB_NAME=ezbookkeeping
EBK_DB_USER=ezbookkeeping_app
EBK_DB_PASSWORD=$ebk_db_password
EBK_SECURITY_SECRET_KEY=$ebk_security_secret
EBK_API_TOKEN=
FINANCE_LISTEN_ADDR=:8000
FINANCE_HEALTHCHECK_URL=http://127.0.0.1:8000/healthz
DB_HOST=postgres
DB_PORT=5432
DB_SSLMODE=disable
APP_TIMEZONE=Asia/Shanghai
EBK_BASE_URL=http://ezbookkeeping:8080/api/v1
MCP_ENABLED=true
MCP_HOUSEHOLD_ID=1
MCP_REQUEST_TIMEOUT=30s
MCP_MAX_CONCURRENT=4
MCP_REQUESTS_PER_MINUTE=120
MCP_MAX_BODY_BYTES=262144
FINANCE_ACCEPTANCE_SECRETS_DIR=$secrets_dir
EOF_ENV
chmod 0600 "$env_file"

if ! getent ahostsv4 finance.localhost 2>/dev/null | awk '{print $1}' | grep -qx '127.0.0.1'; then
  printf '127.0.0.1 finance.localhost ebk.localhost # %s\n' "$hosts_tag" | sudo tee -a /etc/hosts >/dev/null
  hosts_added=1
fi

"${compose[@]}" config >/dev/null
"${compose[@]}" up -d --wait postgres
postgres_cid="$("${compose[@]}" ps -q postgres)"
[[ -n "$postgres_cid" ]] || fail "PostgreSQL container id is unavailable"

goose_version="3.27.3"
goose_bin="$workdir/goose_linux_x86_64"
curl -fsSL -o "$goose_bin" "https://github.com/pressly/goose/releases/download/v${goose_version}/goose_linux_x86_64"
echo "ca18112e2438b3ad608af9a5938beafd01fa36a4a19a3edbe4f29226ca5c8533  $goose_bin" | sha256sum -c - >/dev/null
chmod 0755 "$goose_bin"
docker cp "$goose_bin" "$postgres_cid:/tmp/goose"
docker exec "$postgres_cid" chmod 0755 /tmp/goose
docker exec "$postgres_cid" rm -rf /tmp/family-finance-migrations
docker exec "$postgres_cid" mkdir -p /tmp/family-finance-migrations
docker cp "$ROOT_DIR/db/migrations/." "$postgres_cid:/tmp/family-finance-migrations/"
finance_db_url="postgres://finance_app:${finance_db_password}@127.0.0.1:5432/finance?sslmode=disable"
docker exec "$postgres_cid" /tmp/goose -dir /tmp/family-finance-migrations postgres "$finance_db_url" up >/dev/null

period="$(TZ=Asia/Shanghai date +%Y-%m)"
household_id="$(
  docker exec -i -e PGPASSWORD="$finance_db_password" "$postgres_cid" \
    psql -X -q -A -t -v ON_ERROR_STOP=1 -U finance_app -d finance -v period="$period" <<'SQL'
WITH inserted_household AS (
    INSERT INTO households (name, base_currency, timezone)
    VALUES ('openclaw-release-acceptance', 'CNY', 'Asia/Shanghai')
    RETURNING id
), inserted_policy AS (
    INSERT INTO household_policies (household_id, liquidity_floor_minor, currency)
    SELECT id, 0, 'CNY' FROM inserted_household
), inserted_budget AS (
    INSERT INTO budget_plans (household_id, period, currency)
    SELECT id, :'period', 'CNY' FROM inserted_household
)
SELECT id FROM inserted_household;
SQL
)"
household_id="$(printf '%s' "$household_id" | tr -d '[:space:]')"
[[ "$household_id" =~ ^[1-9][0-9]*$ ]] || fail "acceptance household id is invalid"
export MCP_HOUSEHOLD_ID="$household_id"

"${compose[@]}" up -d ezbookkeeping caddy
caddy_cid="$("${compose[@]}" ps -q caddy)"
[[ -n "$caddy_cid" ]] || fail "Caddy container id is unavailable"

ca_ready=0
for _ in $(seq 1 30); do
  if docker cp "$caddy_cid:/data/caddy/pki/authorities/local/root.crt" "$caddy_root" >/dev/null 2>&1; then
    ca_ready=1
    break
  fi
  sleep 1
done
[[ "$ca_ready" == "1" && -s "$caddy_root" ]] || fail "Caddy local CA root was not created"
chmod 0600 "$caddy_root"
export CURL_CA_BUNDLE="$caddy_root"
export NODE_EXTRA_CA_CERTS="$caddy_root"

ebk_ready=0
for _ in $(seq 1 60); do
  if curl --silent --show-error --fail --connect-timeout 3 --max-time 5 \
    --cacert "$caddy_root" https://ebk.localhost/ >/dev/null 2>&1; then
    ebk_ready=1
    break
  fi
  sleep 2
done
[[ "$ebk_ready" == "1" ]] || fail "ezBookkeeping HTTPS edge did not become ready"

ebk_username="acceptance-user"
if ! "${compose[@]}" exec -T ezbookkeeping /ezbookkeeping/ezbookkeeping userdata user-add \
  --username "$ebk_username" \
  --email acceptance@example.invalid \
  --nickname Acceptance \
  --password "$ebk_user_password" \
  --default-currency CNY >"$workdir/ebk-user-add.stdout" 2>"$workdir/ebk-user-add.stderr"; then
  fail "ezBookkeeping acceptance user creation failed"
fi

if ! "${compose[@]}" exec -T ezbookkeeping /ezbookkeeping/ezbookkeeping userdata user-session-new \
  --username "$ebk_username" --type api --expiresInSeconds 3600 \
  >"$workdir/ebk-session.stdout" 2>"$workdir/ebk-session.stderr"; then
  fail "ezBookkeeping API session creation failed"
fi
ebk_api_token="$(sed -n 's/^\[NewToken\] //p' "$workdir/ebk-session.stdout" | tail -n 1)"
[[ -n "$ebk_api_token" ]] || fail "ezBookkeeping API token marker was not found"
if printf '%s' "$ebk_api_token" | grep -q '[[:space:]]'; then
  fail "ezBookkeeping API token contains whitespace"
fi
export EBK_API_TOKEN="$ebk_api_token"

printf 'header = "Authorization: %s %s"\n' "Bearer" "$ebk_api_token" >"$workdir/ebk-auth.curl"
chmod 0600 "$workdir/ebk-auth.curl"
cat >"$workdir/account.json" <<'JSON'
{"name":"Acceptance Checking","category":2,"type":1,"icon":"wallet","color":"2196F3","currency":"CNY"}
JSON
if ! curl --config "$workdir/ebk-auth.curl" --silent --show-error --fail-with-body \
  --connect-timeout 5 --max-time 30 --cacert "$caddy_root" \
  --header 'Content-Type: application/json' \
  --header 'X-Timezone-Name: Asia/Shanghai' \
  --data-binary @"$workdir/account.json" \
  https://ebk.localhost/api/v1/accounts/add.json >"$workdir/account-response.json"; then
  fail "ezBookkeeping Account API seed request failed"
fi
jq -e '.success == true and (.result.id != null)' "$workdir/account-response.json" >/dev/null \
  || fail "ezBookkeeping Account API did not create the acceptance account"

"${compose[@]}" up -d --build --wait finance-core
finance_ready=0
for _ in $(seq 1 30); do
  if curl --silent --show-error --fail --connect-timeout 3 --max-time 5 \
    --cacert "$caddy_root" --user "acceptance:${finance_auth_password}" \
    https://finance.localhost/healthz >/dev/null 2>&1; then
    finance_ready=1
    break
  fi
  sleep 2
done
[[ "$finance_ready" == "1" ]] || fail "Finance HTTPS health endpoint did not become ready"

printf 'finance_https_health=PASS\n'
printf 'ezbookkeeping_account_seed=PASS\n'
printf 'acceptance_household_id=%s\n' "$household_id"

# Task 3 replaces this fail-closed boundary with the real OpenClaw + Ollama turn.
# bash scripts/acceptance/openclaw-mcp-live-smoke.sh
# Then verify successful scoped rows in agent_tool_audits.
fail "real OpenClaw/Ollama agent stage is not implemented yet"
