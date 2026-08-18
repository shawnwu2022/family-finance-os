#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "production operations contract failed: $*" >&2
  exit 1
}

backup="scripts/backup.sh"
restore="scripts/restore-drill.sh"
preflight="scripts/preflight.sh"
edge="scripts/check-edge-security.sh"
live_smoke="scripts/acceptance/ezbookkeeping-live-smoke.sh"
openclaw_smoke="scripts/acceptance/openclaw-mcp-live-smoke.sh"

for script in "$backup" "$restore" "$preflight" "$edge" "$live_smoke" "$openclaw_smoke"; do
  [[ -f "$script" ]] || fail "required script is missing: $script"
  bash -n "$script" || fail "shell syntax is invalid: $script"
done

grep -Fq 'pg_dump' "$backup" || fail "backup must invoke pg_dump"
grep -Fq -- '-Fc' "$backup" || fail "backup must use PostgreSQL custom format"
grep -Fq 'ezbookkeeping-storage.tar.gz' "$backup" || fail "backup must archive ezBookkeeping storage"
grep -Fq 'RESTIC_REPOSITORY' "$backup" || fail "backup must support a restic repository"
grep -Fq 'RESTIC_PASSWORD_FILE' "$backup" || fail "restic automation must use a password file"
grep -Eq 'restic[[:space:]]+backup' "$backup" || fail "backup must send the snapshot through restic"
grep -Fq 'FINANCE_BACKUP_DIR must not be /' "$backup" || fail "backup must reject filesystem root as the retention directory"
grep -Fq -- "-name '20??????T??????Z'" "$backup" || fail "retention must only remove script-generated timestamp directories"
if grep -Eq '(^|[[:space:]])rsync([[:space:]]|$)' "$backup"; then
  fail "raw rsync is not an encrypted off-site backup path"
fi

grep -Fq 'sha256sum -c' "$restore" || fail "restore drill must verify backup checksums"
grep -Fq 'pg_restore' "$restore" || fail "restore drill must exercise pg_restore"
grep -Fq 'finance_restore_drill' "$restore" || fail "restore drill must use isolated scratch databases"

grep -Fq 'RESTIC_REPOSITORY' "$preflight" || fail "preflight must validate restic repository configuration"
grep -Fq 'RESTIC_PASSWORD_FILE' "$preflight" || fail "preflight must validate restic password file"
grep -Fq 'must live outside the repository' "$preflight" || fail "preflight must reject repository-local restic password files"

grep -Fq 'host port exposure' "$edge" || fail "edge security must enforce host-port exposure"
grep -Fq 'network_mode' "$edge" || fail "edge security must reject host networking"

grep -Fq 'https://' "$live_smoke" || fail "live ezBookkeeping smoke must require HTTPS"
grep -Fq 'sha256sum' "$live_smoke" || fail "live ezBookkeeping smoke must emit only hashed response evidence"
grep -Fq 'transaction ledger is empty' "$live_smoke" || fail "live ezBookkeeping smoke must reject an empty ledger by default"

grep -Fq 'openclaw mcp probe' "$openclaw_smoke" || fail "OpenClaw live smoke must run a real MCP probe"
grep -Fq 'openclaw agent exec' "$openclaw_smoke" || fail "OpenClaw live smoke must run real agent turns"
grep -Fq 'get_household_overview' "$openclaw_smoke" || fail "OpenClaw live smoke must exercise a read tool"
grep -Fq 'simulate_purchase' "$openclaw_smoke" || fail "OpenClaw live smoke must exercise a simulation tool"
grep -Fq 'FINANCE_MCP_SMOKE_TOKEN_FILE' "$openclaw_smoke" || fail "OpenClaw live smoke must read bearer from a file"
grep -Fq 'sha256sum' "$openclaw_smoke" || fail "OpenClaw live smoke must emit hashed evidence"
grep -Fq '401' "$openclaw_smoke" || fail "OpenClaw live smoke must verify unauthenticated rejection"
grep -Fq '403' "$openclaw_smoke" || fail "OpenClaw live smoke must verify Origin rejection"
grep -Fq -- '--code-mode direct' "$openclaw_smoke" || fail "OpenClaw live smoke must use direct tool mode"

echo "Production operations contract OK"
