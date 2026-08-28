#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "local AI benchmark failed: $*" >&2
  exit 1
}

for command_name in curl sha256sum awk grep wc mktemp date; do
  command -v "$command_name" >/dev/null || fail "$command_name is required"
done

base_url="${LOCAL_AI_BASE_URL:-}"
model="${LOCAL_AI_MODEL:-}"
[[ -n "$base_url" ]] || fail "LOCAL_AI_BASE_URL is required"
[[ -n "$model" ]] || fail "LOCAL_AI_MODEL is required"
[[ "$model" =~ ^[A-Za-z0-9._:/@+-]+$ ]] || fail "LOCAL_AI_MODEL contains unsupported characters"
[[ "$base_url" != *"@"* ]] || fail "LOCAL_AI_BASE_URL must not embed credentials"
[[ "$base_url" != *"?"* && "$base_url" != *"#"* ]] || fail "LOCAL_AI_BASE_URL must not contain query or fragment"

transport=""
case "$base_url" in
  https://*)
    transport="https"
    ;;
  http://127.0.0.1|http://127.0.0.1:*|http://127.0.0.1/*|http://127.0.0.1:*/*)
    transport="loopback_http"
    ;;
  http://\[::1\]|http://\[::1\]:*|http://\[::1\]/*|http://\[::1\]:*/*)
    transport="loopback_http"
    ;;
  *)
    fail "LOCAL_AI_BASE_URL must use HTTPS or IP-literal loopback HTTP"
    ;;
esac

workdir="$(mktemp -d /tmp/family-finance-local-ai-benchmark.XXXXXX)"
response_file="$workdir/response.json"
cleanup() {
  rm -rf "$workdir"
}
trap cleanup EXIT INT TERM

provider_root="${base_url%/}"
case "$provider_root" in
  */responses)
    endpoint="$provider_root"
    ;;
  */v1)
    endpoint="$provider_root/responses"
    ;;
  *)
    endpoint="$provider_root/v1/responses"
    ;;
esac
payload=$(printf '{"model":"%s","instructions":"Return the marker FAMILY_FINANCE_LOCAL_AI_OK in your response.","input":"Synthetic local AI benchmark. No financial data is included.","store":false,"parallel_tool_calls":false,"stream":false}' "$model")

elapsed_seconds="$(curl --silent --show-error --fail \
  --proto '=http,https' --proto-redir '=https' \
  --connect-timeout 10 --max-time 120 \
  --header 'Content-Type: application/json' \
  --data-binary "$payload" \
  --output "$response_file" \
  --write-out '%{time_total}' \
  "$endpoint")" || fail "provider request failed"

[[ -s "$response_file" ]] || fail "provider returned an empty response"
grep -Fq '"output"' "$response_file" || fail "provider response is missing output"
grep -Fq '"output_text"' "$response_file" || fail "provider response is missing output_text"
grep -Fq 'FAMILY_FINANCE_LOCAL_AI_OK' "$response_file" || fail "provider response did not satisfy the synthetic marker check"

response_sha256="$(sha256sum "$response_file" | awk '{print $1}')"
response_bytes="$(wc -c <"$response_file" | awk '{print $1}')"
elapsed_ms="$(awk -v seconds="$elapsed_seconds" 'BEGIN { printf "%d", (seconds * 1000) + 0.5 }')"
executed_at_utc="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

printf 'local_ai_benchmark=PASS executed_at_utc=%s transport=%s model=%s elapsed_ms=%s response_bytes=%s response_sha256=%s\n' \
  "$executed_at_utc" "$transport" "$model" "$elapsed_ms" "$response_bytes" "$response_sha256"
