#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT_DIR"

for cmd in docker openssl stat; do
  command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done

docker compose version >/dev/null

require_private_file() {
  local path="$1"
  local label="$2"
  [[ -f "$path" ]] || { echo "ERROR: ${label} must be a regular file." >&2; exit 1; }

  local mode
  mode="$(stat -Lc '%a' "$path")" || { echo "ERROR: could not inspect ${label} file mode." >&2; exit 1; }
  [[ "$mode" =~ ^[0-7]{3,4}$ ]] || { echo "ERROR: invalid ${label} file mode: ${mode}." >&2; exit 1; }
  local permissions="${mode: -3}"
  local group_digit="${permissions:1:1}"
  local other_digit="${permissions:2:1}"
  if [[ "$group_digit" != "0" || "$other_digit" != "0" ]]; then
    echo "ERROR: ${label} permissions are too broad (mode ${mode}); group and other access must be disabled." >&2
    exit 1
  fi
}

if [[ ! -f .env ]]; then
  echo "Create .env from .env.example before deployment." >&2
  exit 1
fi
require_private_file .env ".env"

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
  require_private_file "$RESTIC_PASSWORD_FILE" "RESTIC_PASSWORD_FILE"

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
