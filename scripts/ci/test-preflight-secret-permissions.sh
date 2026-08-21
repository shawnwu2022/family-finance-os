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
FINANCE_AUTH_USER=acceptance
FINANCE_AUTH_HASH='hash'
RESTIC_REPOSITORY=${1:-}
RESTIC_PASSWORD_FILE=${2:-}
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
run_preflight >"$workdir/env-private.out" 2>&1 || fail "preflight rejected private .env"

restic_password="$workdir/restic-password"
printf '%s' 'high-entropy-restic-password-material' >"$restic_password"
chmod 0644 "$restic_password"
write_env 'sftp:backup-host:/srv/restic/family-finance-os' "$restic_password"
chmod 0600 "$repo/.env"
if run_preflight >"$workdir/restic-public.out" 2>&1; then
  fail "preflight accepted RESTIC_PASSWORD_FILE with group/world permissions"
fi

grep -qiE 'permission|mode|group|world' "$workdir/restic-public.out" || fail "insecure restic password failure did not explain file permissions"

chmod 0600 "$restic_password"
run_preflight >"$workdir/restic-private.out" 2>&1 || fail "preflight rejected private RESTIC_PASSWORD_FILE"

echo "Preflight secret permission contract OK"
