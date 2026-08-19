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
workflow=".github/workflows/openclaw-release-acceptance.yml"

bash -n "$provisioner" || fail "provisioner shell syntax is invalid"

grep -Fq 'scripts/acceptance/openclaw-mcp-live-smoke.sh' "$provisioner" || fail "provisioner must invoke the real OpenClaw live smoke"
grep -Fq 'agent_tool_audits' "$provisioner" || fail "provisioner must verify persisted agent tool audits"
grep -Fq 'openclaw agent exec' "scripts/acceptance/openclaw-mcp-live-smoke.sh" || fail "release acceptance must retain real OpenClaw agent turns"
grep -Fq 'bash scripts/acceptance/openclaw-ephemeral-release-acceptance.sh' "$workflow" || fail "release workflow must delegate to the repository provisioner"

if grep -Eq 'actions/setup-go|go test|github.com/modelcontextprotocol/go-sdk' "$workflow"; then
  fail "release workflow must not substitute SDK/unit verification for real OpenClaw acceptance"
fi

if grep -Eq 'Authorization: Bearer [^$<{]' "$workflow" "$provisioner"; then
  fail "literal bearer credentials are forbidden in release acceptance files"
fi

echo "OpenClaw release acceptance contract OK"
