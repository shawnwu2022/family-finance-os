#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "application auth documentation contract failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

security_doc="docs/06-security-privacy.md"
operations_doc="docs/07-operations.md"
for path in "$security_doc" "$operations_doc"; do
  [[ -f "$path" ]] || fail "missing documentation: $path"
done

for legacy in FINANCE_AUTH_USER FINANCE_AUTH_HASH 'Caddy Basic Auth' 'Finance Core 的公网 Basic Auth'; do
  if grep -Fq "$legacy" "$security_doc" "$operations_doc"; then
    fail "legacy Finance Basic Auth guidance remains: $legacy"
  fi
done

for phrase in \
  'Finance Core 负责浏览器用户认证与 household authorization' \
  '密码 + TOTP' \
  '__Host-finance_session' \
  'Caddy 只负责 TLS 与反向代理'; do
  grep -Fq "$phrase" "$security_doc" || fail "security guide missing application-native auth statement: $phrase"
done

for phrase in \
  'FINANCE_AUTH_KEY_HOST_FILE' \
  'FINANCE_ADMIN_PASSWORD_HOST_FILE' \
  'EBK_API_TOKEN_HOST_FILE' \
  'EBK_SECURITY_SECRET_KEY_HOST_FILE' \
  'FINANCE_ADMIN_USERNAME' \
  '首次登录' \
  'TOTP' \
  'recovery codes' \
  'FINANCE_TRUSTED_PROXY_CIDR'; do
  grep -Fq "$phrase" "$operations_doc" || fail "operations guide missing current auth/secret step: $phrase"
done

if grep -Eq '^[[:space:]]*[0-9]+\..*EBK_API_TOKEN.*\.env' "$operations_doc"; then
  fail "operations guide must not put the ezBookkeeping API token in .env"
fi

echo "Application auth documentation contract OK"
