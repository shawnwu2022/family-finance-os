#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

fail() {
  echo "local AI contract failed: $*" >&2
  exit 1
}

benchmark="scripts/acceptance/local-ai-benchmark.sh"
env_example=".env.example"
config_go="internal/config/config.go"
application_go="cmd/finance-core/application.go"
provider_go="internal/llm/openai_compatible.go"

for path in "$benchmark" "$env_example" "$config_go" "$application_go" "$provider_go"; do
  [[ -f "$path" ]] || fail "required file is missing: $path"
done
bash -n "$benchmark" || fail "benchmark shell syntax is invalid"

for needle in LOCAL_AI_BASE_URL LOCAL_AI_MODEL FAMILY_FINANCE_LOCAL_AI_OK sha256sum '%{time_total}' response_sha256= mktemp 'rm -rf'; do
  grep -Fq "$needle" "$benchmark" || fail "benchmark contract is missing $needle"
done
grep -Fq 'https://' "$benchmark" || fail "benchmark must allow HTTPS local providers"
grep -Fq '127.0.0.1' "$benchmark" || fail "benchmark must allow IPv4 loopback HTTP"
grep -Fq '::1' "$benchmark" || fail "benchmark must allow IPv6 loopback HTTP"
if grep -Eq '(Authorization:[[:space:]]*Bearer|LLM_API_KEY|LOCAL_AI_API_KEY)' "$benchmark"; then
  fail "benchmark must remain credential-free"
fi

for mode in disabled local external; do
  grep -Fq "LLMMode${mode^}" "$config_go" || fail "config is missing LLM mode $mode"
done
grep -Fq 'LLM_MODE' "$env_example" || fail ".env.example must document LLM_MODE"
grep -Fq 'LLM_MODE=external' "$env_example" || fail ".env.example must provide the default external example"
grep -Fq 'LLM_MODE=local' "$env_example" || fail ".env.example must document a local-AI example"
grep -Fq 'https://p40-ai.example/v1' "$env_example" || fail ".env.example must show a TLS LAN/P40 local-AI endpoint"

grep -Fq 'LLM_MODE=local requires' "$application_go" || fail "runtime must validate local mode"
grep -Fq 'LLM_MODE=external requires' "$application_go" || fail "runtime must validate external mode"
grep -Fq 'plaintext HTTP LLM endpoints are only allowed for IP-literal loopback addresses' "$provider_go" \
  || fail "provider must preserve the plaintext loopback-only boundary"
grep -Fq 'redirected to insecure HTTP endpoint' "$provider_go" \
  || fail "provider must preserve HTTPS downgrade rejection"

echo "Local AI contract OK"
