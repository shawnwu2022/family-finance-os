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
readme="README.md"
acceptance_doc="docs/08-testing-acceptance.md"
evidence_doc="docs/acceptance/v1-production-evidence.md"
for path in "$security_doc" "$operations_doc" "$readme" "$acceptance_doc" "$evidence_doc"; do
  [[ -f "$path" ]] || fail "missing documentation: $path"
done

for legacy in FINANCE_AUTH_USER FINANCE_AUTH_HASH 'Finance Core 的公网 Basic Auth'; do
  if grep -Fq "$legacy" "$security_doc" "$operations_doc" "$readme" "$acceptance_doc"; then
    fail "legacy Finance Basic Auth guidance remains: $legacy"
  fi
done

if grep -Fq 'Finance Basic Auth' "$readme" || grep -Fq 'Finance 域先经过 Caddy Basic Auth' "$acceptance_doc"; then
  fail "public docs still describe Caddy as the Finance identity provider"
fi
if grep -Fq 'Finance Caddy Basic Auth checked on production edge' "$evidence_doc"; then
  fail "production evidence still asks for the removed Caddy Finance authentication gate"
fi

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

for phrase in \
  'application-native authentication' \
  'make verify-auth-security' \
  'Caddy 只负责 TLS、反向代理'; do
  grep -Fq "$phrase" "$readme" || fail "README missing current Finance auth statement: $phrase"
done

for phrase in \
  'make verify-auth-security' \
  'Finance 域显示应用自己的登录页' \
  'ezBookkeeping owner 2FA enrollment 必须人工确认'; do
  grep -Fq "$phrase" "$acceptance_doc" || fail "acceptance guide missing current auth gate: $phrase"
done

for phrase in \
  'Finance app-native password + mandatory TOTP login verified on production edge' \
  'ezBookkeeping owner 2FA enrollment confirmed' \
  'Production release remains **BLOCKED**'; do
  grep -Fq "$phrase" "$evidence_doc" || fail "production evidence missing current auth/manual-evidence boundary: $phrase"
done

if grep -Eq '^[[:space:]]*[0-9]+\..*EBK_API_TOKEN.*\.env' "$operations_doc"; then
  fail "operations guide must not put the ezBookkeeping API token in .env"
fi

echo "Application auth documentation contract OK"
