#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "backup maintenance failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
REPOSITORY="${RESTIC_MAINTENANCE_REPOSITORY:-}"
PASSWORD_FILE="${RESTIC_PASSWORD_FILE:-}"
KEEP_WITHIN="${BACKUP_KEEP_WITHIN:-2y}"

command -v restic >/dev/null 2>&1 || fail "restic is required"
[[ -n "$REPOSITORY" ]] || fail "RESTIC_MAINTENANCE_REPOSITORY is required"
case "$REPOSITORY" in
  rest:*|sftp:*|rclone:*|http:*|https:*|*://*) fail "maintenance repository must be local, not a network backend" ;;
esac
[[ "$REPOSITORY" == /* ]] || fail "RESTIC_MAINTENANCE_REPOSITORY must be an absolute local filesystem path"
[[ -d "$REPOSITORY" ]] || fail "RESTIC_MAINTENANCE_REPOSITORY must be an existing directory"

[[ -n "$PASSWORD_FILE" ]] || fail "RESTIC_PASSWORD_FILE is required"
[[ -f "$PASSWORD_FILE" && -r "$PASSWORD_FILE" ]] || fail "RESTIC_PASSWORD_FILE must be a readable regular file"
mode="$(stat -Lc '%a' "$PASSWORD_FILE")" || fail "could not inspect RESTIC_PASSWORD_FILE mode"
[[ "$mode" =~ ^[0-7]{3,4}$ ]] || fail "invalid RESTIC_PASSWORD_FILE mode: $mode"
permissions="${mode: -3}"
[[ "${permissions:1:1}" == "0" && "${permissions:2:1}" == "0" ]] || fail "RESTIC_PASSWORD_FILE group/other permissions must be disabled"
password_path="$(cd "$(dirname "$PASSWORD_FILE")" && pwd -P)/$(basename "$PASSWORD_FILE")"
case "$password_path" in
  "$ROOT_DIR"/*) fail "RESTIC_PASSWORD_FILE must live outside the repository" ;;
esac

[[ -n "$KEEP_WITHIN" && "$KEEP_WITHIN" != -* ]] || fail "BACKUP_KEEP_WITHIN must be a positive restic duration"

export RESTIC_REPOSITORY="$REPOSITORY"
export RESTIC_PASSWORD_FILE="$PASSWORD_FILE"

restic snapshots
restic forget --keep-within "$KEEP_WITHIN" --prune
restic check

echo "Backup maintenance completed"
