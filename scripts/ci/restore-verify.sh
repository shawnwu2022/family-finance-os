#!/usr/bin/env bash
set -euo pipefail
umask 077

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

: "${COMPOSE_PROJECT_NAME:?COMPOSE_PROJECT_NAME is required}"
COMPOSE_FILE_PATH="${CI_COMPOSE_FILE:-compose.ci.yaml}"
compose=(docker compose -p "$COMPOSE_PROJECT_NAME" -f "$COMPOSE_FILE_PATH")

postgres_container="$(${compose[@]} ps -q postgres)"
[[ -n "$postgres_container" ]] || {
  echo "restore verification requires the CI postgres service to be running" >&2
  exit 1
}

EBK_SOURCE_DB="ezbookkeeping_ci_$$"
TMP_ROOT="$(mktemp -d /tmp/family-finance-restore-verify.XXXXXX)"
BACKUP_DIR="$TMP_ROOT/backup"
STORAGE_ROOT="$TMP_ROOT/storage"
ENV_FILE="$TMP_ROOT/restore-drill.env"
mkdir -p "$BACKUP_DIR" "$STORAGE_ROOT/ezbookkeeping-storage"

cleanup() {
  docker exec "$postgres_container" dropdb -U finance_app --if-exists "$EBK_SOURCE_DB" >/dev/null 2>&1 || true
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

docker exec "$postgres_container" createdb -U finance_app "$EBK_SOURCE_DB"
docker exec "$postgres_container" psql -U finance_app -d "$EBK_SOURCE_DB" -v ON_ERROR_STOP=1 \
  -c 'CREATE TABLE restore_probe (id bigint PRIMARY KEY); INSERT INTO restore_probe VALUES (1);'

docker exec "$postgres_container" pg_dump -U finance_app -d finance -Fc > "$BACKUP_DIR/finance.dump"
docker exec "$postgres_container" pg_dump -U finance_app -d "$EBK_SOURCE_DB" -Fc > "$BACKUP_DIR/${EBK_SOURCE_DB}.dump"
printf 'restore-drill\n' > "$STORAGE_ROOT/ezbookkeeping-storage/probe.txt"
tar -C "$STORAGE_ROOT" -czf "$BACKUP_DIR/ezbookkeeping-storage.tar.gz" ezbookkeeping-storage
(
  cd "$BACKUP_DIR"
  sha256sum finance.dump "${EBK_SOURCE_DB}.dump" ezbookkeeping-storage.tar.gz > SHA256SUMS
)
printf 'POSTGRES_USER=finance_app\nFINANCE_DB_NAME=finance\nEBK_DB_NAME=%s\n' "$EBK_SOURCE_DB" > "$ENV_FILE"

FINANCE_ENV_FILE="$ENV_FILE" POSTGRES_DOCKER_CONTAINER="$postgres_container" \
  bash scripts/restore-drill.sh "$BACKUP_DIR"

echo "Backup/restore verification OK"
