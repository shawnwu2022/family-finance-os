#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "application preflight security test failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
repo="$workdir/repo"
external="$workdir/external-secrets"
mkdir -p "$repo/scripts" "$repo/bin" "$external"
cp "$ROOT_DIR/scripts/preflight.sh" "$repo/scripts/preflight.sh"

cat >"$repo/bin/docker" <<'EOF_DOCKER'
#!/usr/bin/env bash
set -euo pipefail
case "${1:-} ${2:-}" in
  "compose version"|"compose config") exit 0 ;;
  *) exit 0 ;;
esac
EOF_DOCKER
cat >"$repo/bin/openssl" <<'EOF_OPENSSL'
#!/usr/bin/env bash
exit 0
EOF_OPENSSL
chmod 0755 "$repo/bin/docker" "$repo/bin/openssl"

for name in finance-auth-key finance-admin-password ezbookkeeping-api-token ezbookkeeping-secret-key finance-mcp-token; do
  printf '%s' "test-secret-material-${name}-0123456789" >"$external/$name"
  chmod 0600 "$external/$name"
done

write_env() {
  local registration="${1:-false}"
  local mcp_enabled="${2:-false}"
  local auth_key="${3:-$external/finance-auth-key}"
  local admin_password="${4:-$external/finance-admin-password}"
  local ledger_token="${5:-$external/ezbookkeeping-api-token}"
  local ledger_secret="${6:-$external/ezbookkeeping-secret-key}"
  local mcp_token="${7:-$external/finance-mcp-token}"
  cat >"$repo/.env" <<EOF_ENV
EBK_DOMAIN=book.test.invalid
FINANCE_DOMAIN=finance.test.invalid
POSTGRES_USER=postgres
POSTGRES_PASSWORD=test-postgres-password
FINANCE_DB_PASSWORD=test-finance-password
EBK_DB_PASSWORD=test-ebk-password
FINANCE_AUTH_KEY_HOST_FILE=$auth_key
FINANCE_ADMIN_PASSWORD_HOST_FILE=$admin_password
EBK_API_TOKEN_HOST_FILE=$ledger_token
EBK_SECURITY_SECRET_KEY_HOST_FILE=$ledger_secret
EBK_USER_ENABLE_REGISTER=$registration
MCP_ENABLED=$mcp_enabled
MCP_TOKEN_HOST_FILE=$mcp_token
EOF_ENV
  chmod 0600 "$repo/.env"
}

run_preflight() {
  (cd "$repo" && PATH="$repo/bin:$PATH" bash scripts/preflight.sh)
}

# The application-native auth deployment must not require removed Caddy credentials.
write_env false false
if ! run_preflight >"$workdir/valid.out" 2>&1; then
  cat "$workdir/valid.out" >&2
  fail "preflight rejected the valid application-native auth environment"
fi

# Legacy edge credentials are forbidden after the cutover.
printf '%s\n' 'FINANCE_AUTH_USER=legacy' 'FINANCE_AUTH_HASH=legacy-hash' >>"$repo/.env"
if run_preflight >"$workdir/legacy-caddy.out" 2>&1; then
  fail "preflight accepted removed Caddy Finance credentials"
fi
grep -qiE 'FINANCE_AUTH_(USER|HASH)|legacy|Caddy' "$workdir/legacy-caddy.out" || fail "legacy Caddy credential rejection was unclear"

# Required application secrets must be private host files outside the repository.
write_env false false
chmod 0644 "$external/finance-auth-key"
if run_preflight >"$workdir/public-auth-key.out" 2>&1; then
  fail "preflight accepted a group/world-readable Finance auth key"
fi
grep -qiE 'finance|auth|permission|mode|group|other' "$workdir/public-auth-key.out" || fail "insecure Finance auth key rejection was unclear"
chmod 0600 "$external/finance-auth-key"

repo_token="$repo/ezbookkeeping-api-token"
printf '%s' 'repo-local-ledger-token' >"$repo_token"
chmod 0600 "$repo_token"
write_env false false "$external/finance-auth-key" "$external/finance-admin-password" "$repo_token"
if run_preflight >"$workdir/repo-local-ledger-token.out" 2>&1; then
  fail "preflight accepted a repository-local ezBookkeeping API token"
fi
grep -qiE 'outside|repository|ezBookkeeping|token' "$workdir/repo-local-ledger-token.out" || fail "repository-local ledger token rejection was unclear"

# Steady-state production must not leave ezBookkeeping registration open.
write_env true false
if run_preflight >"$workdir/register-open.out" 2>&1; then
  fail "preflight accepted ezBookkeeping registration enabled"
fi
grep -qiE 'register|registration|EBK_USER_ENABLE_REGISTER' "$workdir/register-open.out" || fail "open registration rejection was unclear"

# MCP uses the same external-private-file boundary when enabled.
write_env false true
run_preflight >"$workdir/mcp-valid.out" 2>&1 || fail "preflight rejected a valid external MCP bearer file"
chmod 0644 "$external/finance-mcp-token"
if run_preflight >"$workdir/mcp-public.out" 2>&1; then
  fail "preflight accepted a group/world-readable MCP bearer file"
fi
grep -qiE 'MCP|permission|mode|group|other' "$workdir/mcp-public.out" || fail "insecure MCP bearer rejection was unclear"

echo "Application preflight security contract OK"
