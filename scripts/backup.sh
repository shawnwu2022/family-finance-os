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
mkdir -p "$BACKUP_ROOT"
BACKUP_ROOT="$(cd "$BACKUP_ROOT" && pwd -P)"
[[ "$BACKUP_ROOT" != "/" ]] || fail "FINANCE_BACKUP_DIR must not be /"
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
  fail "BACKUP_REMOTE is deprecated; configure RESTIC_REPOSITORY and backup secret files"
fi

if [[ -n "${RESTIC_REPOSITORY:-}" ]]; then
  command -v restic >/dev/null 2>&1 || fail "restic is required when RESTIC_REPOSITORY is configured"
  [[ "$RESTIC_REPOSITORY" == rest:https://* ]] || fail "V1 off-site backup repository must use rest:https://"
  [[ -n "${RESTIC_PASSWORD_FILE:-}" ]] || fail "RESTIC_PASSWORD_FILE is required when RESTIC_REPOSITORY is configured"
  [[ -n "${RESTIC_REST_USERNAME:-}" ]] || fail "RESTIC_REST_USERNAME is required when RESTIC_REPOSITORY is configured"
  [[ -n "${RESTIC_REST_PASSWORD_FILE:-}" ]] || fail "RESTIC_REST_PASSWORD_FILE is required when RESTIC_REPOSITORY is configured"
  [[ -r "$RESTIC_PASSWORD_FILE" ]] || fail "RESTIC_PASSWORD_FILE is not readable"
  [[ -r "$RESTIC_REST_PASSWORD_FILE" ]] || fail "RESTIC_REST_PASSWORD_FILE is not readable"

  for secret_file in "$RESTIC_PASSWORD_FILE" "$RESTIC_REST_PASSWORD_FILE"; do
    secret_path="$(cd "$(dirname "$secret_file")" && pwd -P)/$(basename "$secret_file")"
    case "$secret_path" in
      "$ROOT_DIR"/*) fail "backup secret files must live outside the repository" ;;
    esac
  done

  RESTIC_REST_PASSWORD="$(<"$RESTIC_REST_PASSWORD_FILE")" \
    RESTIC_REST_USERNAME="$RESTIC_REST_USERNAME" \
    RESTIC_REPOSITORY="$RESTIC_REPOSITORY" \
    RESTIC_PASSWORD_FILE="$RESTIC_PASSWORD_FILE" \
    restic snapshots --json >/dev/null

  RESTIC_REST_PASSWORD="$(<"$RESTIC_REST_PASSWORD_FILE")" \
    RESTIC_REST_USERNAME="$RESTIC_REST_USERNAME" \
    RESTIC_REPOSITORY="$RESTIC_REPOSITORY" \
    RESTIC_PASSWORD_FILE="$RESTIC_PASSWORD_FILE" \
    restic backup "$DEST" --tag family-finance-os --tag "$STAMP"
fi

RETENTION="${BACKUP_RETENTION_DAYS:-14}"
[[ "$RETENTION" =~ ^[0-9]+$ ]] || fail "BACKUP_RETENTION_DAYS must be a non-negative integer"
find "$BACKUP_ROOT" \
  -mindepth 1 \
  -maxdepth 1 \
  -type d \
  -name '20??????T??????Z' \
  -mtime "+$RETENTION" \
  -print \
  -exec rm -rf -- {} +

echo "Backup completed: $DEST"
