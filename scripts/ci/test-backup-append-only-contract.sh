#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "append-only backup contract failed: $*" >&2
  exit 1
}

backup="scripts/backup.sh"
preflight="scripts/preflight.sh"
maintenance="scripts/backup-maintenance.sh"
env_example=".env.example"

for file in "$backup" "$preflight" "$env_example"; do
  [[ -f "$file" ]] || fail "required file is missing: $file"
done

if grep -Eq 'restic[[:space:]]+forget|restic[[:space:]]+prune|restic[[:space:]]+check' "$backup"; then
  fail "production backup must not have destructive/full-maintenance restic authority"
fi

grep -Fq 'rest:https://' "$preflight" || fail "preflight must require HTTPS REST backup transport"
grep -Fq 'RESTIC_REST_USERNAME' "$preflight" || fail "preflight must require the REST producer username"
grep -Fq 'RESTIC_REST_PASSWORD_FILE' "$preflight" || fail "preflight must require an external REST password file"
grep -Fq 'RESTIC_REST_USERNAME' "$env_example" || fail ".env.example must document the REST producer username"
grep -Fq 'RESTIC_REST_PASSWORD_FILE' "$env_example" || fail ".env.example must document the REST producer password file"

for legacy in BACKUP_KEEP_DAILY BACKUP_KEEP_WEEKLY BACKUP_KEEP_MONTHLY; do
  if grep -Eq "^${legacy}=" "$env_example"; then
    fail ".env.example must not advertise producer-side count retention: $legacy"
  fi
  grep -Fq "$legacy" "$preflight" || fail "preflight must explicitly reject legacy producer retention variable $legacy"
done

[[ -f "$maintenance" ]] || fail "backup-maintenance.sh must exist"
bash -n "$maintenance" || fail "backup-maintenance.sh syntax is invalid"
grep -Fq 'RESTIC_MAINTENANCE_REPOSITORY' "$maintenance" || fail "maintenance must use a dedicated local repository variable"
grep -Fq -- '--keep-within' "$maintenance" || fail "maintenance retention must use --keep-within"
grep -Fq -- '--prune' "$maintenance" || fail "maintenance host must own prune"
grep -Eq 'restic[[:space:]]+check' "$maintenance" || fail "maintenance host must own restic check"

for forbidden in --keep-daily --keep-weekly --keep-monthly --keep-last; do
  if grep -Fq -- "$forbidden" "$maintenance"; then
    fail "count-based append-only retention is forbidden: $forbidden"
  fi
done

if grep -Fq 'RESTIC_REST_PASSWORD_FILE' "$maintenance" || grep -Fq 'RESTIC_REST_USERNAME' "$maintenance"; then
  fail "maintenance must not consume the producer REST credential"
fi

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
mkdir -p "$workdir/bin" "$workdir/repository"
password_file="$workdir/restic-password"
printf '%s' 'high-entropy-maintenance-password-material' >"$password_file"
chmod 0600 "$password_file"
restic_log="$workdir/restic.log"

cat >"$workdir/bin/restic" <<'EOF_RESTIC'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"$RESTIC_TEST_LOG"
EOF_RESTIC
chmod 0755 "$workdir/bin/restic"

if PATH="$workdir/bin:$PATH" \
  RESTIC_TEST_LOG="$restic_log" \
  RESTIC_MAINTENANCE_REPOSITORY='rest:https://backup.test.invalid/family-finance-prod/' \
  RESTIC_PASSWORD_FILE="$password_file" \
  BACKUP_KEEP_WITHIN=2y \
  bash "$maintenance" >"$workdir/network.out" 2>&1; then
  fail "maintenance accepted a network repository"
fi
grep -qi 'local' "$workdir/network.out" || fail "network repository rejection did not explain the local-only boundary"

chmod 0644 "$password_file"
if PATH="$workdir/bin:$PATH" \
  RESTIC_TEST_LOG="$restic_log" \
  RESTIC_MAINTENANCE_REPOSITORY="$workdir/repository" \
  RESTIC_PASSWORD_FILE="$password_file" \
  BACKUP_KEEP_WITHIN=2y \
  bash "$maintenance" >"$workdir/public-password.out" 2>&1; then
  fail "maintenance accepted a group/world-readable repository password"
fi
grep -qiE 'permission|group|other|mode' "$workdir/public-password.out" || fail "maintenance password-permission rejection was unclear"
chmod 0600 "$password_file"

if PATH="$workdir/bin:$PATH" \
  RESTIC_TEST_LOG="$restic_log" \
  RESTIC_MAINTENANCE_REPOSITORY="$workdir/repository" \
  RESTIC_PASSWORD_FILE="$password_file" \
  BACKUP_KEEP_WITHIN=0d \
  bash "$maintenance" >"$workdir/zero-retention.out" 2>&1; then
  fail "maintenance accepted a zero retention window"
fi
grep -qiE 'positive|duration|keep-within' "$workdir/zero-retention.out" || fail "zero retention rejection was unclear"

: >"$restic_log"
PATH="$workdir/bin:$PATH" \
  RESTIC_TEST_LOG="$restic_log" \
  RESTIC_MAINTENANCE_REPOSITORY="$workdir/repository" \
  RESTIC_PASSWORD_FILE="$password_file" \
  BACKUP_KEEP_WITHIN=2y \
  bash "$maintenance" >"$workdir/valid.out" 2>&1 || fail "maintenance rejected a valid local repository configuration"

grep -Fxq 'snapshots' "$restic_log" || fail "maintenance did not list snapshots before destructive work"
grep -Fxq 'forget --keep-within 2y --prune' "$restic_log" || fail "maintenance did not use the expected keep-within prune policy"
grep -Fxq 'check' "$restic_log" || fail "maintenance did not run repository check after prune"

echo "Append-only backup contract OK"
