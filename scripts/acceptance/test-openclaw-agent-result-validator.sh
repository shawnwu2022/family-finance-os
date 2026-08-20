#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "OpenClaw agent result validator contract failed: $*" >&2
  exit 1
}

validator="scripts/acceptance/openclaw-agent-result-validator.mjs"
live_smoke="scripts/acceptance/openclaw-mcp-live-smoke.sh"
[[ -f "$validator" ]] || fail "agent result validator is missing"
[[ -f "$live_smoke" ]] || fail "live smoke helper is missing"
node --check "$validator" >/dev/null || fail "agent result validator JavaScript syntax is invalid"
grep -Fq 'agent_result_validator="scripts/acceptance/openclaw-agent-result-validator.mjs"' "$live_smoke" \
  || fail "live smoke must declare the retry-aware agent result validator"
grep -Fq 'node "$agent_result_validator" "$output" "$server_name" "$tool_name" "$marker" 2' "$live_smoke" \
  || fail "live smoke must validate agent results with the bounded retry contract"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

write_fixture() {
  local path="$1"
  local calls="$2"
  local failures="$3"
  local visible_text="$4"
  local runtime_tool="$5"
  local summary_tool="$6"

  node - "$path" "$calls" "$failures" "$visible_text" "$runtime_tool" "$summary_tool" <<'NODE'
const fs = require('fs');
const [path, callsRaw, failuresRaw, visibleText, runtimeTool, summaryTool] = process.argv.slice(2);
const calls = Number(callsRaw);
const failures = Number(failuresRaw);
const payload = {
  payloads: [{ text: visibleText }],
  meta: {
    aborted: false,
    finalAssistantVisibleText: visibleText,
    systemPromptReport: {
      tools: {
        entries: [{ name: runtimeTool }],
      },
    },
    toolSummary: {
      calls,
      tools: [summaryTool],
      failures,
    },
  },
};
fs.writeFileSync(path, JSON.stringify(payload));
NODE
}

expect_pass() {
  local fixture="$1"
  if ! node "$validator" "$fixture" finance get_household_overview FINANCE_MCP_READ_OK 2 >/dev/null; then
    fail "validator rejected expected-pass fixture: $fixture"
  fi
}

expect_fail() {
  local fixture="$1"
  if node "$validator" "$fixture" finance get_household_overview FINANCE_MCP_READ_OK 2 >/dev/null 2>&1; then
    fail "validator accepted expected-fail fixture: $fixture"
  fi
}

write_fixture "$workdir/one.json" 1 0 FINANCE_MCP_READ_OK finance__get_household_overview finance__get_household_overview
expect_pass "$workdir/one.json"

# Pinned OpenClaw v2026.7.1-2 permits one same-model idle-timeout retry. The
# acceptance validator therefore tolerates at most two successful calls to the
# one allowed tool while still rejecting loops and unrelated tools.
write_fixture "$workdir/two.json" 2 0 FINANCE_MCP_READ_OK finance__get_household_overview finance__get_household_overview
expect_pass "$workdir/two.json"

write_fixture "$workdir/zero.json" 0 0 FINANCE_MCP_READ_OK finance__get_household_overview finance__get_household_overview
expect_fail "$workdir/zero.json"
write_fixture "$workdir/three.json" 3 0 FINANCE_MCP_READ_OK finance__get_household_overview finance__get_household_overview
expect_fail "$workdir/three.json"
write_fixture "$workdir/failure.json" 1 1 FINANCE_MCP_READ_OK finance__get_household_overview finance__get_household_overview
expect_fail "$workdir/failure.json"
write_fixture "$workdir/wrong-marker.json" 1 0 WRONG finance__get_household_overview finance__get_household_overview
expect_fail "$workdir/wrong-marker.json"
write_fixture "$workdir/wrong-runtime-tool.json" 1 0 FINANCE_MCP_READ_OK finance__get_cashflow finance__get_household_overview
expect_fail "$workdir/wrong-runtime-tool.json"
write_fixture "$workdir/wrong-summary-tool.json" 1 0 FINANCE_MCP_READ_OK finance__get_household_overview finance__get_cashflow
expect_fail "$workdir/wrong-summary-tool.json"

echo "OpenClaw agent result validator contract OK"
