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

if [[ -n "${RESTIC_REST_PASSWORD:-}" ]]; then
  echo "ERROR: RESTIC_REST_PASSWORD must not be set; use RESTIC_REST_PASSWORD_FILE." >&2
  exit 1
fi

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

  require_private_file "$RESTIC_PASSWORD_FILE" "RESTIC_PASSWORD_FILE"
  require_private_file "$RESTIC_REST_PASSWORD_FILE" "RESTIC_REST_PASSWORD_FILE"

  for secret_file in "$RESTIC_PASSWORD_FILE" "$RESTIC_REST_PASSWORD_FILE"; do
    secret_path="$(readlink -f -- "$secret_file")" || {
      echo "ERROR: could not resolve backup secret file path." >&2
      exit 1
    }
    case "$secret_path" in
      "$ROOT_DIR"/*)
        echo "ERROR: backup secret files must live outside the repository." >&2
        exit 1
        ;;
    esac
  done
fi

case "${MCP_ENABLED:-false}" in
  true|TRUE|True|1|t|T)
    mcp_container_token="${MCP_TOKEN_FILE:-/run/secrets/finance-mcp-token}"
    case "$mcp_container_token" in
      /run/secrets/*)
        mcp_token_name="${mcp_container_token#/run/secrets/}"
        ;;
      *)
        echo "ERROR: MCP_TOKEN_FILE must reference the Compose-mounted /run/secrets directory." >&2
        exit 1
        ;;
    esac
    if [[ -z "$mcp_token_name" || "$mcp_token_name" == "." || "$mcp_token_name" == ".." || "$mcp_token_name" == */* ]]; then
      echo "ERROR: MCP_TOKEN_FILE must reference one file directly under /run/secrets." >&2
      exit 1
    fi
    require_private_file "$ROOT_DIR/secrets/$mcp_token_name" "MCP bearer host file"
    ;;
esac

docker compose config >/dev/null

echo "Preflight OK"
