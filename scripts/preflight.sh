#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT_DIR"

for cmd in docker openssl; do
  command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done

docker compose version >/dev/null

if [[ ! -f .env ]]; then
  echo "Create .env from .env.example before deployment." >&2
  exit 1
fi

if grep -Eq 'REPLACE_WITH|example\.com' .env; then
  echo "ERROR: .env still contains deployment placeholders." >&2
  exit 1
fi

for key in FINANCE_AUTH_USER FINANCE_AUTH_HASH; do
  if ! grep -Eq "^${key}=[^[:space:]]+" .env; then
    echo "ERROR: ${key} must be set for the public Finance Core edge." >&2
    exit 1
  fi
done

set -a
# shellcheck disable=SC1091
source .env
set +a

if [[ -n "${BACKUP_REMOTE:-}" ]]; then
  echo "ERROR: BACKUP_REMOTE is deprecated; use RESTIC_REPOSITORY and RESTIC_PASSWORD_FILE." >&2
  exit 1
fi

if [[ -n "${RESTIC_PASSWORD_FILE:-}" && -z "${RESTIC_REPOSITORY:-}" ]]; then
  echo "ERROR: RESTIC_PASSWORD_FILE is set but RESTIC_REPOSITORY is empty." >&2
  exit 1
fi

if [[ -n "${RESTIC_REPOSITORY:-}" ]]; then
  command -v restic >/dev/null || { echo "Missing command: restic" >&2; exit 1; }
  [[ "$RESTIC_REPOSITORY" == sftp:* ]] || { echo "ERROR: RESTIC_REPOSITORY must use restic SFTP syntax for V1." >&2; exit 1; }
  [[ -n "${RESTIC_PASSWORD_FILE:-}" ]] || { echo "ERROR: RESTIC_PASSWORD_FILE is required with RESTIC_REPOSITORY." >&2; exit 1; }
  [[ -r "$RESTIC_PASSWORD_FILE" ]] || { echo "ERROR: RESTIC_PASSWORD_FILE is not readable." >&2; exit 1; }

  password_path="$(cd "$(dirname "$RESTIC_PASSWORD_FILE")" && pwd -P)/$(basename "$RESTIC_PASSWORD_FILE")"
  case "$password_path" in
    "$ROOT_DIR"/*)
      echo "ERROR: RESTIC_PASSWORD_FILE must live outside the repository." >&2
      exit 1
      ;;
  esac
fi

docker compose config >/dev/null

echo "Preflight OK"
