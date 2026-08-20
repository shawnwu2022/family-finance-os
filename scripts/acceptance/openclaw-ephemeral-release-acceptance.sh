#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "OpenClaw ephemeral release acceptance failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

for command_name in docker curl openssl jq sha256sum sed grep awk getent npm node; do
  command -v "$command_name" >/dev/null || fail "$command_name is required"
done
docker compose version >/dev/null || fail "Docker Compose v2 is required"

live_smoke="scripts/acceptance/openclaw-mcp-live-smoke.sh"
audit_table="agent_tool_audits"
[[ -f "$live_smoke" ]] || fail "live smoke helper is missing"
[[ -n "$audit_table" ]] || fail "audit table contract is missing"

openclaw_version="2026.7.1-2"
ollama_image="ollama/ollama:0.32.5"
ollama_model="qwen3.5:4b"

workdir="$(mktemp -d /tmp/family-finance-openclaw-acceptance.XXXXXX)"
chmod 0700 "$workdir"
smoke_secrets_dir="$workdir/secrets"
container_secrets_dir="$workdir/container-secrets"
mkdir -m 0700 "$smoke_secrets_dir"
mkdir -m 0755 "$container_secrets_dir"
env_file="$workdir/acceptance.env"
caddy_root="$workdir/caddy-root.crt"
project="family-finance-openclaw-${GITHUB_RUN_ID:-$$}-${RANDOM}"
hosts_tag="family-finance-openclaw-${GITHUB_RUN_ID:-$$}-${RANDOM}"
hosts_added=0
ollama_container=""
ollama_proxy_pid=""
ollama_proxy_diag="$workdir/ollama-boundary.jsonl"

export FINANCE_ACCEPTANCE_SECRETS_DIR="$container_secrets_dir"

compose=(
  docker compose
  --env-file "$env_file"
  -p "$project"
  -f compose.yaml
  -f compose.openclaw-acceptance.yaml
)

cleanup() {
  set +e
  if [[ -n "$ollama_proxy_pid" ]]; then
    kill "$ollama_proxy_pid" >/dev/null 2>&1 || true
    wait "$ollama_proxy_pid" >/dev/null 2>&1 || true
  fi
  if [[ -n "$ollama_container" ]]; then
    docker rm -f "$ollama_container" >/dev/null 2>&1 || true
  fi
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
printf '%s' "$mcp_token" >"$smoke_secrets_dir/finance-mcp-token"
chmod 0600 "$smoke_secrets_dir/finance-mcp-token"
printf '%s' "$mcp_token" >"$container_secrets_dir/finance-mcp-token"
chmod 0444 "$container_secrets_dir/finance-mcp-token"

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
FINANCE_ACCEPTANCE_SECRETS_DIR=$container_secrets_dir
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
{"name":"Acceptance Checking","category":2,"type":1,"icon":"1","color":"2196F3","currency":"CNY"}
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

npm install --global "openclaw@${openclaw_version}" --no-audit --no-fund >"$workdir/openclaw-install.stdout" 2>"$workdir/openclaw-install.stderr" \
  || fail "pinned OpenClaw installation failed"
installed_openclaw_version="$(openclaw --version 2>/dev/null | tr -d '\r' | tail -n 1)"
[[ "$installed_openclaw_version" == *"${openclaw_version}"* ]] \
  || fail "installed OpenClaw version does not match the pinned release"

ollama_container="${project}-ollama"
docker run -d --name "$ollama_container" --pull always \
  -p 127.0.0.1:11434:11434 "$ollama_image" >"$workdir/ollama-container-id" \
  || fail "Ollama container failed to start"

ollama_ready=0
for _ in $(seq 1 60); do
  if curl --silent --show-error --fail --connect-timeout 2 --max-time 5 \
    http://127.0.0.1:11434/api/tags >"$workdir/ollama-tags.json" 2>/dev/null; then
    ollama_ready=1
    break
  fi
  sleep 2
done
[[ "$ollama_ready" == "1" ]] || fail "Ollama native API did not become ready"

if ! docker exec "$ollama_container" ollama pull "$ollama_model" \
  >"$workdir/ollama-pull.stdout" 2>"$workdir/ollama-pull.stderr"; then
  fail "Ollama acceptance model pull failed"
fi

jq -n --arg model "$ollama_model" '{model:$model,messages:[{role:"user",content:"Reply with exactly: pong"}],stream:false,think:false,keep_alive:"5m"}' \
  >"$workdir/ollama-smoke-request.json"
if ! curl --silent --show-error --fail --connect-timeout 5 --max-time 300 \
  --header 'Content-Type: application/json' \
  --data-binary @"$workdir/ollama-smoke-request.json" \
  http://127.0.0.1:11434/api/chat >"$workdir/ollama-smoke-response.json"; then
  fail "Ollama model smoke failed"
fi
jq -e '.message.content | type == "string" and length > 0' "$workdir/ollama-smoke-response.json" >/dev/null \
  || fail "Ollama model smoke returned no assistant content"

ollama_native_tool_probe() {
  local request="$workdir/ollama-tool-request.json"
  local response="$workdir/ollama-tool-response.json"

  jq -n --arg model "$ollama_model" '
{
  model: $model,
  messages: [
    {
      role: "user",
      content: "Call finance_acceptance_tool_probe exactly once with key=family-finance. Do not answer directly."
    }
  ],
  stream: false,
  think: false,
  keep_alive: "5m",
  tools: [
    {
      type: "function",
      function: {
        name: "finance_acceptance_tool_probe",
        description: "Return a deterministic release-acceptance nonce for the requested key.",
        parameters: {
          type: "object",
          additionalProperties: false,
          properties: {
            key: {
              type: "string",
              enum: ["family-finance"]
            }
          },
          required: ["key"]
        }
      }
    }
  ]
}
' >"$request"

  if ! curl --silent --show-error --fail --connect-timeout 5 --max-time 300 \
    --header 'Content-Type: application/json' \
    --data-binary @"$request" \
    http://127.0.0.1:11434/api/chat >"$response"; then
    fail "Ollama native tool-call preflight request failed"
  fi

  if ! jq -e '
    (.message.tool_calls | type == "array")
    and (.message.tool_calls | length == 1)
    and (.message.tool_calls[0].function.name == "finance_acceptance_tool_probe")
    and (.message.tool_calls[0].function.arguments == {"key":"family-finance"})
  ' "$response" >/dev/null; then
    fail "Ollama model did not emit the required native function call"
  fi

  printf 'ollama_native_tool_call=PASS\n'
}

ollama_native_tool_probe

: >"$ollama_proxy_diag"
chmod 0600 "$ollama_proxy_diag"
OLLAMA_PROXY_UPSTREAM=http://127.0.0.1:11434 \
OLLAMA_PROXY_LISTEN=127.0.0.1:11435 \
OLLAMA_PROXY_DIAG_FILE="$ollama_proxy_diag" \
OLLAMA_PROXY_SHADOW_STRIP_SYSTEM=1 \
  node "$ROOT_DIR/scripts/acceptance/ollama-request-proxy.mjs" \
  >"$workdir/ollama-proxy.stdout" 2>"$workdir/ollama-proxy.stderr" &
ollama_proxy_pid=$!

ollama_proxy_ready=0
for _ in $(seq 1 30); do
  if ! kill -0 "$ollama_proxy_pid" >/dev/null 2>&1; then
    fail "sanitized Ollama boundary proxy exited before readiness"
  fi
  if curl --silent --show-error --fail --connect-timeout 2 --max-time 5 \
    http://127.0.0.1:11435/api/tags >/dev/null 2>&1; then
    ollama_proxy_ready=1
    break
  fi
  sleep 1
done
[[ "$ollama_proxy_ready" == "1" ]] || fail "sanitized Ollama boundary proxy did not become ready"
printf 'ollama_boundary_proxy=PASS\n'

openclaw_home="$workdir/openclaw-home"
openclaw_state="$workdir/openclaw-state"
openclaw_read_config="$workdir/openclaw-read.json"
openclaw_simulation_config="$workdir/openclaw-simulation.json"
mkdir -m 0700 "$openclaw_home" "$openclaw_state"
export OPENCLAW_HOME="$openclaw_home"
export OPENCLAW_STATE_DIR="$openclaw_state"
export OLLAMA_API_KEY="ollama-local"
export FINANCE_MCP_OPENCLAW_TOKEN="$mcp_token"

write_openclaw_agent_config() {
  local config_path="$1"
  local allowed_tool="$2"
  jq -n --arg allowed_tool "$allowed_tool" '
{
  agents: {
    defaults: {
      model: { primary: "ollama/qwen3.5:4b" },
      params: { num_ctx: 32768 }
    }
  },
  models: {
    providers: {
      ollama: {
        baseUrl: "http://127.0.0.1:11435",
        api: "ollama"
      }
    }
  },
  tools: {
    allow: [$allowed_tool]
  },
  mcp: {
    servers: {
      finance: {
        url: "https://finance.localhost/mcp",
        transport: "streamable-http",
        enabled: true,
        connectionTimeoutMs: 10000,
        requestTimeoutMs: 30000,
        headers: {
          Authorization: "Bearer ${FINANCE_MCP_OPENCLAW_TOKEN}"
        }
      }
    }
  }
}
' >"$config_path"
  chmod 0600 "$config_path"
}

write_openclaw_agent_config "$openclaw_read_config" "finance__get_household_overview"
write_openclaw_agent_config "$openclaw_simulation_config" "finance__simulate_purchase"
export OPENCLAW_CONFIG_PATH="$openclaw_read_config"

if ! openclaw models list --provider ollama --json >"$workdir/openclaw-models.json" 2>"$workdir/openclaw-models.stderr"; then
  fail "OpenClaw could not discover the local Ollama provider"
fi
if ! grep -Fq "$ollama_model" "$workdir/openclaw-models.json"; then
  fail "OpenClaw model catalog does not contain the pinned Ollama model"
fi

export FINANCE_ENV_FILE="$env_file"
export FINANCE_MCP_SMOKE_URL="https://finance.localhost/mcp"
export FINANCE_MCP_SMOKE_TOKEN_FILE="$smoke_secrets_dir/finance-mcp-token"
export OPENCLAW_FINANCE_MCP_SERVER="finance"
export OPENCLAW_FINANCE_SMOKE_AGENT_TIMEOUT="300"
export OPENCLAW_FINANCE_SMOKE_MODEL="ollama/${ollama_model}"
export OPENCLAW_FINANCE_SMOKE_CONFIG="$openclaw_read_config"
export OPENCLAW_FINANCE_SMOKE_READ_CONFIG="$openclaw_read_config"
export OPENCLAW_FINANCE_SMOKE_SIMULATION_CONFIG="$openclaw_simulation_config"

bash "$live_smoke"

query_audit_count() {
  local tool_name="$1"
  docker exec -i -e PGPASSWORD="$finance_db_password" "$postgres_cid" \
    psql -X -q -A -t -v ON_ERROR_STOP=1 -U finance_app -d finance \
    -v household_id="$household_id" -v tool_name="$tool_name" <<'SQL' | tr -d '[:space:]'
SELECT count(*)
FROM agent_tool_audits
WHERE household_id = :'household_id'
  AND tool_name = :'tool_name'
  AND status = 'success'
  AND completed_at IS NOT NULL
  AND output_sha256 IS NOT NULL;
SQL
}

read_audit_count="$(query_audit_count get_household_overview)"
simulation_audit_count="$(query_audit_count simulate_purchase)"
[[ "$read_audit_count" =~ ^[0-9]+$ && "$read_audit_count" -ge 1 ]] \
  || fail "successful get_household_overview audit row is missing"
[[ "$simulation_audit_count" =~ ^[0-9]+$ && "$simulation_audit_count" -ge 1 ]] \
  || fail "successful simulate_purchase audit row is missing"

printf 'openclaw_version=%s\n' "$openclaw_version"
printf 'ollama_model=%s\n' "$ollama_model"
printf 'audit_get_household_overview=%s\n' "$read_audit_count"
printf 'audit_simulate_purchase=%s\n' "$simulation_audit_count"
printf 'openclaw_release_acceptance=PASS\n'