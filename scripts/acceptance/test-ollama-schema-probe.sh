#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "Ollama schema probe contract failed: $*" >&2
  exit 1
}

probe="scripts/acceptance/ollama-schema-probe.sh"

[[ -f "$probe" ]] || fail "probe script is missing: $probe"
bash -n "$probe" || fail "probe shell syntax is invalid"

grep -Fq 'ollama/ollama:0.32.5' "$probe" || fail "probe must pin the acceptance Ollama runtime"
grep -Fq 'qwen3.5:4b' "$probe" || fail "probe must pin the acceptance model"
grep -Fq 'properties: {}' "$probe" || fail "probe must exercise OpenClaw-normalized empty-object schema"
grep -Fq 'message.tool_calls' "$probe" || fail "probe must validate native Ollama tool calls"
grep -Fq 'ollama_openclaw_empty_schema_probe=PASS' "$probe" || fail "probe must emit a sanitized PASS marker"

echo "Ollama schema probe contract OK"
