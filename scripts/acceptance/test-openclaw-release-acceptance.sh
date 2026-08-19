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

if grep -Eq 'actions/setup-go|go test|github.com/modelcontextprotocol/go-sdk' "$workflow"; then
  fail "release workflow must not substitute SDK/unit verification for real OpenClaw acceptance"
fi

if grep -Eq 'Authorization: Bearer [^$<{]' "$workflow" "$provisioner"; then
  fail "literal bearer credentials are forbidden in release acceptance files"
fi

echo "OpenClaw release acceptance contract OK"
