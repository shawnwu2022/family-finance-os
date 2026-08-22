#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "preflight secret-permission test failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
repo="$workdir/repo"
mkdir -p "$repo/scripts" "$repo/bin"
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
cat >"$repo/bin/restic" <<'EOF_RESTIC'
#!/usr/bin/env bash
exit 0
EOF_RESTIC
chmod 0755 "$repo/bin/docker" "$repo/bin/openssl" "$repo/bin/restic"

write_env() {
  cat >"$repo/.env" <<EOF_ENV
# Documentation comments may contain example.com and must not be treated as active placeholders.
# Example: RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-prod/
FINANCE_AUTH_USER=acceptance
FINANCE_AUTH_HASH='hash'
RESTIC_REPOSITORY=${1:-}
RESTIC_PASSWORD_FILE=${2:-}
RESTIC_REST_USERNAME=${3:-}
RESTIC_REST_PASSWORD_FILE=${4:-}
BACKUP_KEEP_DAILY=${5:-}
MCP_ENABLED=${6:-false}
MCP_TOKEN_FILE=${7:-/run/secrets/finance-mcp-token}
EOF_ENV
}

run_preflight() {
  (cd "$repo" && PATH="$repo/bin:$PATH" bash scripts/preflight.sh)
}

write_env
chmod 0644 "$repo/.env"
if run_preflight >"$workdir/env-public.out" 2>&1; then
  fail "preflight accepted .env with group/world permissions"
fi

grep -qiE 'permission|mode|group|world' "$workdir/env-public.out" || fail "insecure .env failure did not explain file permissions"

chmod 0600 "$repo/.env"
run_preflight >"$workdir/env-private.out" 2>&1 || fail "preflight rejected private .env because a commented example looked like an active placeholder"

# Effective readability differs for privileged UID 0. Keep this contract UID-independent:
# production preflight must explicitly check readability, while mode behavior is exercised below.
grep -Fq '[[ -r "$path" ]]' "$repo/scripts/preflight.sh" || fail "preflight must explicitly require secret readability"

restic_password="$workdir/restic-password"
rest_server_password="$workdir/rest-server-password"
printf '%s' 'high-entropy-restic-password-material' >"$restic_password"
printf '%s' 'high-entropy-rest-server-password-material' >"$rest_server_password"
chmod 0600 "$restic_password" "$rest_server_password"

valid_rest_repo='rest:https://backup.test.invalid/family-finance-prod/'
write_env "$valid_rest_repo" "$restic_password" family-finance-prod "$rest_server_password"
chmod 0600 "$repo/.env"
if ! run_preflight >"$workdir/rest-producer-valid.out" 2>&1; then
  cat "$workdir/rest-producer-valid.out" >&2
  fail "preflight rejected valid HTTPS REST producer configuration"
fi

write_env 'rest:https://backup.test.invalid/family-finance-typo/' "$restic_password" family-finance-prod "$rest_server_password"
if run_preflight >"$workdir/private-repo-mismatch.out" 2>&1; then
  fail "preflight accepted a private repository path that does not match RESTIC_REST_USERNAME"
fi
grep -qiE 'private|repository|username|RESTIC_REST_USERNAME' "$workdir/private-repo-mismatch.out" || fail "private repository username/path mismatch failure was unclear"

chmod 0644 "$restic_password"
write_env "$valid_rest_repo" "$restic_password" family-finance-prod "$rest_server_password"
if run_preflight >"$workdir/restic-public.out" 2>&1; then
  fail "preflight accepted RESTIC_PASSWORD_FILE with group/world permissions"
fi
grep -qiE 'permission|mode|group|world' "$workdir/restic-public.out" || fail "insecure restic password failure did not explain file permissions"
chmod 0600 "$restic_password"

chmod 0644 "$rest_server_password"
write_env "$valid_rest_repo" "$restic_password" family-finance-prod "$rest_server_password"
if run_preflight >"$workdir/rest-server-public.out" 2>&1; then
  fail "preflight accepted RESTIC_REST_PASSWORD_FILE with group/world permissions"
fi
grep -qiE 'permission|mode|group|world' "$workdir/rest-server-public.out" || fail "insecure REST producer password failure did not explain secret permissions"
chmod 0600 "$rest_server_password"

repo_rest_password="$repo/rest-server-password"
printf '%s' 'repo-local-rest-server-password' >"$repo_rest_password"
chmod 0600 "$repo_rest_password"
write_env "$valid_rest_repo" "$restic_password" family-finance-prod "$repo_rest_password"
if run_preflight >"$workdir/rest-server-repo-local.out" 2>&1; then
  fail "preflight accepted repository-local RESTIC_REST_PASSWORD_FILE"
fi
grep -qiE 'outside|repository' "$workdir/rest-server-repo-local.out" || fail "repository-local REST producer password failure was unclear"

external_symlink="$workdir/rest-server-password-link"
ln -s "$repo_rest_password" "$external_symlink"
write_env "$valid_rest_repo" "$restic_password" family-finance-prod "$external_symlink"
if run_preflight >"$workdir/rest-server-symlink.out" 2>&1; then
  fail "preflight accepted an external symlink resolving to a repository-local REST password"
fi
grep -qiE 'outside|repository|symlink' "$workdir/rest-server-symlink.out" || fail "repository-local REST password symlink failure was unclear"

write_env "$valid_rest_repo" "$restic_password" '' "$rest_server_password"
if run_preflight >"$workdir/rest-username-missing.out" 2>&1; then
  fail "preflight accepted missing RESTIC_REST_USERNAME"
fi
grep -Fqi 'RESTIC_REST_USERNAME' "$workdir/rest-username-missing.out" || fail "missing REST producer username failure was unclear"

write_env "$valid_rest_repo" "$restic_password" family-finance-prod ''
if run_preflight >"$workdir/rest-password-file-missing.out" 2>&1; then
  fail "preflight accepted missing RESTIC_REST_PASSWORD_FILE"
fi
grep -Fqi 'RESTIC_REST_PASSWORD_FILE' "$workdir/rest-password-file-missing.out" || fail "missing REST producer password-file failure was unclear"

write_env 'rest:https://family-finance-prod:plaintext-password@backup.test.invalid/family-finance-prod/' "$restic_password" family-finance-prod "$rest_server_password"
if run_preflight >"$workdir/rest-url-userinfo.out" 2>&1; then
  fail "preflight accepted credentials embedded in RESTIC_REPOSITORY"
fi
grep -qiE 'credential|userinfo|URL' "$workdir/rest-url-userinfo.out" || fail "embedded REST URL credential rejection was unclear"

write_env 'sftp:backup-host:/srv/restic/family-finance-os' "$restic_password" family-finance-prod "$rest_server_password"
if run_preflight >"$workdir/sftp-legacy.out" 2>&1; then
  fail "preflight accepted legacy SFTP producer repository"
fi
grep -qiE 'rest:https|HTTPS REST|production off-site' "$workdir/sftp-legacy.out" || fail "legacy SFTP rejection did not explain the HTTPS REST requirement"

write_env 'rest:http://backup.test.invalid/family-finance-prod/' "$restic_password" family-finance-prod "$rest_server_password"
if run_preflight >"$workdir/rest-http.out" 2>&1; then
  fail "preflight accepted plaintext REST producer repository"
fi
grep -qiE 'rest:https|HTTPS REST|production off-site' "$workdir/rest-http.out" || fail "plaintext REST rejection did not explain the HTTPS requirement"

write_env "$valid_rest_repo" "$restic_password" family-finance-prod "$rest_server_password" 14
if run_preflight >"$workdir/legacy-retention.out" 2>&1; then
  fail "preflight accepted legacy producer-side BACKUP_KEEP_DAILY"
fi
grep -Fqi 'BACKUP_KEEP_DAILY' "$workdir/legacy-retention.out" || fail "legacy producer retention rejection was unclear"

mkdir -p "$repo/secrets"
mcp_token="$repo/secrets/finance-mcp-token"
printf '%s' '0123456789abcdef0123456789abcdef' >"$mcp_token"
chmod 0644 "$mcp_token"
write_env '' '' '' '' '' true /run/secrets/finance-mcp-token
chmod 0600 "$repo/.env"
if run_preflight >"$workdir/mcp-public.out" 2>&1; then
  fail "preflight accepted MCP bearer file with group/world permissions"
fi

grep -qiE 'MCP|permission|mode|group|world' "$workdir/mcp-public.out" || fail "insecure MCP bearer failure did not explain secret permissions"

chmod 0600 "$mcp_token"
run_preflight >"$workdir/mcp-private.out" 2>&1 || fail "preflight rejected private MCP bearer file"

echo "Preflight secret permission contract OK"
