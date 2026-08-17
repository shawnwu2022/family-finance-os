#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "restore drill failed: $*" >&2
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

BACKUP_DIR="${1:-${RESTORE_DRILL_BACKUP_DIR:-}}"
[[ -n "$BACKUP_DIR" ]] || fail "usage: scripts/restore-drill.sh <backup-directory>"
BACKUP_DIR="$(cd "$BACKUP_DIR" && pwd -P)"
[[ -f "$BACKUP_DIR/SHA256SUMS" ]] || fail "SHA256SUMS is missing"
[[ -f "$BACKUP_DIR/${FINANCE_DB}.dump" ]] || fail "finance database dump is missing"
[[ -f "$BACKUP_DIR/${EBK_DB}.dump" ]] || fail "ezBookkeeping database dump is missing"
[[ -f "$BACKUP_DIR/ezbookkeeping-storage.tar.gz" ]] || fail "ezBookkeeping storage archive is missing"

(
  cd "$BACKUP_DIR"
  sha256sum -c SHA256SUMS
)

postgres_exec() {
  if [[ -n "${POSTGRES_DOCKER_CONTAINER:-}" ]]; then
    docker exec -i "$POSTGRES_DOCKER_CONTAINER" "$@"
  else
    docker compose exec -T postgres "$@"
  fi
}

suffix="$(date -u +%Y%m%d%H%M%S)_$$"
finance_restore_drill="finance_restore_drill_${suffix}"
ebookkeeping_restore_drill="ezbookkeeping_restore_drill_${suffix}"

cleanup() {
  postgres_exec dropdb -U "$POSTGRES_USER" --if-exists "$finance_restore_drill" >/dev/null 2>&1 || true
  postgres_exec dropdb -U "$POSTGRES_USER" --if-exists "$ebookkeeping_restore_drill" >/dev/null 2>&1 || true
  if [[ -n "${storage_tmp:-}" && -d "$storage_tmp" ]]; then
    rm -rf "$storage_tmp"
  fi
}
trap cleanup EXIT

postgres_exec createdb -U "$POSTGRES_USER" "$finance_restore_drill"
postgres_exec createdb -U "$POSTGRES_USER" "$ebookkeeping_restore_drill"
postgres_exec pg_restore -U "$POSTGRES_USER" -d "$finance_restore_drill" < "$BACKUP_DIR/${FINANCE_DB}.dump"
postgres_exec pg_restore -U "$POSTGRES_USER" -d "$ebookkeeping_restore_drill" < "$BACKUP_DIR/${EBK_DB}.dump"

finance_tables="$(postgres_exec psql -U "$POSTGRES_USER" -d "$finance_restore_drill" -Atqc "SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname = 'public'")"
ebookkeeping_tables="$(postgres_exec psql -U "$POSTGRES_USER" -d "$ebookkeeping_restore_drill" -Atqc "SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname = 'public'")"
[[ "$finance_tables" =~ ^[1-9][0-9]*$ ]] || fail "restored finance database has no public tables"
[[ "$ebookkeeping_tables" =~ ^[1-9][0-9]*$ ]] || fail "restored ezBookkeeping database has no public tables"

while IFS= read -r archive_path; do
  case "$archive_path" in
    /*|../*|*/../*|*/..) fail "unsafe path in storage archive: $archive_path" ;;
  esac
done < <(tar -tzf "$BACKUP_DIR/ezbookkeeping-storage.tar.gz")

storage_tmp="$(mktemp -d)"
tar -xzf "$BACKUP_DIR/ezbookkeeping-storage.tar.gz" -C "$storage_tmp"
[[ -d "$storage_tmp/ezbookkeeping-storage" ]] || fail "storage archive does not contain ezbookkeeping-storage"

echo "Restore drill OK: finance_tables=$finance_tables ezbookkeeping_tables=$ebookkeeping_tables"
