#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "OpenClaw runtime model params contract failed: $*" >&2
  exit 1
}

provisioner="scripts/acceptance/openclaw-ephemeral-release-acceptance.sh"
[[ -f "$provisioner" ]] || fail "acceptance provisioner is missing"
bash -n "$provisioner" || fail "acceptance provisioner shell syntax is invalid"

# OpenClaw v2026.7.1-2 resolves agents.defaults.params as generic stream params.
# Native Ollama num_ctx is a model runtime parameter and must live on the exact
# provider/model entry so it reaches model.params and becomes options.num_ctx.
if grep -Eq '^      params: \{ num_ctx: 32768 \}$' "$provisioner"; then
  fail "num_ctx must not use global agents.defaults.params"
fi

awk '
  /model: \{ primary: "ollama\/qwen3\.5:4b" \}/ { saw_primary=1; next }
  saw_primary && /models: \{/ { saw_models=1; next }
  saw_models && /"ollama\/qwen3\.5:4b": \{/ { saw_entry=1; next }
  saw_entry && /params: \{ num_ctx: 32768 \}/ { found=1 }
  END { exit(found ? 0 : 1) }
' "$provisioner" || fail "qwen3.5:4b must declare per-model params.num_ctx=32768"

# The pinned stable runtime applies an implicit 120s LLM idle watchdog when the
# CLI supplies a bounded run timeout. Slow local/self-hosted providers can opt
# into a larger provider request timeout; keep the whole agent run larger so the
# provider budget does not consume the entire tool-call + final-answer turn.
grep -Fq 'timeoutSeconds: 300' "$provisioner" \
  || fail "Ollama provider must declare timeoutSeconds=300 for slow local inference"
grep -Fq 'OPENCLAW_FINANCE_SMOKE_AGENT_TIMEOUT="600"' "$provisioner" \
  || fail "release acceptance agent run timeout must be 600 seconds"

echo "OpenClaw runtime model params contract OK"
