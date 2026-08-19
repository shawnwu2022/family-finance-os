#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "OpenClaw MCP live smoke failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

ENV_FILE="${FINANCE_ENV_FILE:-$ROOT_DIR/.env}"
[[ -f "$ENV_FILE" ]] || fail "environment file not found: $ENV_FILE"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

for command_name in openclaw curl sha256sum node; do
  command -v "$command_name" >/dev/null || fail "$command_name is required"
done

server_name="${OPENCLAW_FINANCE_MCP_SERVER:-finance}"
[[ "$server_name" =~ ^[A-Za-z0-9._-]+$ ]] || fail "OPENCLAW_FINANCE_MCP_SERVER contains unsupported characters"

if [[ -n "${FINANCE_MCP_SMOKE_URL:-}" ]]; then
  endpoint="${FINANCE_MCP_SMOKE_URL%/}"
else
  : "${FINANCE_DOMAIN:?FINANCE_DOMAIN or FINANCE_MCP_SMOKE_URL is required}"
  endpoint="https://${FINANCE_DOMAIN%/}/mcp"
fi
[[ "$endpoint" == https://* ]] || fail "live MCP acceptance requires HTTPS"

token_file="${FINANCE_MCP_SMOKE_TOKEN_FILE:-$ROOT_DIR/secrets/finance-mcp-token}"
[[ -f "$token_file" ]] || fail "MCP bearer file not found: $token_file"
[[ -r "$token_file" ]] || fail "MCP bearer file is not readable: $token_file"
token="$(cat "$token_file")"
[[ -n "$token" ]] || fail "MCP bearer file is empty"
if printf '%s' "$token" | grep -q '[[:space:]]'; then
  fail "MCP bearer contains whitespace"
fi

agent_timeout="${OPENCLAW_FINANCE_SMOKE_AGENT_TIMEOUT:-120}"
[[ "$agent_timeout" =~ ^[1-9][0-9]*$ ]] || fail "OPENCLAW_FINANCE_SMOKE_AGENT_TIMEOUT must be a positive integer"
agent_model="${OPENCLAW_FINANCE_SMOKE_MODEL:-}"
agent_config="${OPENCLAW_FINANCE_SMOKE_CONFIG:-}"
if [[ -n "$agent_config" ]]; then
  [[ -f "$agent_config" ]] || fail "OpenClaw config not found: $agent_config"
  export OPENCLAW_CONFIG_PATH="$agent_config"
fi

expected_tools=(
  generate_monthly_report
  get_asset_allocation
  get_budget_status
  get_cashflow
  get_debt_status
  get_goal_status
  get_household_overview
  get_safe_to_spend
  get_spending_analysis
  simulate_extra_debt_payment
  simulate_goal
  simulate_purchase
)

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

doctor_json="$workdir/doctor.json"
probe_json="$workdir/probe.json"
if ! openclaw mcp doctor "$server_name" --probe --json >"$doctor_json" 2>"$workdir/doctor.stderr"; then
  fail "openclaw mcp doctor --probe failed"
fi
if ! openclaw mcp probe "$server_name" --json >"$probe_json" 2>"$workdir/probe.stderr"; then
  fail "openclaw mcp probe failed"
fi

node - "$probe_json" "$server_name" "$endpoint" "${expected_tools[@]}" <<'NODE'
const fs = require('fs');
const [path, serverName, endpoint, ...expected] = process.argv.slice(2);
const payload = JSON.parse(fs.readFileSync(path, 'utf8'));
const server = payload.servers && payload.servers[serverName];
if (!server) throw new Error('selected MCP server is missing from probe result');
if (server.tools !== expected.length) throw new Error(`tool count ${server.tools} != ${expected.length}`);
if (server.resources !== undefined) throw new Error('MCP server unexpectedly advertises resources');
if (server.prompts !== undefined) throw new Error('MCP server unexpectedly advertises prompts');
if (!Array.isArray(payload.diagnostics) || payload.diagnostics.length !== 0) throw new Error('MCP probe contains diagnostics');
if (typeof server.launch !== 'string' || !server.launch.includes(endpoint)) throw new Error('OpenClaw MCP server points at a different endpoint');
const got = new Set(Array.isArray(payload.tools) ? payload.tools : []);
const wanted = expected.map((name) => `${serverName}__${name}`);
if (got.size !== wanted.length) throw new Error(`namespaced tool count ${got.size} != ${wanted.length}`);
for (const name of wanted) {
  if (!got.has(name)) throw new Error(`missing tool ${name}`);
}
NODE

doctor_digest="$(sha256sum "$doctor_json" | awk '{print $1}')"
probe_digest="$(sha256sum "$probe_json" | awk '{print $1}')"

missing_status="$(curl --silent --show-error --connect-timeout 10 --max-time 30 --proto '=https' --tlsv1.2 \
  --request POST --header 'Content-Type: application/json' --data '{}' \
  --output /dev/null --write-out '%{http_code}' "$endpoint")"
[[ "$missing_status" == "401" ]] || fail "missing bearer returned HTTP $missing_status, want 401"

wrong_status="$(curl --silent --show-error --connect-timeout 10 --max-time 30 --proto '=https' --tlsv1.2 \
  --request POST --header 'Authorization: Bearer definitely-not-the-finance-token' \
  --header 'Content-Type: application/json' --data '{}' \
  --output /dev/null --write-out '%{http_code}' "$endpoint")"
[[ "$wrong_status" == "401" ]] || fail "invalid bearer returned HTTP $wrong_status, want 401"

escaped_token="${token//\\/\\\\}"
escaped_token="${escaped_token//\"/\\\"}"
printf 'header = "Authorization: Bearer %s"\n' "$escaped_token" >"$workdir/auth.curl"
origin_status="$(curl --config "$workdir/auth.curl" --silent --show-error --connect-timeout 10 --max-time 30 --proto '=https' --tlsv1.2 \
  --request POST --header 'Origin: https://untrusted.invalid' \
  --header 'Content-Type: application/json' --data '{}' \
  --output /dev/null --write-out '%{http_code}' "$endpoint")"
[[ "$origin_status" == "403" ]] || fail "untrusted Origin returned HTTP $origin_status, want 403"

agent_args=(--json --timeout "$agent_timeout" --thinking off)
if [[ -n "$agent_model" ]]; then
  agent_args+=(--model "$agent_model")
fi

run_agent_check() {
  local label="$1"
  local marker="$2"
  local prompt="$3"
  local digest_path="$4"
  local output="$workdir/${label}.json"

  if ! openclaw agent --local --agent main --message "$prompt" "${agent_args[@]}" >"$output" 2>"$workdir/${label}.stderr"; then
    fail "OpenClaw agent $label turn failed"
  fi

  if ! node - "$output" "$marker" <<'NODE'
const fs = require('fs');
const [path, marker] = process.argv.slice(2);
const payload = JSON.parse(fs.readFileSync(path, 'utf8'));
const payloads = Array.isArray(payload.payloads) ? payload.payloads : [];
const texts = payloads
  .map((entry) => (entry && typeof entry.text === 'string' ? entry.text.trim() : ''))
  .filter(Boolean);
if (texts.length === 0) throw new Error('agent result contains no assistant text payload');
if (texts[texts.length - 1] !== marker) throw new Error('agent final assistant marker does not match');
NODE
  then
    fail "OpenClaw agent $label result validation failed"
  fi

  sha256sum "$output" | awk '{print $1}' >"$digest_path"
}

read_prompt="Acceptance check. You MUST call the OpenClaw-managed MCP tool ${server_name}__get_household_overview exactly once. Do not use shell, browser, filesystem, memory, or any other tool. Only after that tool succeeds, reply exactly FINANCE_MCP_READ_OK. If the tool is unavailable or fails, do not output that marker."
run_agent_check read FINANCE_MCP_READ_OK "$read_prompt" "$workdir/read.digest"
read_digest="$(cat "$workdir/read.digest")"

simulation_prompt="Acceptance check. You MUST call the OpenClaw-managed MCP tool ${server_name}__simulate_purchase exactly once with amount_minor=100 and currency=CNY. Do not use shell, browser, filesystem, memory, or any other tool. Only after that tool succeeds, reply exactly FINANCE_MCP_SIM_OK. If the tool is unavailable or fails, do not output that marker."
run_agent_check simulation FINANCE_MCP_SIM_OK "$simulation_prompt" "$workdir/simulation.digest"
simulation_digest="$(cat "$workdir/simulation.digest")"

printf 'openclaw_finance_tools=%d\n' "${#expected_tools[@]}"
printf 'missing_bearer_status=%s\n' "$missing_status"
printf 'invalid_bearer_status=%s\n' "$wrong_status"
printf 'untrusted_origin_status=%s\n' "$origin_status"
printf 'doctor_sha256=%s\n' "$doctor_digest"
printf 'probe_sha256=%s\n' "$probe_digest"
printf 'read_agent_sha256=%s\n' "$read_digest"
printf 'simulation_agent_sha256=%s\n' "$simulation_digest"
printf 'openclaw_mcp_live_smoke=PASS\n'
