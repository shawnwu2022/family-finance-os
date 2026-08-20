#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "OpenClaw simulation prompt contract failed: $*" >&2
  exit 1
}

live_smoke="scripts/acceptance/openclaw-mcp-live-smoke.sh"
[[ -f "$live_smoke" ]] || fail "live smoke helper is missing"
bash -n "$live_smoke" || fail "live smoke shell syntax is invalid"

# simulate_purchase expects amount_minor as a decimal-digit JSON string and a
# three-character currency. Keep the acceptance turn deterministic: the model
# must receive the exact values rather than infer, scale, or convert them.
grep -Fq '{"amount_minor":"100","currency":"CNY"}' "$live_smoke" \
  || fail "simulation prompt must contain the exact schema-aligned JSON object"
grep -Fq 'Do not transform, scale, convert, or otherwise reinterpret these values.' "$live_smoke" \
  || fail "simulation prompt must forbid value reinterpretation"

echo "OpenClaw simulation prompt contract OK"
