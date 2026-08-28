#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

fail() {
  echo "application auth edge contract failed: $*" >&2
  exit 1
}

if grep -Eq '(^|[[:space:]])basic_auth([[:space:]]|$)' Caddyfile Caddyfile.acceptance; then
  fail "Caddy must not perform Finance user authentication"
fi
for file in Caddyfile Caddyfile.acceptance compose.yaml .env.example; do
  if grep -Eq 'FINANCE_AUTH_(USER|HASH)' "$file"; then
    fail "$file must not contain legacy Caddy Finance credentials"
  fi
done
grep -Fq 'reverse_proxy finance-core:8000' Caddyfile || fail "Finance must still reverse_proxy to finance-core"

grep -Fq 'subnet: 172.30.0.0/24' compose.yaml || fail "app network must use deterministic 172.30.0.0/24 subnet"
for spec in \
  'caddy:172.30.0.10' \
  'ezbookkeeping:172.30.0.20' \
  'finance-core:172.30.0.30' \
  'postgres:172.30.0.40'; do
  service="${spec%%:*}"
  address="${spec#*:}"
  block="$(awk -v service="$service" '
    $0 == "  " service ":" { in_service=1 }
    in_service && /^  [A-Za-z0-9_-]+:/ && $0 != "  " service ":" { exit }
    in_service { print }
  ' compose.yaml)"
  grep -Fq "ipv4_address: ${address}" <<<"$block" || fail "$service must use ${address}"
done

for pair in \
  'EBK_AUTH_ENABLE_TWO_FACTOR: "true"' \
  'EBK_USER_ENABLE_REGISTER: ${EBK_USER_ENABLE_REGISTER:-false}' \
  'EBK_SECURITY_MAX_FAILURES_PER_IP_PER_MINUTE: "5"' \
  'EBK_SECURITY_MAX_FAILURES_PER_USER_PER_MINUTE: "5"' \
  'EBK_SECURITY_TOKEN_EXPIRED_TIME: "43200"' \
  'EBK_SECURITY_TOKEN_MIN_REFRESH_INTERVAL: "1800"' \
  'EBK_SECURITY_ENABLE_API_TOKEN: "true"' \
  'EBK_SECURITY_API_TOKEN_ALLOWED_REMOTE_IPS: 172.30.0.30' \
  'EBK_SECURITY_TRUSTED_PROXY_IPS: 172.30.0.10/32'; do
  grep -Fq "$pair" compose.yaml || fail "missing hardened ezBookkeeping setting: $pair"
done

grep -Fq 'FINANCE_TRUSTED_PROXY_CIDR: 172.30.0.10/32' compose.yaml \
  || fail "Finance Core must trust only the deterministic Caddy address for forwarded login source IPs"
if grep -Eq 'FINANCE_TRUSTED_PROXY_CIDR:[[:space:]]*\$\{' compose.yaml; then
  fail "reference Compose must not allow .env to widen the Finance trusted-proxy boundary"
fi

if grep -Eq '^[[:space:]]*EBK_SECURITY_SECRET_KEY:' compose.yaml || grep -Eq '^EBK_SECURITY_SECRET_KEY=' .env.example; then
  fail "ezBookkeeping secret key must not be supplied as a plaintext environment value"
fi
if grep -Eq '^[[:space:]]*EBK_API_TOKEN:' compose.yaml || grep -Eq '^EBK_API_TOKEN=' .env.example; then
  fail "ezBookkeeping API token must not be supplied as a plaintext environment value"
fi
for path in \
  '/run/secrets/finance-auth-key' \
  '/run/secrets/finance-admin-password' \
  '/run/secrets/ezbookkeeping-api-token' \
  '/run/secrets/ezbookkeeping-secret-key'; do
  grep -Fq "$path" compose.yaml || fail "Compose must mount/reference $path"
done
grep -Fq 'EBK_API_TOKEN_FILE:' compose.yaml || fail "Finance Core must receive only an ezBookkeeping API token file path"
grep -Fq 'EBK_SECURITY_SECRET_KEY_FILE:' compose.yaml || fail "ezBookkeeping wrapper must receive only a secret-key file path"
grep -Fq 'EBK_SECURITY_SECRET_KEY_FILE' infra/ezbookkeeping/docker-entrypoint.sh || fail "ezBookkeeping secret-file wrapper is missing"
grep -Fq -- '--conf-path' infra/ezbookkeeping/docker-entrypoint.sh || fail "ezBookkeeping wrapper must use the pinned upstream --conf-path flag"
if grep -Eq -- '(^|[[:space:]])--config([[:space:]]|=)' infra/ezbookkeeping/docker-entrypoint.sh; then
  fail "ezBookkeeping wrapper must not invent unsupported --config flag"
fi

for host_key in FINANCE_AUTH_KEY_HOST_FILE FINANCE_ADMIN_PASSWORD_HOST_FILE EBK_API_TOKEN_HOST_FILE EBK_SECURITY_SECRET_KEY_HOST_FILE; do
  grep -Fq "${host_key}=" .env.example || fail ".env.example must document ${host_key}"
done

echo "Application-native auth edge contract OK"
