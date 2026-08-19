#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "OpenClaw ephemeral release acceptance failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

live_smoke="scripts/acceptance/openclaw-mcp-live-smoke.sh"
audit_table="agent_tool_audits"
[[ -f "$live_smoke" ]] || fail "live smoke helper is missing"
[[ -n "$audit_table" ]] || fail "audit table contract is missing"

# Task 2/3 fill in the real production-shaped bootstrap and then invoke:
# bash scripts/acceptance/openclaw-mcp-live-smoke.sh
# followed by a scoped PostgreSQL query against agent_tool_audits.
fail "ephemeral Finance/OpenClaw environment is not implemented yet"
