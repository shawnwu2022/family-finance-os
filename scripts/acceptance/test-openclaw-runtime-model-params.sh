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

echo "OpenClaw runtime model params contract OK"
