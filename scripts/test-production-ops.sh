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

[[ -f "$backup" ]] || fail "backup script is missing"
grep -Fq 'pg_dump' "$backup" || fail "backup must invoke pg_dump"
grep -Fq -- '-Fc' "$backup" || fail "backup must use PostgreSQL custom format"
grep -Fq 'ezbookkeeping-storage.tar.gz' "$backup" || fail "backup must archive ezBookkeeping storage"
grep -Fq 'RESTIC_REPOSITORY' "$backup" || fail "backup must support a restic repository"
grep -Fq 'RESTIC_PASSWORD_FILE' "$backup" || fail "restic automation must use a password file"
grep -Eq 'restic[[:space:]]+backup' "$backup" || fail "backup must send the snapshot through restic"
if grep -Eq '(^|[[:space:]])rsync([[:space:]]|$)' "$backup"; then
  fail "raw rsync is not an encrypted off-site backup path"
fi

[[ -f "$restore" ]] || fail "restore drill script is missing"
grep -Fq 'sha256sum -c' "$restore" || fail "restore drill must verify backup checksums"
grep -Fq 'pg_restore' "$restore" || fail "restore drill must exercise pg_restore"
grep -Fq 'finance_restore_drill' "$restore" || fail "restore drill must use isolated scratch databases"

[[ -f "$preflight" ]] || fail "preflight script is missing"
grep -Fq 'RESTIC_REPOSITORY' "$preflight" || fail "preflight must validate restic repository configuration"
grep -Fq 'RESTIC_PASSWORD_FILE' "$preflight" || fail "preflight must validate restic password file"
grep -Fq 'must live outside the repository' "$preflight" || fail "preflight must reject repository-local restic password files"

[[ -f "$edge" ]] || fail "edge security script is missing"
grep -Fq 'host port exposure' "$edge" || fail "edge security must enforce host-port exposure"
grep -Fq 'network_mode' "$edge" || fail "edge security must reject host networking"

echo "Production operations contract OK"
