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
app_secrets="$workdir/app-secrets"
mkdir -p "$repo/scripts" "$repo/bin" "$app_secrets"
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

for name in finance-auth-key finance-admin-password ezbookkeeping-api-token ezbookkeeping-secret-key finance-mcp-token; do
  printf '%s' "private-${name}-material-0123456789" >"$app_secrets/$name"
  chmod 0600 "$app_secrets/$name"
done

write_env() {
  cat >"$repo/.env" <<EOF_ENV
# Documentation comments may contain example.com and must not be treated as active placeholders.
# Example: RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-prod/
FINANCE_AUTH_KEY_HOST_FILE=$app_secrets/finance-auth-key
FINANCE_ADMIN_PASSWORD_HOST_FILE=$app_secrets/finance-admin-password
EBK_API_TOKEN_HOST_FILE=$app_secrets/ezbookkeeping-api-token
EBK_SECURITY_SECRET_KEY_HOST_FILE=$app_secrets/ezbookkeeping-secret-key
EBK_USER_ENABLE_REGISTER=false
RESTIC_REPOSITORY=${1:-}
RESTIC_PASSWORD_FILE=${2:-}
RESTIC_REST_USERNAME=${3:-}
RESTIC_REST_PASSWORD_FILE=${4:-}
BACKUP_KEEP_DAILY=${5:-}
MCP_ENABLED=${6:-false}
MCP_TOKEN_HOST_FILE=${7:-$app_secrets/finance-mcp-token}
MCP_TOKEN_FILE=/run/secrets/finance-mcp-token
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

grep -qiE 'permission|mode|group|world|other' "$workdir/env-public.out" || fail "insecure .env failure did not explain file permissions"

chmod 0600 "$repo/.env"
run_preflight >"$workdir/env-private.out" 2>&1 || fail "preflight rejected private .env because a commented example looked like an active placeholder"

# A placeholder near the start of a large active .env must never be hidden by
# grep -q closing a pipeline early under `set -o pipefail`.
write_env
printf '%s\n' 'ACTIVE_PLACEHOLDER=REPLACE_WITH_SECRET' >>"$repo/.env"
i=0
while [[ "$i" -lt 20000 ]]; do
  printf 'FILLER_%05d=value\n' "$i" >>"$repo/.env"
  i=$((i + 1))
done
chmod 0600 "$repo/.env"
if run_preflight >"$workdir/large-placeholder.out" 2>&1; then
  fail "preflight accepted an active placeholder in a large .env"
fi
grep -qi 'placeholder' "$workdir/large-placeholder.out" || fail "large-env placeholder rejection was unclear"

# Restore a valid environment for the remaining cases.
write_env
chmod 0600 "$repo/.env"

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

write_env "$valid_rest_repo" "$restic_password" family-finance-prod "$rest_server_password"
printf '%s\n' 'RESTIC_REST_PASSWORD=plaintext-must-not-live-in-env' >>"$repo/.env"
chmod 0600 "$repo/.env"
if run_preflight >"$workdir/native-rest-password.out" 2>&1; then
  fail "preflight accepted plaintext RESTIC_REST_PASSWORD in .env"
fi
grep -Fqi 'RESTIC_REST_PASSWORD' "$workdir/native-rest-password.out" || fail "plaintext RESTIC_REST_PASSWORD rejection was unclear"

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
grep -qiE 'permission|mode|group|world|other' "$workdir/restic-public.out" || fail "insecure restic password failure did not explain file permissions"
chmod 0600 "$restic_password"

chmod 0644 "$rest_server_password"
write_env "$valid_rest_repo" "$restic_password" family-finance-prod "$rest_server_password"
if run_preflight >"$workdir/rest-server-public.out" 2>&1; then
  fail "preflight accepted RESTIC_REST_PASSWORD_FILE with group/world permissions"
fi
grep -qiE 'permission|mode|group|world|other' "$workdir/rest-server-public.out" || fail "insecure REST producer password failure did not explain secret permissions"
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

mcp_token="$app_secrets/finance-mcp-token"
chmod 0644 "$mcp_token"
write_env '' '' '' '' '' true "$mcp_token"
chmod 0600 "$repo/.env"
if run_preflight >"$workdir/mcp-public.out" 2>&1; then
  fail "preflight accepted MCP bearer file with group/world permissions"
fi

grep -qiE 'MCP|permission|mode|group|world|other' "$workdir/mcp-public.out" || fail "insecure MCP bearer failure did not explain secret permissions"

chmod 0600 "$mcp_token"
run_preflight >"$workdir/mcp-private.out" 2>&1 || fail "preflight rejected private MCP bearer file"

echo "Preflight secret permission contract OK"
