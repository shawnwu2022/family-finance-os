#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "OpenClaw secret-file boundary contract failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

provisioner="scripts/acceptance/openclaw-ephemeral-release-acceptance.sh"
[[ -f "$provisioner" ]] || fail "provisioner is missing"
bash -n "$provisioner" || fail "provisioner shell syntax is invalid"

for legacy in FINANCE_AUTH_USER FINANCE_AUTH_HASH EBK_SECURITY_SECRET_KEY= EBK_API_TOKEN=; do
  if grep -Fq "$legacy" "$provisioner"; then
    fail "provisioner still contains legacy plaintext/edge credential contract: $legacy"
  fi
done

for host_var in \
  FINANCE_AUTH_KEY_HOST_FILE \
  FINANCE_ADMIN_PASSWORD_HOST_FILE \
  EBK_API_TOKEN_HOST_FILE \
  EBK_SECURITY_SECRET_KEY_HOST_FILE; do
  grep -Fq "$host_var" "$provisioner" || fail "provisioner must supply $host_var"
done

grep -Fq 'finance-auth-key' "$provisioner" || fail "Finance auth key secret fixture is missing"
grep -Fq 'finance-admin-password' "$provisioner" || fail "Finance admin password secret fixture is missing"
grep -Fq 'ezbookkeeping-api-token' "$provisioner" || fail "ezBookkeeping API token secret fixture is missing"
grep -Fq 'ezbookkeeping-secret-key' "$provisioner" || fail "ezBookkeeping signing secret fixture is missing"
grep -Fq 'chmod 0600' "$provisioner" || fail "secret fixtures must use private file permissions"
grep -Fq 'chown 65532:65532' "$provisioner" || fail "Finance secret fixtures must be readable by the nonroot runtime UID"
grep -Fq 'chown 1000:1000' "$provisioner" || fail "ezBookkeeping signing secret must be readable by its runtime UID"
grep -Fq 'printf '\''%s'\'' "$ebk_api_token" >"$ebk_api_token_host_file"' "$provisioner" \
  || fail "generated ezBookkeeping API token must be written to the mounted token file before Finance Core starts"

finance_start_line="$(grep -nF 'up -d --build --wait finance-core' "$provisioner" | head -n 1 | cut -d: -f1)"
token_write_line="$(grep -nF 'printf '\''%s'\'' "$ebk_api_token" >"$ebk_api_token_host_file"' "$provisioner" | head -n 1 | cut -d: -f1)"
[[ "$finance_start_line" =~ ^[0-9]+$ && "$token_write_line" =~ ^[0-9]+$ && "$token_write_line" -lt "$finance_start_line" ]] \
  || fail "ezBookkeeping API token file must be populated before Finance Core startup"

if grep -Fq -- '--user "acceptance:${finance_auth_password}"' "$provisioner"; then
  fail "Finance health check must not depend on removed Caddy Basic Auth"
fi

echo "OpenClaw secret-file boundary contract OK"
