#!/usr/bin/env bash
set -euo pipefail

if [[ -n "${RESTIC_REST_PASSWORD:-}" ]]; then
  echo "ERROR: RESTIC_REST_PASSWORD must not be set; use RESTIC_REST_PASSWORD_FILE." >&2
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT_DIR"

for cmd in docker openssl stat readlink; do
  command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done

docker compose version >/dev/null

require_private_file() {
  local path="$1"
  local label="$2"
  [[ -f "$path" ]] || { echo "ERROR: ${label} must be a regular file." >&2; exit 1; }
  [[ -r "$path" ]] || { echo "ERROR: ${label} must be readable by the deployment account." >&2; exit 1; }
  [[ -s "$path" ]] || { echo "ERROR: ${label} must not be empty." >&2; exit 1; }

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

require_external_private_file() {
  local path="$1"
  local label="$2"
  [[ -n "$path" ]] || { echo "ERROR: ${label} path must be set." >&2; exit 1; }
  require_private_file "$path" "$label"

  local resolved
  resolved="$(readlink -f -- "$path")" || { echo "ERROR: could not resolve ${label} path." >&2; exit 1; }
  case "$resolved" in
    "$ROOT_DIR"/*)
      echo "ERROR: ${label} must live outside the repository." >&2
      exit 1
      ;;
  esac
}

if [[ ! -f .env ]]; then
  echo "Create .env from .env.example before deployment." >&2
  exit 1
fi
require_private_file .env ".env"

# Match active assignment/content lines in one grep process. Avoid a filter | grep -q
# pipeline here: with pipefail, an early successful -q can SIGPIPE the producer and
# turn a real placeholder match into a false negative for a sufficiently large .env.
if grep -Eq '^[[:space:]]*[^#[:space:]].*(REPLACE_WITH|example\.com)' .env; then
  echo "ERROR: .env still contains deployment placeholders." >&2
  exit 1
fi

if grep -Eq '^[[:space:]]*(export[[:space:]]+)?RESTIC_REST_PASSWORD[[:space:]]*=' .env; then
  echo "ERROR: RESTIC_REST_PASSWORD must not be set; use RESTIC_REST_PASSWORD_FILE." >&2
  exit 1
fi

for legacy in FINANCE_AUTH_USER FINANCE_AUTH_HASH EBK_API_TOKEN EBK_SECURITY_SECRET_KEY; do
  if [[ -n "${!legacy:-}" ]] || grep -Eq "^[[:space:]]*(export[[:space:]]+)?${legacy}[[:space:]]*=" .env; then
    echo "ERROR: ${legacy} is forbidden in the application-native auth deployment; use the documented secret-file boundary instead." >&2
    exit 1
  fi
done

set -a
# shellcheck disable=SC1091
source .env
set +a

if [[ -n "${RESTIC_REST_PASSWORD:-}" ]]; then
  echo "ERROR: RESTIC_REST_PASSWORD must not be set; use RESTIC_REST_PASSWORD_FILE." >&2
  exit 1
fi

for legacy in FINANCE_AUTH_USER FINANCE_AUTH_HASH EBK_API_TOKEN EBK_SECURITY_SECRET_KEY; do
  if [[ -n "${!legacy:-}" ]]; then
    echo "ERROR: ${legacy} is forbidden in the application-native auth deployment; use the documented secret-file boundary instead." >&2
    exit 1
  fi
done

case "${EBK_USER_ENABLE_REGISTER:-false}" in
  false|FALSE|False|0|f|F) ;;
  *)
    echo "ERROR: EBK_USER_ENABLE_REGISTER must be false for steady-state production; use a one-time explicit override only while creating the first owner." >&2
    exit 1
    ;;
esac

for spec in \
  "FINANCE_AUTH_KEY_HOST_FILE:Finance auth key host file" \
  "FINANCE_ADMIN_PASSWORD_HOST_FILE:Finance administrator password host file" \
  "EBK_API_TOKEN_HOST_FILE:ezBookkeeping API token host file" \
  "EBK_SECURITY_SECRET_KEY_HOST_FILE:ezBookkeeping security secret host file"; do
  key="${spec%%:*}"
  label="${spec#*:}"
  path="${!key:-}"
  require_external_private_file "$path" "$label"
done

if [[ -n "${BACKUP_REMOTE:-}" ]]; then
  echo "ERROR: BACKUP_REMOTE is deprecated; use RESTIC_REPOSITORY and backup secret files." >&2
  exit 1
fi

for legacy in BACKUP_KEEP_DAILY BACKUP_KEEP_WEEKLY BACKUP_KEEP_MONTHLY; do
  if [[ -n "${!legacy:-}" ]]; then
    echo "ERROR: ${legacy} is obsolete on the production producer; retention/prune belongs to the backup-maintenance host." >&2
    exit 1
  fi
done

if [[ -n "${RESTIC_PASSWORD_FILE:-}${RESTIC_REST_USERNAME:-}${RESTIC_REST_PASSWORD_FILE:-}" && -z "${RESTIC_REPOSITORY:-}" ]]; then
  echo "ERROR: restic producer credentials are set but RESTIC_REPOSITORY is empty." >&2
  exit 1
fi

if [[ -n "${RESTIC_REPOSITORY:-}" ]]; then
  command -v restic >/dev/null || { echo "Missing command: restic" >&2; exit 1; }
  [[ "$RESTIC_REPOSITORY" == rest:https://* ]] || { echo "ERROR: RESTIC_REPOSITORY must use rest:https:// for the V1 production off-site contract." >&2; exit 1; }
  rest_endpoint="${RESTIC_REPOSITORY#rest:https://}"
  rest_authority="${rest_endpoint%%/*}"
  [[ -n "$rest_authority" ]] || { echo "ERROR: RESTIC_REPOSITORY must include an HTTPS authority." >&2; exit 1; }
  [[ "$rest_authority" != *"@"* ]] || { echo "ERROR: RESTIC_REPOSITORY must not embed credentials in the URL." >&2; exit 1; }
  [[ -n "${RESTIC_PASSWORD_FILE:-}" ]] || { echo "ERROR: RESTIC_PASSWORD_FILE is required with RESTIC_REPOSITORY." >&2; exit 1; }
  [[ -n "${RESTIC_REST_USERNAME:-}" ]] || { echo "ERROR: RESTIC_REST_USERNAME is required with RESTIC_REPOSITORY." >&2; exit 1; }
  [[ -n "${RESTIC_REST_PASSWORD_FILE:-}" ]] || { echo "ERROR: RESTIC_REST_PASSWORD_FILE is required with RESTIC_REPOSITORY." >&2; exit 1; }

  rest_path="${rest_endpoint#*/}"
  [[ "$rest_path" != "$rest_endpoint" && -n "$rest_path" ]] || { echo "ERROR: RESTIC_REPOSITORY must include the private repository path." >&2; exit 1; }
  rest_private_repo="${rest_path%%/*}"
  [[ "$rest_private_repo" == "$RESTIC_REST_USERNAME" ]] || { echo "ERROR: RESTIC_REPOSITORY first path component must match RESTIC_REST_USERNAME for --private-repos." >&2; exit 1; }

  require_external_private_file "$RESTIC_PASSWORD_FILE" "RESTIC_PASSWORD_FILE"
  require_external_private_file "$RESTIC_REST_PASSWORD_FILE" "RESTIC_REST_PASSWORD_FILE"
fi

case "${MCP_ENABLED:-false}" in
  true|TRUE|True|1|t|T)
    require_external_private_file "${MCP_TOKEN_HOST_FILE:-}" "MCP bearer host file"
    ;;
  false|FALSE|False|0|f|F|'') ;;
  *)
    echo "ERROR: MCP_ENABLED must be a boolean." >&2
    exit 1
    ;;
esac

docker compose config >/dev/null

echo "Preflight OK"
