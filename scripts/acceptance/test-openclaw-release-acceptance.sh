#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "OpenClaw release acceptance contract failed: $*" >&2
  exit 1
}

required=(
  compose.openclaw-acceptance.yaml
  Caddyfile.acceptance
  scripts/acceptance/openclaw-ephemeral-release-acceptance.sh
  .github/workflows/openclaw-release-acceptance.yml
)
for path in "${required[@]}"; do
  [[ -f "$path" ]] || fail "required file is missing: $path"
done

provisioner="scripts/acceptance/openclaw-ephemeral-release-acceptance.sh"
live_smoke="scripts/acceptance/openclaw-mcp-live-smoke.sh"
workflow=".github/workflows/openclaw-release-acceptance.yml"

bash -n "$provisioner" || fail "provisioner shell syntax is invalid"
bash -n "$live_smoke" || fail "OpenClaw live smoke shell syntax is invalid"

grep -Fq "$live_smoke" "$provisioner" || fail "provisioner must reference the real OpenClaw live smoke"
grep -Fq 'agent_tool_audits' "$provisioner" || fail "provisioner must verify persisted agent tool audits"
grep -Fq 'bash scripts/acceptance/test-openclaw-release-acceptance.sh' "$workflow" || fail "release workflow must gate the expensive run on the static acceptance contract"
grep -Fq 'bash scripts/acceptance/openclaw-ephemeral-release-acceptance.sh' "$workflow" || fail "release workflow must delegate to the repository provisioner"

# The pinned stable OpenClaw v2026.7.1-2 CLI exposes `openclaw agent --local`.
grep -Fq 'openclaw agent --local' "$live_smoke" || fail "live smoke must use the pinned stable OpenClaw local agent CLI"
grep -Fq -- '--agent main' "$live_smoke" || fail "live smoke must select the stable main agent explicitly"
grep -Fq -- '--message "$prompt"' "$live_smoke" || fail "live smoke must pass the acceptance prompt through --message"
if grep -Fq 'openclaw agent exec' "$live_smoke"; then
  fail "pinned OpenClaw v2026.7.1-2 does not expose agent exec"
fi
if grep -Fq -- '--code-mode direct' "$live_smoke"; then
  fail "pinned OpenClaw v2026.7.1-2 does not expose --code-mode direct"
fi

# Agent validation must execute in the parent shell. A failing parser inside command
# substitution can otherwise be masked by a later successful sha256 command.
if grep -Fq 'read_digest="$(run_agent_check' "$live_smoke" || grep -Fq 'simulation_digest="$(run_agent_check' "$live_smoke"; then
  fail "agent marker validation must not run inside command substitution"
fi
grep -Fq 'run_agent_check read get_household_overview FINANCE_MCP_READ_OK "$read_prompt" "$workdir/read.digest"' "$live_smoke" || fail "read agent validation must bind the exact Finance tool and write its digest only after validation succeeds"
grep -Fq 'run_agent_check simulation simulate_purchase FINANCE_MCP_SIM_OK "$simulation_prompt" "$workdir/simulation.digest"' "$live_smoke" || fail "simulation agent validation must bind the exact Finance tool and write its digest only after validation succeeds"

# Pinned stable OpenClaw stores the final assistant-visible answer and attempt tool
# summary under result.meta. Do not infer successful agent behavior from payloads alone.
grep -Fq 'finalAssistantVisibleText' "$live_smoke" || fail "agent validation must use stable meta.finalAssistantVisibleText"
grep -Fq 'toolSummary' "$live_smoke" || fail "agent validation must require stable meta.toolSummary"
grep -Fq 'toolSummary.tools' "$live_smoke" || fail "agent validation must verify the exact namespaced MCP tool"
grep -Fq 'toolSummary.calls' "$live_smoke" || fail "agent validation must require at least one tool call"
grep -Fq 'toolSummary.failures' "$live_smoke" || fail "agent validation must reject tool failures"

# Keep the local 4B model's active agent tool surface intentionally narrow. The
# separate MCP probe still verifies the full 12-tool server surface; these two are
# the actual read/simulation tools exercised by agent turns and persisted audits.
grep -Fq 'tools: {' "$provisioner" || fail "OpenClaw acceptance config must declare an agent tool policy"
grep -Fq 'allow: [' "$provisioner" || fail "OpenClaw acceptance config must use a narrow tool allowlist"
grep -Fq '"finance__get_household_overview"' "$provisioner" || fail "OpenClaw agent allowlist must include the read acceptance tool"
grep -Fq '"finance__simulate_purchase"' "$provisioner" || fail "OpenClaw agent allowlist must include the simulation acceptance tool"

# Task 2: production-shaped bootstrap must be explicit and fail closed.
grep -Fq 'docker compose' "$provisioner" || fail "provisioner must run the real Docker Compose stack"
grep -Fq 'goose_linux_x86_64' "$provisioner" || fail "provisioner must install pinned goose for Finance migrations"
grep -Fq 'ca18112e2438b3ad608af9a5938beafd01fa36a4a19a3edbe4f29226ca5c8533' "$provisioner" || fail "provisioner must verify the pinned goose checksum"
grep -Fq 'finance.localhost' "$provisioner" || fail "provisioner must use the loopback Finance HTTPS hostname"
grep -Fq '/data/caddy/pki/authorities/local/root.crt' "$provisioner" || fail "provisioner must obtain Caddy local CA trust material"
grep -Fq 'goose -dir' "$provisioner" || fail "provisioner must migrate the Finance database"
grep -Fq 'INSERT INTO households' "$provisioner" || fail "provisioner must create an acceptance household"
grep -Fq 'INSERT INTO household_policies' "$provisioner" || fail "provisioner must create the household policy required by simulations"
grep -Fq '/ezbookkeeping/ezbookkeeping userdata user-add' "$provisioner" || fail "provisioner must create a real ezBookkeeping user through its CLI"
grep -Fq 'user-session-new' "$provisioner" || fail "provisioner must create a real ezBookkeeping API session"
grep -Fq '\[NewToken\]' "$provisioner" || fail "provisioner must parse the documented ezBookkeeping CLI token marker"
grep -Fq 'accounts/add.json' "$provisioner" || fail "provisioner must seed ledger data through the real ezBookkeeping Account API"
grep -Fq '/healthz' "$provisioner" || fail "provisioner must verify Finance health through the HTTPS edge"

# Task 3: the remaining production gate must be real OpenClaw + local Ollama.
grep -Fq 'openclaw_version="2026.7.1-2"' "$provisioner" || fail "provisioner must pin the OpenClaw acceptance version"
grep -Fq 'npm install --global "openclaw@${openclaw_version}"' "$provisioner" || fail "provisioner must install the pinned real OpenClaw CLI"
grep -Fq 'ollama_image="ollama/ollama:0.32.5"' "$provisioner" || fail "provisioner must pin the Ollama runtime image"
grep -Fq 'ollama_model="qwen3.5:4b"' "$provisioner" || fail "provisioner must pin the tool-capable local acceptance model"

# Before paying the cost of full OpenClaw turns, prove that the pinned model handles
# the exact read tool surface used by acceptance on both native Ollama response modes.
# OpenClaw's pinned native adapter uses streaming /api/chat, while non-stream is the
# control that separates model/schema behavior from streaming compatibility.
grep -Fq 'ollama_native_finance_tool_probe()' "$live_smoke" || fail "live smoke must define the exact Finance Ollama tool-call preflight"
grep -Fq 'OPENCLAW_FINANCE_SMOKE_OLLAMA_PREFLIGHT' "$live_smoke" || fail "Finance Ollama preflight must be acceptance-only"
grep -Fq 'OPENCLAW_FINANCE_SMOKE_OLLAMA_PREFLIGHT' "$workflow" || fail "ephemeral release workflow must enable the Finance Ollama preflight"
grep -Fq 'finance__get_household_overview' "$live_smoke" || fail "native Ollama preflight must use the exact namespaced Finance read tool"
grep -Fq 'Get the current deterministic Finance Core household overview. Preserve quality and warning metadata.' "$live_smoke" || fail "native Ollama preflight must use the real Finance read-tool description"
grep -Fq 'additionalProperties: false' "$live_smoke" || fail "native Ollama preflight must use the real empty-object Finance read schema"
grep -Fq 'message.tool_calls' "$live_smoke" || fail "native Ollama preflight must validate message.tool_calls"
grep -Fq 'ollama_native_finance_tool_call_nonstream=PASS' "$live_smoke" || fail "non-stream Finance tool-call preflight marker is missing"
grep -Fq 'ollama_native_finance_tool_call_stream=PASS' "$live_smoke" || fail "streaming Finance tool-call preflight marker is missing"

grep -Fq 'OPENCLAW_CONFIG_PATH' "$provisioner" || fail "provisioner must isolate the OpenClaw config"
grep -Fq 'OPENCLAW_STATE_DIR' "$provisioner" || fail "provisioner must isolate OpenClaw state"
grep -Fq 'OPENCLAW_HOME' "$provisioner" || fail "provisioner must isolate OpenClaw home"
grep -Fq 'OLLAMA_API_KEY="ollama-local"' "$provisioner" || fail "provisioner must use the local Ollama auth marker"
grep -Fq 'http://127.0.0.1:11434' "$provisioner" || fail "OpenClaw acceptance must use Ollama native loopback API"
grep -Fq 'transport: "streamable-http"' "$provisioner" || fail "OpenClaw config must use canonical Streamable HTTP MCP transport"
grep -Fq 'Bearer ${FINANCE_MCP_OPENCLAW_TOKEN}' "$provisioner" || fail "OpenClaw MCP bearer must use environment interpolation"
if grep -Fq 'catalogRefresh' "$provisioner"; then
  fail "pinned OpenClaw v2026.7.1-2 config must not use non-stable models.catalogRefresh"
fi
grep -Eq '^[[:space:]]*bash[[:space:]]+"\$live_smoke"[[:space:]]*$' "$provisioner" || fail "provisioner must actually execute the real OpenClaw live smoke"
grep -Eq '^[[:space:]]*read_audit_count="\$\(query_audit_count get_household_overview\)"' "$provisioner" || fail "provisioner must verify the read-tool audit row"
grep -Eq '^[[:space:]]*simulation_audit_count="\$\(query_audit_count simulate_purchase\)"' "$provisioner" || fail "provisioner must verify the simulation-tool audit row"
grep -Fq "status = 'success'" "$provisioner" || fail "provisioner must require successful audit completion"
grep -Fq 'docker exec -i -e PGPASSWORD=' "$provisioner" || fail "audit verification must feed SQL through psql stdin for variable expansion"
grep -Fq "<<'SQL'" "$provisioner" || fail "audit verification must use psql stdin rather than an unexpanded -c query"
grep -Fq 'openclaw_release_acceptance=PASS' "$provisioner" || fail "provisioner must emit the final sanitized PASS marker"

if grep -Eq 'actions/setup-go|go test|github.com/modelcontextprotocol/go-sdk' "$workflow"; then
  fail "release workflow must not substitute SDK/unit verification for real OpenClaw acceptance"
fi

if grep -Eq 'Authorization: Bearer [^$<{]' "$workflow" "$provisioner"; then
  fail "literal bearer credentials are forbidden in release acceptance files"
fi

echo "OpenClaw release acceptance contract OK"
