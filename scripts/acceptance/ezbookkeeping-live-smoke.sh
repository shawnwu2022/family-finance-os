#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "ezBookkeeping live smoke failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

ENV_FILE="${FINANCE_ENV_FILE:-$ROOT_DIR/.env}"
[[ -f "$ENV_FILE" ]] || fail "environment file not found: $ENV_FILE"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

command -v curl >/dev/null || fail "curl is required"
command -v sha256sum >/dev/null || fail "sha256sum is required"

: "${EBK_API_TOKEN:?EBK_API_TOKEN is required}"
: "${APP_TIMEZONE:?APP_TIMEZONE is required}"

if [[ -n "${EZBOOKKEEPING_SMOKE_BASE_URL:-}" ]]; then
  base_url="${EZBOOKKEEPING_SMOKE_BASE_URL%/}"
else
  : "${EBK_DOMAIN:?EBK_DOMAIN or EZBOOKKEEPING_SMOKE_BASE_URL is required}"
  base_url="https://${EBK_DOMAIN%/}/api/v1"
fi
[[ "$base_url" == https://* ]] || fail "live acceptance smoke requires HTTPS"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

curl_config() {
  printf '%s\n' \
    'silent' \
    'show-error' \
    'fail' \
    'connect-timeout = 10' \
    'max-time = 30' \
    'proto = "=https"' \
    'tlsv1.2' \
    "header = \"Authorization: Bearer ${EBK_API_TOKEN}\"" \
    "header = \"X-Timezone-Name: ${APP_TIMEZONE}\"" \
    'header = "Accept: application/json"'
}

probe() {
  local name="$1"
  local endpoint="$2"
  local output="$workdir/${name}.json"

  curl_config | curl --config - --output "$output" "$base_url/$endpoint"
  grep -Eq '"success"[[:space:]]*:[[:space:]]*true' "$output" || fail "$name response did not contain success=true"

  local digest
  digest="$(sha256sum "$output" | awk '{print $1}')"
  printf '%s_sha256=%s\n' "$name" "$digest"
}

probe accounts 'accounts/list.json'
probe categories 'transaction/categories/list.json'

transactions_output="$workdir/transactions.json"
curl_config | curl --config - --output "$transactions_output" \
  "$base_url/transactions/list.json?count=1&max_time=0&trim_account=false&trim_category=true&trim_tag=true&with_pictures=false"
grep -Eq '"success"[[:space:]]*:[[:space:]]*true' "$transactions_output" || fail "transactions response did not contain success=true"
transactions_digest="$(sha256sum "$transactions_output" | awk '{print $1}')"
transactions_total="$(grep -Eo '"totalCount"[[:space:]]*:[[:space:]]*[0-9]+' "$transactions_output" | head -n1 | sed -E 's/.*:[[:space:]]*//' || true)"
[[ "$transactions_total" =~ ^[0-9]+$ ]] || fail "transactions response did not expose totalCount"
if [[ "${EZBOOKKEEPING_SMOKE_REQUIRE_TRANSACTIONS:-1}" == "1" && "$transactions_total" -le 0 ]]; then
  fail "transaction ledger is empty; import real statement data before production acceptance"
fi
printf 'transactions_sha256=%s\n' "$transactions_digest"
printf 'transactions_total=%s\n' "$transactions_total"
printf 'ezbookkeeping_live_smoke=PASS\n'
