# Append-only Off-site Backup Boundary Design

Date: 2026-08-22
Status: awaiting written-spec review
Runtime target before this change: `cbfba3268a9c747d874d84845910dca1f2c5657d`

## 1. Problem

The current production backup contract uses a restic SFTP repository from the production VPS and lets `scripts/backup.sh` run `restic forget --prune` with the same credentials used for backup. A production-host compromise can therefore use those credentials to delete off-site recovery points. That violates the production acceptance requirement that compromise of the production VPS must not be sufficient to destroy every off-site backup.

The current implementation also uses count-based retention (`--keep-daily`, `--keep-weekly`, `--keep-monthly`) from the producer. Restic's append-only guidance explicitly recommends moving destructive maintenance to a separate well-secured client and using `--keep-within` policies for append-only repositories.

## 2. Goals

1. The production VPS can create new encrypted off-site snapshots but cannot delete or overwrite existing repository data through its backup credential.
2. Destructive retention, prune, and full repository maintenance are controlled only from the independent backup/maintenance host.
3. Production backup remains simple: dump, archive, checksum, `restic backup`, local staging cleanup.
4. The off-site transport uses HTTPS and authenticated `rest-server --append-only` rather than plain SFTP semantics.
5. Real disaster recovery remains possible from an independent recovery environment.
6. The repository contains executable contracts that prevent future regressions back to production-side `forget`/`prune` or insecure transport.
7. Existing application containers and financial business logic are unchanged.

## 3. Non-goals

- No Kubernetes, Redis, queue, object-storage layer, MinIO, HA control plane, or backup microservice is added to the production application stack.
- No automatic provisioning of a cloud backup VM is added to Family Finance OS.
- No attempt is made to make the compromised production VPS unable to read its own historical encrypted backups; the security property is deletion resistance, not confidentiality from the production host that already owns the source financial data.
- No production release is declared complete by repository tests alone. The real append-only boundary and off-host restore must still be proven in production acceptance.

## 4. Selected architecture

### 4.1 Trust domains

There are two security domains:

**Production VPS**
- Runs the existing Family Finance OS Docker Compose stack.
- Owns live financial data.
- Owns the restic repository encryption password because it must create restic snapshots.
- Owns only an append-only REST credential for the off-site repository.
- Does not own any credential or filesystem access that can delete existing off-site repository objects.

**Backup / maintenance host**
- Is independent from the production VPS.
- Runs rest-server in append-only mode for the production credential.
- Owns the repository filesystem and full maintenance authority.
- Runs destructive retention/prune/check locally against the repository filesystem, without exposing a full-access network credential to the production VPS.
- Is the preferred recovery source for an off-host restore drill.

Data flow:

```text
Production VPS
  pg_dump + storage tar + SHA256
          |
          | restic backup
          | REST Basic Auth credential
          | HTTPS only
          v
Backup host
  rest-server --append-only
          |
          v
Encrypted restic repository
          ^
          |
          | local full-access maintenance only
          |
  backup-maintenance.sh
```

## 5. Backend contract

The V1 off-site backend becomes the restic REST backend:

```text
RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-os/
```

Production preflight must reject configured off-site repositories that are not `rest:https://...`.

The producer uses three secret/config inputs:

```text
RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password
RESTIC_REST_USERNAME=family-finance-os
RESTIC_REST_PASSWORD_FILE=/etc/family-finance/rest-server-password
```

`RESTIC_PASSWORD_FILE` is the restic repository encryption password file.

`RESTIC_REST_PASSWORD_FILE` is a Family Finance OS wrapper variable, not a native restic variable. The production backup script reads this file and exports the native `RESTIC_REST_PASSWORD` environment variable only for the restic process environment. `RESTIC_REST_USERNAME` is native restic configuration and may remain in `.env`; the password may not.

Both password files must:
- be regular files;
- be readable by the backup operator;
- have no group or world permissions;
- live outside the Git repository.

No REST password is placed in a repository URL, command-line argument, Git file, or evidence log.

## 6. Backup-host service contract

The reference V1 service is rest-server 0.14.0 or the exact later version explicitly selected and recorded during production acceptance. The required behavior is more important than packaging:

- authentication enabled;
- `--append-only` enabled;
- HTTPS used directly or through a TLS-terminating reverse proxy;
- repository data persisted on the independent backup host;
- the production credential cannot access a non-append-only endpoint for the same repository;
- the full-access repository filesystem is not mounted or exposed to the production VPS.

For a dedicated Family Finance OS repository, the recommended rest-server flags are equivalent to:

```text
rest-server --append-only --private-repos
```

With `RESTIC_REST_USERNAME=family-finance-os`, `--private-repos` therefore authorizes the repository path `/family-finance-os/`, matching the `RESTIC_REPOSITORY` example above. The final deployment guide must preserve that username/path relationship for the actual backup host.

## 7. Production backup flow

`scripts/backup.sh` keeps the existing local preparation behavior:

1. load the production environment;
2. create a UTC timestamp directory under the configured local backup root;
3. `pg_dump -Fc` both Finance Core and ezBookkeeping databases;
4. validate each dump with `pg_restore --list`;
5. archive ezBookkeeping storage;
6. create `SHA256SUMS`;
7. if off-site backup is configured, authenticate to the HTTPS REST backend and run `restic backup`;
8. keep the current bounded local staging-retention cleanup.

The production script must not execute:
- `restic forget`;
- `restic prune`;
- any direct repository object deletion;
- any alternate SFTP/rsync destructive retention path.

A successful `restic backup` exit is the producer's remote-write success criterion. Full repository integrity checks and destructive retention are maintenance-host responsibilities.

## 8. Maintenance flow

A new `scripts/backup-maintenance.sh` is intended for the independent backup/maintenance host only.

It requires:

```text
RESTIC_MAINTENANCE_REPOSITORY=/srv/restic/family-finance-os
RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password
BACKUP_KEEP_WITHIN=2y
```

Safety rules:

- `RESTIC_MAINTENANCE_REPOSITORY` must resolve to an absolute local filesystem path.
- The maintenance script rejects `rest:`, `sftp:`, `rclone:`, cloud backend URLs, and other network repository forms.
- It never consumes the producer's `RESTIC_REST_USERNAME` or REST password.
- It validates the repository encryption password file using the same private-file rules.

Maintenance sequence:

```text
restic snapshots
restic forget --keep-within "$BACKUP_KEEP_WITHIN" --prune
restic check
```

The default retention is `2y`. This deliberately keeps every snapshot in the protected window instead of down-sampling with count-based daily/weekly/monthly policies. The value may be changed by the backup administrator, but the repository contract allows only `--keep-within` for the destructive policy in this V1 append-only design.

The operator must inspect abnormal snapshot growth or suspicious timestamps before allowing destructive maintenance to continue. Real production scheduling should therefore prefer maintenance after a backup-health review rather than blindly sharing production credentials or executing prune from the producer.

## 9. Migration from the current SFTP model

The transition is intentionally explicit; there is no transparent in-place switch of credentials.

1. On the independent backup host, initialize the new restic repository locally at `/srv/restic/family-finance-os` using the intended repository encryption password.
2. Serve that existing repository through the authenticated HTTPS rest-server append-only endpoint with the `family-finance-os` private-repository user/path relationship.
3. Configure the production VPS with the new `rest:https://.../family-finance-os/` repository, REST username, and REST password file.
4. Run production preflight.
5. Create a new production backup and confirm a snapshot appears.
6. From the production credential, prove destructive operations are denied. The production acceptance evidence records only command result/status and sanitized snapshot identifiers.
7. From the maintenance host, run `restic check` and a non-destructive `forget --keep-within 2y --dry-run` before the first real retention run.
8. Perform an off-host restore from the new REST repository.
9. Only after the new boundary and restore are proven may the old SFTP repository be retired according to its own retention policy.

No existing SFTP backup is deleted as part of the repository code change.

## 10. Preflight changes

`scripts/preflight.sh` will enforce:

- deprecated `BACKUP_REMOTE` remains rejected;
- configured off-site backup must use `rest:https://`;
- `RESTIC_PASSWORD_FILE` is required and private;
- `RESTIC_REST_USERNAME` is required;
- `RESTIC_REST_PASSWORD_FILE` is required and private;
- both secret files live outside the repository;
- legacy SFTP repositories are rejected for the V1 production contract;
- legacy producer retention variables are rejected when present, so old deployment configuration does not silently retain destructive semantics.

Legacy variables to reject after migration:

```text
BACKUP_KEEP_DAILY
BACKUP_KEEP_WEEKLY
BACKUP_KEEP_MONTHLY
```

`BACKUP_RETENTION_DAYS` remains valid because it controls only local staging directories on the already-compromisable production VPS and has no bearing on off-site deletion resistance.

## 11. Repository changes

The implementation PR is expected to change only the backup/configuration surface:

- `.env.example`
- `scripts/preflight.sh`
- `scripts/backup.sh`
- new `scripts/backup-maintenance.sh`
- `scripts/test-production-ops.sh`
- optionally a focused CI contract script if keeping the assertions isolated improves readability
- `docs/07-operations.md` only as much as required to stop documenting the unsafe SFTP/prune production path
- `docs/acceptance/v1-production-evidence.md` only if the contract wording needs alignment; no real gate is promoted to PASS

A later documentation-only PR will add the full delivery layer:

- `docs/DEPLOYMENT.md`
- `docs/USER_GUIDE.md`
- `docs/ADMIN_GUIDE.md`
- `docs/BACKUP_RESTORE.md`
- `docs/TROUBLESHOOTING.md`

## 12. TDD / verification strategy

Implementation follows regression-first TDD.

### RED contract

Before production code is changed, add tests that fail against the current SFTP design and assert:

1. production `backup.sh` contains no `restic forget` or `restic prune`;
2. producer repository contract requires `rest:https://`;
3. preflight requires a REST username and external private REST password file;
4. producer configuration no longer accepts SFTP as the production off-site contract;
5. destructive maintenance is implemented only in `backup-maintenance.sh`;
6. maintenance accepts only a local repository path;
7. maintenance retention uses `--keep-within` and does not use `--keep-daily`, `--keep-weekly`, `--keep-monthly`, `--keep-last`, or their count-based equivalents.

The initial CI run must fail specifically on these new expectations before implementation is added.

### GREEN verification

After implementation:

- shell syntax passes;
- production-ops contract passes;
- repository-native `make verify` passes;
- MCP Security and Edge Security remain green;
- OpenClaw release acceptance remains green;
- diff review confirms no unrelated business/domain code change.

Because this changes production configuration/backup contracts, it is runtime-target-affecting. After merge, production acceptance selects the merge commit as the new runtime target and reruns applicable gates.

## 13. Real production acceptance

Repository tests prove the intended contract, not the external service configuration. M-03 is considered fully closed only when real evidence shows:

1. the production VPS successfully creates a snapshot through the HTTPS append-only REST endpoint;
2. the production credential cannot delete or overwrite existing snapshots;
3. the maintenance host can perform authorized `forget --keep-within ...`, prune, and check;
4. `restic check` succeeds from the maintenance/recovery authority;
5. a real snapshot is restored on a separate host/environment;
6. restored ezBookkeeping and Finance Core become healthy/ready;
7. sampled restored data matches using counts, ranges, and hashes only;
8. actual RTO is recorded;
9. evidence contains no raw statements, account numbers, passwords, tokens, or repository encryption secrets.

Until those items are executed, production release remains BLOCKED.

## 14. Rollback

If the new REST endpoint is unavailable during migration:

- do not revert production to a destructive SFTP credential automatically;
- keep local staging backups running if safe to do so;
- retain the old SFTP repository untouched as a historical recovery source;
- repair the append-only endpoint and rerun preflight/backup acceptance before declaring off-site protection restored.

If an implementation defect is found before production migration, revert the implementation PR and keep the production release BLOCKED. No data migration is performed by CI or by merging this repository change.

## 15. Security rationale and references

The design follows current upstream behavior:

- rest-server `--append-only` permits creation of new backups while preventing deletion/modification of existing repository data.
- Restic recommends a separate well-secured client for `forget`, `prune`, and other full-access maintenance on append-only repositories.
- Restic specifically recommends `--keep-within` for append-only retention because a compromised producer can inject snapshots with manipulated timestamps that make count-based retention select attacker snapshots over legitimate ones.
- Restic supports `RESTIC_REST_USERNAME` and `RESTIC_REST_PASSWORD` environment variables for REST backend authentication.

References:

- https://github.com/restic/rest-server/blob/master/README.md
- https://restic.readthedocs.io/en/latest/060_forget.html
- https://restic.readthedocs.io/en/latest/075_scripting.html
- https://restic.readthedocs.io/en/stable/030_preparing_a_new_repo.html
