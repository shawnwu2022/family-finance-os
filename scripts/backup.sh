#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [[ ! -f .env ]]; then
  echo "ERROR: .env not found" >&2
  exit 1
fi

set -a
# shellcheck disable=SC1091
source .env
set +a

STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DEST="$ROOT_DIR/backups/$STAMP"
mkdir -p "$DEST"

for db in "${FINANCE_DB_NAME:-finance}" "${EBK_DB_NAME:-ezbookkeeping}"; do
  docker compose exec -T postgres pg_dump \
    -U "$POSTGRES_USER" \
    -d "$db" \
    -Fc > "$DEST/${db}.dump"
  docker compose exec -T postgres pg_restore --list < "$DEST/${db}.dump" > /dev/null
done

tar -C "$ROOT_DIR/data" -czf "$DEST/ezbookkeeping-storage.tar.gz" ezbookkeeping-storage
(
  cd "$DEST"
  sha256sum *.dump *.tar.gz > SHA256SUMS
)

if [[ -n "${BACKUP_REMOTE:-}" ]]; then
  rsync -a "$DEST/" "${BACKUP_REMOTE%/}/$STAMP/"
fi

RETENTION="${BACKUP_RETENTION_DAYS:-7}"
find "$ROOT_DIR/backups" -mindepth 1 -maxdepth 1 -type d -mtime "+$RETENTION" -print -exec rm -rf {} +

echo "Backup completed: $DEST"
