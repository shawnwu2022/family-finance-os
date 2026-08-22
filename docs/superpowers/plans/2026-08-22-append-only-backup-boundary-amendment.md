# Append-only Backup Boundary Plan Amendment

This amendment is authoritative over Task 1 / Task 2 Step 4 of `2026-08-22-append-only-backup-boundary.md`.

## Reason

The repository already has a dynamic preflight harness at `scripts/ci/test-preflight-secret-permissions.sh`. The original plan incorrectly placed the new REST producer configuration probes after producer implementation. That violates regression-first TDD. The new preflight behavior must be specified and observed RED before modifying `.env.example`, `scripts/preflight.sh`, or `scripts/backup.sh`.

## Corrected Task 1 RED scope

The RED commit contains exactly:

- create `scripts/ci/test-backup-append-only-contract.sh`;
- modify `scripts/test-production-ops.sh` to invoke it;
- modify `scripts/ci/test-preflight-secret-permissions.sh` to encode the new producer configuration contract.

No production/configuration implementation file is changed in the RED commit.

### Dynamic preflight contract

Extend the existing `write_env` harness to support these fields:

```text
RESTIC_REPOSITORY
RESTIC_PASSWORD_FILE
RESTIC_REST_USERNAME
RESTIC_REST_PASSWORD_FILE
BACKUP_KEEP_DAILY (test-only legacy probe)
MCP_ENABLED
MCP_TOKEN_FILE
```

The test must assert that this configuration is accepted once implementation exists:

```text
RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-prod/
RESTIC_PASSWORD_FILE=<external private file>
RESTIC_REST_USERNAME=family-finance-prod
RESTIC_REST_PASSWORD_FILE=<external private file>
```

Both secret files use mode `0600` and live outside the temporary repository root.

The test must assert rejection of each of these cases:

```text
RESTIC_REPOSITORY=sftp:backup-host:/srv/restic/family-finance-os
RESTIC_REPOSITORY=rest:http://backup.example.com/family-finance-prod/
missing RESTIC_REST_USERNAME
missing RESTIC_REST_PASSWORD_FILE
RESTIC_PASSWORD_FILE mode 0644
RESTIC_REST_PASSWORD_FILE mode 0644
RESTIC_REST_PASSWORD_FILE inside repository
BACKUP_KEEP_DAILY=14
```

The existing `.env` and MCP secret-permission assertions remain intact.

## Required RED evidence

After the test-only commit, open a Draft PR so GitHub Actions run against the exact head. CI must fail because the current production preflight still accepts only `sftp:` and therefore rejects the newly specified valid `rest:https://...` producer configuration. The failure must be traceable to the new backup/preflight contract, not a syntax error or unrelated gate.

Only after this exact RED is observed may Task 2 modify `.env.example`, `scripts/preflight.sh`, or `scripts/backup.sh`.

## Corrected Task 2

Task 2 no longer adds tests after implementation. It implements only the minimum producer/configuration behavior required to make the Task 1 static and dynamic contracts advance toward GREEN. The focused contract is expected to remain RED solely because `scripts/backup-maintenance.sh` is still absent until Task 3.
