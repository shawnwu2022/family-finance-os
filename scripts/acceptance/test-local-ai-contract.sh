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
grep -Fq 'func allowedProviderEndpoint' "$provider_go" || fail "provider endpoint policy is missing"
grep -Fq 'func allowedProviderRedirect' "$provider_go" || fail "provider redirect policy is missing"
grep -Fq 'func isLoopbackHTTPProviderURL' "$provider_go" || fail "provider loopback HTTP policy is missing"
grep -Fq 'strings.EqualFold(endpoint.Scheme, "https")' "$provider_go" || fail "provider must allow secure HTTPS endpoints"
grep -Fq 'ip.IsLoopback()' "$provider_go" || fail "provider must restrict plaintext HTTP to loopback IPs"
grep -Fq 'return isLoopbackHTTPProviderURL(origin) && isLoopbackHTTPProviderURL(target)' "$provider_go" \
  || fail "provider redirects must not downgrade a secure origin to plaintext HTTP"

echo "Local AI contract OK"
