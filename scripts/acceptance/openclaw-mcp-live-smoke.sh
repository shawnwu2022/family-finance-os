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

agent_result_validator="scripts/acceptance/openclaw-agent-result-validator.mjs"
[[ -f "$agent_result_validator" ]] || fail "OpenClaw agent result validator is missing"

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
read_agent_config="${OPENCLAW_FINANCE_SMOKE_READ_CONFIG:-$agent_config}"
simulation_agent_config="${OPENCLAW_FINANCE_SMOKE_SIMULATION_CONFIG:-$agent_config}"
ollama_preflight="${OPENCLAW_FINANCE_SMOKE_OLLAMA_PREFLIGHT:-0}"
[[ "$ollama_preflight" == "0" || "$ollama_preflight" == "1" ]] || fail "OPENCLAW_FINANCE_SMOKE_OLLAMA_PREFLIGHT must be 0 or 1"
for config_path in "$agent_config" "$read_agent_config" "$simulation_agent_config"; do
  if [[ -n "$config_path" ]]; then
    [[ -f "$config_path" ]] || fail "OpenClaw config not found: $config_path"
  fi
done
if [[ -n "$agent_config" ]]; then
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

emit_agent_diagnostics() {
  local label="$1"
  local output="$2"
  node - "$label" "$output" <<'NODE'
const fs = require('fs');
const [label, path] = process.argv.slice(2);
let payload;
try {
  payload = JSON.parse(fs.readFileSync(path, 'utf8'));
} catch {
  console.error(`openclaw_agent_diag label=${label} json_valid=false`);
  process.exit(0);
}
const payloads = Array.isArray(payload.payloads) ? payload.payloads : [];
const textPayloadCount = payloads.filter((entry) => entry && typeof entry.text === 'string' && entry.text.trim()).length;
const meta = payload && typeof payload.meta === 'object' && payload.meta !== null ? payload.meta : {};
const agentMeta = meta && typeof meta.agentMeta === 'object' && meta.agentMeta !== null ? meta.agentMeta : {};
const toolSummary = meta && typeof meta.toolSummary === 'object' && meta.toolSummary !== null ? meta.toolSummary : {};
const systemPromptReport = meta && typeof meta.systemPromptReport === 'object' && meta.systemPromptReport !== null
  ? meta.systemPromptReport
  : {};
const reportTools = systemPromptReport && typeof systemPromptReport.tools === 'object' && systemPromptReport.tools !== null
  ? systemPromptReport.tools
  : {};
const runtimeToolNames = Array.isArray(reportTools.entries)
  ? reportTools.entries
      .map((entry) => (entry && typeof entry.name === 'string' ? entry.name : ''))
      .filter(Boolean)
  : [];
const safe = {
  label,
  payloadCount: payloads.length,
  textPayloadCount,
  topKeys: payload && typeof payload === 'object' ? Object.keys(payload).sort() : [],
  metaKeys: Object.keys(meta).sort(),
  agentMetaKeys: Object.keys(agentMeta).sort(),
  provider: typeof agentMeta.provider === 'string' ? agentMeta.provider : undefined,
  model: typeof agentMeta.model === 'string' ? agentMeta.model : undefined,
  stopReason: typeof meta.stopReason === 'string' ? meta.stopReason : undefined,
  aborted: typeof meta.aborted === 'boolean' ? meta.aborted : undefined,
  hasError: Boolean(meta.error),
  hasFinalAssistantVisibleText: typeof meta.finalAssistantVisibleText === 'string' && Boolean(meta.finalAssistantVisibleText.trim()),
  runtimeToolCount: runtimeToolNames.length,
  runtimeToolNames,
  toolCalls: typeof toolSummary.calls === 'number' ? toolSummary.calls : undefined,
  toolNames: Array.isArray(toolSummary.tools) ? toolSummary.tools : undefined,
  toolFailures: typeof toolSummary.failures === 'number' ? toolSummary.failures : undefined,
  durationMs: typeof meta.durationMs === 'number' ? meta.durationMs : undefined,
};
console.error(`openclaw_agent_diag ${JSON.stringify(safe)}`);
NODE
  printf 'openclaw_agent_output_sha256 label=%s sha256=%s\n' "$label" "$(sha256sum "$output" | awk '{print $1}')" >&2
}

run_agent_check() {
  local label="$1"
  local tool_name="$2"
  local marker="$3"
  local prompt="$4"
  local digest_path="$5"
  local turn_config="$6"
  local output="$workdir/${label}.json"
  local session_id="finance-acceptance-${label}"

  if [[ -n "$turn_config" ]]; then
    if ! OPENCLAW_CONFIG_PATH="$turn_config" openclaw agent --local --agent main --session-id "$session_id" --message "$prompt" "${agent_args[@]}" >"$output" 2>"$workdir/${label}.stderr"; then
      fail "OpenClaw agent $label turn failed"
    fi
  elif ! openclaw agent --local --agent main --session-id "$session_id" --message "$prompt" "${agent_args[@]}" >"$output" 2>"$workdir/${label}.stderr"; then
    fail "OpenClaw agent $label turn failed"
  fi

  if ! node "$agent_result_validator" "$output" "$server_name" "$tool_name" "$marker" 2 >/dev/null; then
    emit_agent_diagnostics "$label" "$output"
    fail "OpenClaw agent $label result validation failed"
  fi

  sha256sum "$output" | awk '{print $1}' >"$digest_path"
}

ollama_native_finance_tool_probe() {
  [[ "$ollama_preflight" == "1" ]] || return 0
  [[ "$agent_model" == ollama/* ]] || fail "Ollama Finance preflight requires an ollama/* acceptance model"

  local native_model="${agent_model#ollama/}"
  local expected_tool="${server_name}__get_household_overview"
  local description="Get the current deterministic Finance Core household overview. Preserve quality and warning metadata."
  local mode stream_json request response

  for mode in nonstream stream; do
    if [[ "$mode" == "nonstream" ]]; then
      stream_json=false
    else
      stream_json=true
    fi
    request="$workdir/ollama-finance-${mode}-request.json"
    response="$workdir/ollama-finance-${mode}-response.json"

    jq -n \
      --arg model "$native_model" \
      --arg prompt "$read_prompt" \
      --arg tool "$expected_tool" \
      --arg description "$description" \
      --argjson stream "$stream_json" '
{
  model: $model,
  messages: [{role: "user", content: $prompt}],
  stream: $stream,
  think: false,
  keep_alive: "5m",
  tools: [
    {
      type: "function",
      function: {
        name: $tool,
        description: $description,
        parameters: {
          type: "object",
          additionalProperties: false
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
      fail "Ollama exact Finance ${mode} tool-call preflight request failed"
    fi

    if [[ "$mode" == "nonstream" ]]; then
      if ! jq -e --arg tool "$expected_tool" '
        (.message.tool_calls | type == "array")
        and (.message.tool_calls | length == 1)
        and (.message.tool_calls[0].function.name == $tool)
        and ((.message.tool_calls[0].function.arguments // {}) == {})
      ' "$response" >/dev/null; then
        fail "Ollama model did not emit the exact Finance non-stream function call"
      fi
      printf 'ollama_native_finance_tool_call_nonstream=PASS\n'
    else
      if ! jq -s -e --arg tool "$expected_tool" '
        [.[].message.tool_calls[]?] as $calls
        | ($calls | length == 1)
          and ($calls[0].function.name == $tool)
          and (($calls[0].function.arguments // {}) == {})
      ' "$response" >/dev/null; then
        fail "Ollama model did not emit the exact Finance streaming function call"
      fi
      printf 'ollama_native_finance_tool_call_stream=PASS\n'
    fi
  done
}

read_prompt="Acceptance check. You MUST call the only available tool, ${server_name}__get_household_overview, exactly once. Do not answer from memory or prior knowledge. Only after that tool succeeds, reply exactly FINANCE_MCP_READ_OK. If the tool is unavailable or fails, do not output that marker."
ollama_native_finance_tool_probe
run_agent_check read get_household_overview FINANCE_MCP_READ_OK "$read_prompt" "$workdir/read.digest" "$read_agent_config"
read_digest="$(cat "$workdir/read.digest")"

simulation_prompt='Acceptance check. You MUST call the only available tool, '"${server_name}"'__simulate_purchase, exactly once with the exact JSON object {"amount_minor":"100","currency":"CNY"}. Do not transform, scale, convert, or otherwise reinterpret these values. Do not answer from memory or prior knowledge. Only after that tool succeeds, reply exactly FINANCE_MCP_SIM_OK. If the tool is unavailable or fails, do not output that marker.'
run_agent_check simulation simulate_purchase FINANCE_MCP_SIM_OK "$simulation_prompt" "$workdir/simulation.digest" "$simulation_agent_config"
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