#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "backup failed: $*" >&2
  exit 1
}

if [[ -n "${RESTIC_REST_PASSWORD:-}" ]]; then
  fail "RESTIC_REST_PASSWORD must not be set; use RESTIC_REST_PASSWORD_FILE"
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

require_private_backup_file() {
  local path="$1"
  local label="$2"

  [[ -f "$path" ]] || fail "${label} must be a regular file"
  [[ -r "$path" ]] || fail "${label} is not readable"

  local mode
  mode="$(stat -Lc '%a' "$path")" || fail "could not inspect ${label} file mode"
  [[ "$mode" =~ ^[0-7]{3,4}$ ]] || fail "invalid ${label} file mode: $mode"
  local permissions="${mode: -3}"
  if [[ "${permissions:1:1}" != "0" || "${permissions:2:1}" != "0" ]]; then
    fail "${label} group and other access must be disabled (mode ${mode})"
  fi
}

ENV_FILE="${FINANCE_ENV_FILE:-$ROOT_DIR/.env}"
[[ -f "$ENV_FILE" ]] || fail "environment file not found: $ENV_FILE"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

if [[ -n "${RESTIC_REST_PASSWORD:-}" ]]; then
  fail "RESTIC_REST_PASSWORD must not be set; use RESTIC_REST_PASSWORD_FILE"
fi

if [[ -n "${RESTIC_PASSWORD_FILE:-}${RESTIC_REST_USERNAME:-}${RESTIC_REST_PASSWORD_FILE:-}" && -z "${RESTIC_REPOSITORY:-}" ]]; then
  fail "restic producer credentials are set but RESTIC_REPOSITORY is empty"
fi

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
  command -v readlink >/dev/null 2>&1 || fail "readlink is required when RESTIC_REPOSITORY is configured"
  command -v stat >/dev/null 2>&1 || fail "stat is required when RESTIC_REPOSITORY is configured"
  [[ "$RESTIC_REPOSITORY" == rest:https://* ]] || fail "V1 off-site backup repository must use rest:https://"
  rest_endpoint="${RESTIC_REPOSITORY#rest:https://}"
  rest_authority="${rest_endpoint%%/*}"
  [[ -n "$rest_authority" ]] || fail "RESTIC_REPOSITORY must include an HTTPS authority"
  [[ "$rest_authority" != *"@"* ]] || fail "RESTIC_REPOSITORY must not embed credentials in the URL"
  [[ -n "${RESTIC_PASSWORD_FILE:-}" ]] || fail "RESTIC_PASSWORD_FILE is required when RESTIC_REPOSITORY is configured"
  [[ -n "${RESTIC_REST_USERNAME:-}" ]] || fail "RESTIC_REST_USERNAME is required when RESTIC_REPOSITORY is configured"
  [[ -n "${RESTIC_REST_PASSWORD_FILE:-}" ]] || fail "RESTIC_REST_PASSWORD_FILE is required when RESTIC_REPOSITORY is configured"

  require_private_backup_file "$RESTIC_PASSWORD_FILE" "RESTIC_PASSWORD_FILE"
  require_private_backup_file "$RESTIC_REST_PASSWORD_FILE" "RESTIC_REST_PASSWORD_FILE"

  for secret_file in "$RESTIC_PASSWORD_FILE" "$RESTIC_REST_PASSWORD_FILE"; do
    secret_path="$(readlink -f -- "$secret_file")" || fail "could not resolve backup secret file path"
    case "$secret_path" in
      "$ROOT_DIR"/*) fail "backup secret files must live outside the repository" ;;
    esac
  done

  run_restic() {
    RESTIC_REST_PASSWORD="$(<"$RESTIC_REST_PASSWORD_FILE")" \
      RESTIC_REST_USERNAME="$RESTIC_REST_USERNAME" \
      RESTIC_REPOSITORY="$RESTIC_REPOSITORY" \
      RESTIC_PASSWORD_FILE="$RESTIC_PASSWORD_FILE" \
      restic "$@"
  }

  run_restic snapshots --json >/dev/null
  run_restic backup --group-by '' "$DEST" --tag family-finance-os --tag "$STAMP"
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
