#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "backup failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ENV_FILE="${FINANCE_ENV_FILE:-$ROOT_DIR/.env}"
[[ -f "$ENV_FILE" ]] || fail "environment file not found: $ENV_FILE"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

: "${POSTGRES_USER:?POSTGRES_USER is required}"
FINANCE_DB="${FINANCE_DB_NAME:-finance}"
EBK_DB="${EBK_DB_NAME:-ezbookkeeping}"
for db in "$FINANCE_DB" "$EBK_DB"; do
  [[ "$db" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || fail "database name contains unsupported characters: $db"
done

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_ROOT="${FINANCE_BACKUP_DIR:-$ROOT_DIR/backups}"
DEST="$BACKUP_ROOT/$STAMP"
STORAGE_DIR="$ROOT_DIR/data/ezbookkeeping-storage"
mkdir -p "$DEST"
[[ -d "$STORAGE_DIR" ]] || fail "ezBookkeeping storage directory not found: $STORAGE_DIR"

for db in "$FINANCE_DB" "$EBK_DB"; do
  docker compose exec -T postgres pg_dump \
    -U "$POSTGRES_USER" \
    -d "$db" \
    -Fc > "$DEST/${db}.dump"
  docker compose exec -T postgres pg_restore --list < "$DEST/${db}.dump" > /dev/null
done

tar -C "$ROOT_DIR/data" -czf "$DEST/ezbookkeeping-storage.tar.gz" ezbookkeeping-storage
(
  cd "$DEST"
  sha256sum "${FINANCE_DB}.dump" "${EBK_DB}.dump" ezbookkeeping-storage.tar.gz > SHA256SUMS
)

if [[ -n "${BACKUP_REMOTE:-}" ]]; then
  fail "BACKUP_REMOTE is deprecated; configure RESTIC_REPOSITORY and RESTIC_PASSWORD_FILE"
fi

if [[ -n "${RESTIC_REPOSITORY:-}" ]]; then
  command -v restic >/dev/null 2>&1 || fail "restic is required when RESTIC_REPOSITORY is configured"
  [[ "$RESTIC_REPOSITORY" == sftp:* ]] || fail "V1 off-site backup repository must use restic SFTP syntax"
  [[ -n "${RESTIC_PASSWORD_FILE:-}" ]] || fail "RESTIC_PASSWORD_FILE is required when RESTIC_REPOSITORY is configured"
  [[ -r "$RESTIC_PASSWORD_FILE" ]] || fail "RESTIC_PASSWORD_FILE is not readable"

  password_path="$(cd "$(dirname "$RESTIC_PASSWORD_FILE")" && pwd -P)/$(basename "$RESTIC_PASSWORD_FILE")"
  case "$password_path" in
    "$ROOT_DIR"/*) fail "RESTIC_PASSWORD_FILE must live outside the repository" ;;
  esac

  export RESTIC_REPOSITORY RESTIC_PASSWORD_FILE
  restic snapshots --json >/dev/null
  restic backup "$DEST" --tag family-finance-os --tag "$STAMP"
  restic forget \
    --keep-daily "${BACKUP_KEEP_DAILY:-14}" \
    --keep-weekly "${BACKUP_KEEP_WEEKLY:-8}" \
    --keep-monthly "${BACKUP_KEEP_MONTHLY:-12}" \
    --prune
  restic check
fi

RETENTION="${BACKUP_RETENTION_DAYS:-14}"
[[ "$RETENTION" =~ ^[0-9]+$ ]] || fail "BACKUP_RETENTION_DAYS must be a non-negative integer"
find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d -mtime "+$RETENTION" -print -exec rm -rf {} +

echo "Backup completed: $DEST"
