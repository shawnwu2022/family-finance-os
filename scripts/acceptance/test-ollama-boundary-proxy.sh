#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "Ollama boundary proxy contract failed: $*" >&2
  exit 1
}

proxy="scripts/acceptance/ollama-request-proxy.mjs"
provisioner="scripts/acceptance/openclaw-ephemeral-release-acceptance.sh"
workflow=".github/workflows/openclaw-release-acceptance.yml"

[[ -f "$proxy" ]] || fail "sanitized proxy is missing: $proxy"
node --check "$proxy" >/dev/null || fail "sanitized proxy JavaScript syntax is invalid"

grep -Fq '127.0.0.1:11435' "$provisioner" || fail "OpenClaw acceptance must route native Ollama through the diagnostic loopback proxy"
grep -Fq 'OLLAMA_PROXY_UPSTREAM=http://127.0.0.1:11434' "$provisioner" || fail "proxy upstream must remain the pinned local Ollama endpoint"
grep -Fq 'OLLAMA_PROXY_DIAG_FILE=' "$provisioner" || fail "provisioner must isolate the sanitized boundary diagnostic file"
grep -Fq 'models:' "$provisioner" || fail "OpenClaw config must declare the diagnostic Ollama provider route"
grep -Fq 'baseUrl: "http://127.0.0.1:11435"' "$provisioner" || fail "OpenClaw provider must use the loopback diagnostic proxy"
grep -Fq 'api: "ollama"' "$provisioner" || fail "diagnostic route must preserve native Ollama API semantics"

grep -Fq 'messageContentChars' "$proxy" || fail "proxy must record message lengths rather than message content"
grep -Fq 'toolNames' "$proxy" || fail "proxy must record model-facing tool names"
grep -Fq 'toolSchemaSha256' "$proxy" || fail "proxy must hash tool schemas rather than logging raw definitions"
grep -Fq 'responseToolCallCount' "$proxy" || fail "proxy must record whether raw Ollama responses contain tool calls"
grep -Fq 'responseToolNames' "$proxy" || fail "proxy must record raw Ollama tool names"

grep -Fq 'shadowNoSystemToolCallCount' "$proxy" || fail "proxy must compare the same request without system messages"
grep -Fq 'shadowNoSystemToolNames' "$proxy" || fail "proxy must record shadow tool names only"
grep -Fq 'shadowNoSystemAttempts' "$proxy" || fail "proxy must record the number of no-system shadow attempts"
grep -Fq 'OLLAMA_PROXY_SHADOW_STRIP_SYSTEM' "$proxy" || fail "no-system shadow probing must be explicitly opt-in"
grep -Fq 'OLLAMA_PROXY_SHADOW_STRIP_SYSTEM=1' "$provisioner" || fail "release diagnostic must explicitly enable the no-system shadow probe"

if grep -Eq 'messageContent[^C]|systemPrompt[^C]|assistantText|toolOutput|Authorization|Bearer|FINANCE_MCP_OPENCLAW_TOKEN' "$proxy"; then
  fail "proxy source must not define raw prompt/result/credential diagnostic fields"
fi

grep -Fq 'bash scripts/acceptance/test-ollama-boundary-proxy.sh' "$workflow" || fail "release workflow must fail fast on the boundary proxy contract"

echo "Ollama boundary proxy contract OK"
