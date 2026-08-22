# Append-only Backup Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the production-side destructive restic/SFTP backup contract with an HTTPS append-only REST producer boundary and move destructive retention/prune/check to an independent backup-maintenance host.

**Architecture:** The production VPS keeps preparing PostgreSQL dumps, ezBookkeeping storage archives, checksums, and `restic backup`, but authenticates only to `rest:https://...` using a dedicated append-only REST credential. A separate maintenance host owns the repository filesystem and runs `forget --keep-within ... --prune` plus `check` locally; production never receives that authority.

**Tech Stack:** Bash, restic REST backend, rest-server append-only mode, Docker Compose, repository-native shell contracts, GitHub Actions exact-head verification.

**Spec:** `docs/superpowers/specs/2026-08-22-append-only-backup-boundary-design.md`

## Global Constraints

- Production VPS may create new encrypted off-site snapshots but must not be able to delete or overwrite existing off-site repository data with its producer credential.
- Configured production off-site repositories must use `rest:https://...`; legacy SFTP producer repositories are rejected by preflight.
- Production `scripts/backup.sh` must not invoke `restic forget`, `restic prune`, `restic check`, or direct remote repository deletion.
- Destructive repository maintenance runs only from `scripts/backup-maintenance.sh` against an absolute local filesystem repository path on the backup/maintenance host.
- Maintenance retention uses `--keep-within`; count-based retention flags (`--keep-daily`, `--keep-weekly`, `--keep-monthly`, `--keep-last`) are not part of the V1 append-only contract.
- `RESTIC_PASSWORD_FILE` and `RESTIC_REST_PASSWORD_FILE` must be private regular files outside the Git repository with no group/world permissions.
- The REST authentication password is never stored in `.env`, a repository URL, a command-line argument, Git, or acceptance evidence.
- `BACKUP_RETENTION_DAYS` remains local-staging-only and does not grant remote destructive authority.
- No Kubernetes, Redis, queue, object-storage layer, MinIO, HA subsystem, new application service, new application host port, finance-domain behavior, or database schema is introduced.
- Repository tests do not close the real production DR gate; production release remains BLOCKED until the append-only endpoint, denied destructive producer action, maintenance authority, off-host restore, and RTO are proven in the real environment.

---

## File Map

- Create `scripts/ci/test-backup-append-only-contract.sh`: focused static regression contract for producer/maintenance separation.
- Modify `scripts/test-production-ops.sh`: execute the focused backup-security contract as part of the canonical production-operations gate.
- Modify `.env.example`: document the HTTPS REST producer contract and remove producer-side count retention variables.
- Modify `scripts/preflight.sh`: validate HTTPS REST repository, producer REST identity, external private secret files, and reject legacy SFTP/count-retention configuration.
- Modify `scripts/backup.sh`: keep local backup preparation + append-only `restic backup`; remove remote destructive maintenance/check.
- Create `scripts/backup-maintenance.sh`: local-filesystem-only full-authority retention/prune/check for the backup host.
- Modify `docs/07-operations.md`: replace the obsolete SFTP/producer-prune procedure with the append-only producer + separate maintenance-host procedure.
- Modify `docs/acceptance/v1-production-evidence.md` only if needed to align wording with the actual executable contract; every real-world gate remains `NOT RUN`.
- Create post-merge evidence-only update after the implementation merge commit becomes the new runtime target.

---

### Task 1: Add the append-only backup regression contract (RED)

**Files:**
- Create: `scripts/ci/test-backup-append-only-contract.sh`
- Modify: `scripts/test-production-ops.sh`

**Interfaces:**
- Consumes: current `scripts/backup.sh`, `scripts/preflight.sh`, `.env.example`.
- Produces: executable static contract invoked by `scripts/test-production-ops.sh`; later tasks must make it green without weakening assertions.

- [ ] **Step 1: Add a failing focused contract**

Create `scripts/ci/test-backup-append-only-contract.sh` with this behavior:

```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "append-only backup contract failed: $*" >&2
  exit 1
}

backup="scripts/backup.sh"
preflight="scripts/preflight.sh"
maintenance="scripts/backup-maintenance.sh"
env_example=".env.example"

for file in "$backup" "$preflight" "$env_example"; do
  [[ -f "$file" ]] || fail "required file is missing: $file"
done

if grep -Eq 'restic[[:space:]]+forget|restic[[:space:]]+prune|restic[[:space:]]+check' "$backup"; then
  fail "production backup must not have destructive/full-maintenance restic authority"
fi

grep -Fq 'rest:https://' "$preflight" || fail "preflight must require HTTPS REST backup transport"
grep -Fq 'RESTIC_REST_USERNAME' "$preflight" || fail "preflight must require the REST producer username"
grep -Fq 'RESTIC_REST_PASSWORD_FILE' "$preflight" || fail "preflight must require an external REST password file"
grep -Fq 'RESTIC_REST_USERNAME' "$env_example" || fail ".env.example must document the REST producer username"
grep -Fq 'RESTIC_REST_PASSWORD_FILE' "$env_example" || fail ".env.example must document the REST producer password file"

for legacy in BACKUP_KEEP_DAILY BACKUP_KEEP_WEEKLY BACKUP_KEEP_MONTHLY; do
  if grep -Eq "^${legacy}=" "$env_example"; then
    fail ".env.example must not advertise producer-side count retention: $legacy"
  fi
  grep -Fq "$legacy" "$preflight" || fail "preflight must explicitly reject legacy producer retention variable $legacy"
done

[[ -f "$maintenance" ]] || fail "backup-maintenance.sh must exist"
bash -n "$maintenance" || fail "backup-maintenance.sh syntax is invalid"
grep -Fq 'RESTIC_MAINTENANCE_REPOSITORY' "$maintenance" || fail "maintenance must use a dedicated local repository variable"
grep -Fq -- '--keep-within' "$maintenance" || fail "maintenance retention must use --keep-within"
grep -Fq -- '--prune' "$maintenance" || fail "maintenance host must own prune"
grep -Eq 'restic[[:space:]]+check' "$maintenance" || fail "maintenance host must own restic check"

for forbidden in --keep-daily --keep-weekly --keep-monthly --keep-last; do
  if grep -Fq -- "$forbidden" "$maintenance"; then
    fail "count-based append-only retention is forbidden: $forbidden"
  fi
done

if grep -Fq 'RESTIC_REST_PASSWORD_FILE' "$maintenance" || grep -Fq 'RESTIC_REST_USERNAME' "$maintenance"; then
  fail "maintenance must not consume the producer REST credential"
fi

echo "Append-only backup contract OK"
```

- [ ] **Step 2: Wire the contract into the canonical production-operations test**

In `scripts/test-production-ops.sh`, add:

```bash
backup_append_only_contract="scripts/ci/test-backup-append-only-contract.sh"
```

Include it in the required-script syntax loop, then run it before the final success line:

```bash
bash "$backup_append_only_contract"
```

- [ ] **Step 3: Run the focused contract and verify RED**

Run:

```bash
bash scripts/ci/test-backup-append-only-contract.sh
```

Expected: FAIL against the current implementation because `scripts/backup.sh` still contains `restic forget ... --prune` / `restic check`, preflight still requires SFTP, `.env.example` still advertises count retention, and `scripts/backup-maintenance.sh` does not exist.

- [ ] **Step 4: Run canonical production-ops and verify the same intentional RED**

Run:

```bash
bash scripts/test-production-ops.sh
```

Expected: FAIL only because the new append-only backup contract is not implemented; pre-existing production-operation assertions before the new contract remain green.

- [ ] **Step 5: Commit the RED test only**

```bash
git add -- scripts/ci/test-backup-append-only-contract.sh scripts/test-production-ops.sh
git commit -m "test: require append-only backup authority boundary"
```

Do not modify producer/maintenance implementation in this commit.

---

### Task 2: Make production producer config/preflight append-only-safe (GREEN part 1)

**Files:**
- Modify: `.env.example`
- Modify: `scripts/preflight.sh`
- Modify: `scripts/backup.sh`

**Interfaces:**
- Consumes: Task 1 regression assertions.
- Produces: production configuration contract `RESTIC_REPOSITORY=rest:https://...`, `RESTIC_REST_USERNAME`, `RESTIC_REST_PASSWORD_FILE`; producer has backup-only remote behavior.

- [ ] **Step 1: Replace the off-site variables in `.env.example`**

Replace the SFTP/count-retention section with:

```env
# Optional encrypted off-site backup. V1 production uses the restic REST backend over HTTPS
# against a rest-server endpoint configured append-only for this producer identity.
# With rest-server --private-repos, the first repository path component matches the username.
RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-prod/
# Restic repository encryption password. Absolute path outside this repository.
RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password
# Producer identity for the append-only REST endpoint. Not a secret by itself.
RESTIC_REST_USERNAME=family-finance-prod
# Family Finance OS wrapper variable. Absolute private file outside this repository;
# backup.sh reads it and passes native RESTIC_REST_PASSWORD only to restic child processes.
RESTIC_REST_PASSWORD_FILE=/etc/family-finance/rest-server-password
# Local staging backups are retained independently from off-site maintenance retention.
BACKUP_RETENTION_DAYS=14
```

Do not retain `BACKUP_KEEP_DAILY`, `BACKUP_KEEP_WEEKLY`, or `BACKUP_KEEP_MONTHLY` in `.env.example`.

- [ ] **Step 2: Extend `scripts/preflight.sh` with producer secret validation**

Keep `require_private_file`. Replace the SFTP-specific restic block with logic equivalent to:

```bash
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
  [[ -n "${RESTIC_PASSWORD_FILE:-}" ]] || { echo "ERROR: RESTIC_PASSWORD_FILE is required with RESTIC_REPOSITORY." >&2; exit 1; }
  [[ -n "${RESTIC_REST_USERNAME:-}" ]] || { echo "ERROR: RESTIC_REST_USERNAME is required with RESTIC_REPOSITORY." >&2; exit 1; }
  [[ -n "${RESTIC_REST_PASSWORD_FILE:-}" ]] || { echo "ERROR: RESTIC_REST_PASSWORD_FILE is required with RESTIC_REPOSITORY." >&2; exit 1; }

  require_private_file "$RESTIC_PASSWORD_FILE" "RESTIC_PASSWORD_FILE"
  require_private_file "$RESTIC_REST_PASSWORD_FILE" "RESTIC_REST_PASSWORD_FILE"

  for secret_file in "$RESTIC_PASSWORD_FILE" "$RESTIC_REST_PASSWORD_FILE"; do
    secret_path="$(cd "$(dirname "$secret_file")" && pwd -P)/$(basename "$secret_file")"
    case "$secret_path" in
      "$ROOT_DIR"/*)
        echo "ERROR: backup secret files must live outside the repository." >&2
        exit 1
        ;;
    esac
  done
fi
```

Do not accept `sftp:` as a production off-site repository after this change.

- [ ] **Step 3: Remove destructive restic operations from `scripts/backup.sh`**

Keep dump/archive/checksum and local staging cleanup. Replace the current restic block with a helper that scopes the REST password to each restic child process:

```bash
run_restic() {
  RESTIC_REST_PASSWORD="$(<"$RESTIC_REST_PASSWORD_FILE")" \
    RESTIC_REST_USERNAME="$RESTIC_REST_USERNAME" \
    RESTIC_REPOSITORY="$RESTIC_REPOSITORY" \
    RESTIC_PASSWORD_FILE="$RESTIC_PASSWORD_FILE" \
    restic "$@"
}

if [[ -n "${RESTIC_REPOSITORY:-}" ]]; then
  command -v restic >/dev/null 2>&1 || fail "restic is required when RESTIC_REPOSITORY is configured"
  [[ "$RESTIC_REPOSITORY" == rest:https://* ]] || fail "V1 off-site backup repository must use rest:https://"
  [[ -n "${RESTIC_PASSWORD_FILE:-}" ]] || fail "RESTIC_PASSWORD_FILE is required when RESTIC_REPOSITORY is configured"
  [[ -n "${RESTIC_REST_USERNAME:-}" ]] || fail "RESTIC_REST_USERNAME is required when RESTIC_REPOSITORY is configured"
  [[ -n "${RESTIC_REST_PASSWORD_FILE:-}" ]] || fail "RESTIC_REST_PASSWORD_FILE is required when RESTIC_REPOSITORY is configured"
  [[ -r "$RESTIC_PASSWORD_FILE" ]] || fail "RESTIC_PASSWORD_FILE is not readable"
  [[ -r "$RESTIC_REST_PASSWORD_FILE" ]] || fail "RESTIC_REST_PASSWORD_FILE is not readable"

  for secret_file in "$RESTIC_PASSWORD_FILE" "$RESTIC_REST_PASSWORD_FILE"; do
    secret_path="$(cd "$(dirname "$secret_file")" && pwd -P)/$(basename "$secret_file")"
    case "$secret_path" in
      "$ROOT_DIR"/*) fail "backup secret files must live outside the repository" ;;
    esac
  done

  run_restic snapshots --json >/dev/null
  run_restic backup "$DEST" --tag family-finance-os --tag "$STAMP"
fi
```

Delete the producer-side `restic forget ... --prune` and `restic check` calls entirely.

- [ ] **Step 4: Add direct shell tests for invalid producer configuration**

Run focused temporary environment-file probes against `scripts/preflight.sh` using a test harness or the existing preflight test pattern. At minimum prove these cases fail before Docker Compose execution:

```text
RESTIC_REPOSITORY=sftp:...
RESTIC_REPOSITORY=rest:http://...
RESTIC_REPOSITORY=rest:https://... with missing RESTIC_REST_USERNAME
RESTIC_REPOSITORY=rest:https://... with missing RESTIC_REST_PASSWORD_FILE
RESTIC_REST_PASSWORD_FILE inside repository
RESTIC_REST_PASSWORD_FILE mode 0644
BACKUP_KEEP_DAILY set
```

And prove the valid shape reaches the later Compose validation with:

```text
RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-prod/
RESTIC_REST_USERNAME=family-finance-prod
private external RESTIC_PASSWORD_FILE
private external RESTIC_REST_PASSWORD_FILE
```

- [ ] **Step 5: Run Task 1 contract**

Run:

```bash
bash scripts/ci/test-backup-append-only-contract.sh
```

Expected: still FAIL only because `scripts/backup-maintenance.sh` does not exist yet; producer-side destructive/SFTP/count-retention assertions are now green.

- [ ] **Step 6: Commit producer changes**

```bash
git add -- .env.example scripts/preflight.sh scripts/backup.sh
git commit -m "security: make production backups append-only producers"
```

---

### Task 3: Add backup-host-only destructive maintenance (GREEN part 2)

**Files:**
- Create: `scripts/backup-maintenance.sh`
- Test: `scripts/ci/test-backup-append-only-contract.sh`

**Interfaces:**
- Consumes: local encrypted restic repository directory and repository encryption password file.
- Produces: the only repository-owned V1 path that runs destructive retention/prune/check.

- [ ] **Step 1: Implement `scripts/backup-maintenance.sh`**

Create:

```bash
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
[[ "$REPOSITORY" == /* ]] || fail "RESTIC_MAINTENANCE_REPOSITORY must be an absolute local filesystem path"
[[ -d "$REPOSITORY" ]] || fail "RESTIC_MAINTENANCE_REPOSITORY must be an existing directory"
case "$REPOSITORY" in
  rest:*|sftp:*|rclone:*|http:*|https:*|*://*) fail "maintenance repository must be local, not a network backend" ;;
esac

[[ -n "$PASSWORD_FILE" ]] || fail "RESTIC_PASSWORD_FILE is required"
[[ -f "$PASSWORD_FILE" && -r "$PASSWORD_FILE" ]] || fail "RESTIC_PASSWORD_FILE must be a readable regular file"
mode="$(stat -Lc '%a' "$PASSWORD_FILE")" || fail "could not inspect RESTIC_PASSWORD_FILE mode"
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
```

Do not read `RESTIC_REST_USERNAME`, `RESTIC_REST_PASSWORD`, or `RESTIC_REST_PASSWORD_FILE` in this script.

- [ ] **Step 2: Verify shell syntax**

Run:

```bash
bash -n scripts/backup-maintenance.sh
```

Expected: PASS.

- [ ] **Step 3: Verify rejection of network repository shapes without touching a real repository**

Use a temporary private password file and run:

```bash
RESTIC_MAINTENANCE_REPOSITORY='rest:https://backup.example.com/family-finance-prod/' \
RESTIC_PASSWORD_FILE="$tmp_password" \
bash scripts/backup-maintenance.sh
```

Expected: non-zero with `maintenance repository must be local` before invoking restic maintenance.

Repeat for `sftp:host:/repo`.

- [ ] **Step 4: Run the focused contract**

Run:

```bash
bash scripts/ci/test-backup-append-only-contract.sh
```

Expected: PASS.

- [ ] **Step 5: Run canonical production-ops**

Run:

```bash
bash scripts/test-production-ops.sh
```

Expected: PASS.

- [ ] **Step 6: Commit maintenance host implementation**

```bash
git add -- scripts/backup-maintenance.sh
git commit -m "ops: separate destructive backup maintenance authority"
```

---

### Task 4: Align operations and acceptance documentation with the executable boundary

**Files:**
- Modify: `docs/07-operations.md`
- Modify only if wording is stale: `docs/acceptance/v1-production-evidence.md`

**Interfaces:**
- Consumes: executable producer/maintenance behavior from Tasks 2-3.
- Produces: deployment/operations instructions that no longer teach the unsafe SFTP + production prune model.

- [ ] **Step 1: Replace SFTP producer instructions in `docs/07-operations.md`**

Document these exact roles:

```text
Production VPS:
  RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-prod/
  RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password
  RESTIC_REST_USERNAME=family-finance-prod
  RESTIC_REST_PASSWORD_FILE=/etc/family-finance/rest-server-password
  ./scripts/backup.sh

Backup host:
  rest-server authentication enabled
  rest-server --append-only --private-repos
  repository directory created/initialized locally by backup administrator
  RESTIC_MAINTENANCE_REPOSITORY=/srv/restic/family-finance-prod
  RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password
  BACKUP_KEEP_WITHIN=2y
  ./scripts/backup-maintenance.sh
```

Explicitly state that the REST username and first private-repository path component match when `--private-repos` is used.

- [ ] **Step 2: Document initialization and migration sequence**

The runbook must state:

1. create the repository locally on the backup host with the intended restic encryption password;
2. expose only the append-only REST endpoint to production;
3. configure production producer secrets;
4. run `./scripts/preflight.sh`;
5. run `./scripts/backup.sh`;
6. prove producer destructive actions are denied;
7. run maintenance-host `restic check` and `forget --keep-within 2y --dry-run` before first real retention;
8. perform off-host restore;
9. keep the old SFTP repository untouched until the new boundary and restore are proven.

- [ ] **Step 3: Remove contradictory statements**

Delete/replace any statement that says:

```text
V1 requires an SFTP backup target
production backup.sh performs restic retention/prune/check
BACKUP_KEEP_DAILY/WEEKLY/MONTHLY are production variables
producer credentials may perform destructive repository maintenance
```

- [ ] **Step 4: Keep acceptance evidence honest**

If `docs/acceptance/v1-production-evidence.md` already says the off-site snapshot/append-only boundary is `NOT RUN`, preserve it. Only adjust terminology from SFTP to REST/append-only if stale wording remains. Do not mark any real backup/restore/immutability item PASS.

- [ ] **Step 5: Run documentation-sensitive contracts**

Run:

```bash
bash scripts/ci/test-backup-append-only-contract.sh
bash scripts/test-production-ops.sh
make verify-contract
```

Expected: PASS.

- [ ] **Step 6: Commit documentation alignment**

```bash
git add -- docs/07-operations.md docs/acceptance/v1-production-evidence.md
git commit -m "docs: align backup operations with append-only boundary"
```

If the evidence file required no change, stage only `docs/07-operations.md`.

---

### Task 5: Exact-head verification, review, and merge

**Files:**
- No new product files; this task validates the complete implementation diff.

**Interfaces:**
- Consumes: Tasks 1-4 complete implementation.
- Produces: one mergeable M-03 PR exact head with all required checks green and no unresolved review findings.

- [ ] **Step 1: Run local/repository-native focused verification**

Run:

```bash
bash scripts/ci/test-backup-append-only-contract.sh
bash scripts/test-production-ops.sh
make verify-contract
```

Expected: PASS.

- [ ] **Step 2: Run full canonical verification**

Run:

```bash
make verify
```

Expected: PASS, including Go, web, MCP security, edge security, production-ops, restore drill, and container build.

- [ ] **Step 3: Audit the final diff scope**

Confirm the final implementation diff is limited to:

```text
.env.example
scripts/preflight.sh
scripts/backup.sh
scripts/backup-maintenance.sh
scripts/ci/test-backup-append-only-contract.sh
scripts/test-production-ops.sh
docs/07-operations.md
optional wording-only docs/acceptance/v1-production-evidence.md
spec/plan documents
```

No finance-domain Go/SQL/frontend code, Docker/Compose service topology, Caddy host-port exposure, or dependency manifest should change.

- [ ] **Step 4: Open or update the Draft PR to `main`**

PR body must include:

```text
M-03 threat: production credentials could delete all off-site restic recovery points.
RED: exact test-only commit/run proving the current SFTP/producer-prune contract fails the new boundary.
GREEN: exact implementation head and focused/full verification runs.
Security property: production producer can append via HTTPS REST but destructive retention/prune/check remains on an independent local maintenance authority.
Real boundary: production release remains BLOCKED until an actual append-only endpoint, denied producer deletion, maintenance check, off-host restore, and RTO are proven.
```

Keep Draft until exact-head required workflows complete.

- [ ] **Step 5: Require exact-head GitHub gates**

For the final head, require:

```text
CI: success
MCP Security: success
Edge Security: success
OpenClaw Release Acceptance: success, including Real OpenClaw when the workflow runs it
```

If a required workflow fails for code/config reasons, diagnose and fix using a new RED/GREEN cycle. If a long-running real acceptance is still progressing, do not bypass it.

- [ ] **Step 6: Review all PR threads**

List inline review threads. For every valid finding:

1. add a failing regression first when behavior is affected;
2. implement the minimal fix;
3. rerun exact-head required checks;
4. reply with the fix/evidence;
5. resolve the thread only after green evidence.

Do not merge with an unresolved valid P0/P1/P2 security/data-correctness finding.

- [ ] **Step 7: Merge with expected-head guard**

Use merge-commit semantics and the exact final head SHA. Verify GitHub returns `merged=true`, capture the merge commit SHA, then verify `main` points to that merge commit.

- [ ] **Step 8: Delete the merged implementation branch**

After verifying `main`, remove the remote implementation branch and verify no stale branch remains.

---

### Task 6: Post-merge runtime-target/evidence convergence

**Files:**
- Modify: GitHub Issue #26 production acceptance tracker.
- Modify via evidence-only PR: `docs/acceptance/v1-production-evidence.md`.

**Interfaces:**
- Consumes: Task 5 M-03 merge commit and final exact-head workflow evidence.
- Produces: new frozen production runtime target and accurate BLOCKED real-environment acceptance ledger.

- [ ] **Step 1: Select the M-03 merge commit as the new runtime target**

Because backup/configuration contracts are runtime-affecting, replace the old runtime target `cbfba3268a9c747d874d84845910dca1f2c5657d` with the M-03 merge commit SHA.

Record separately:

```text
new main runtime target = <M-03 merge commit>
validated PR exact head = <final implementation head>
validated runtime tree = <tree SHA of merge payload>
```

- [ ] **Step 2: Update Issue #26 without claiming production deployment**

Mark repository-level M-03 implementation complete, but keep these real items unchecked:

```text
real production backup
append-only REST snapshot
producer destructive-operation denial proof
maintenance-host restic check
separate-host restore
restored health/readiness
sampled restored data
RTO
```

Production release remains BLOCKED.

- [ ] **Step 3: Create an evidence-only branch/PR**

Update `docs/acceptance/v1-production-evidence.md` to:

- select the new runtime target;
- record final implementation exact-head CI/MCP/Edge/OpenClaw evidence;
- describe the repository-level append-only contract as implemented;
- keep every real external backup/restore gate `NOT RUN`;
- retain the policy that public/third-party runtime-image CVE scanning itself is not a mandatory blocking gate, while known confirmed reachable high-severity findings remain blockers.

No code/config change belongs in this evidence PR.

- [ ] **Step 4: Verify evidence-only diff**

Confirm the PR changes only:

```text
docs/acceptance/v1-production-evidence.md
```

Required checks must pass on the exact metadata head under the existing Ruleset.

- [ ] **Step 5: Merge evidence-only PR and clean the branch**

Merge with expected-head guard after all required checks are green. Verify `main`, delete the evidence branch, and keep the runtime target frozen at the M-03 runtime merge commit because metadata-only evidence commits do not redefine the runtime target.

---

## Plan Self-Review

- Spec coverage: producer deletion resistance, HTTPS REST transport, separate maintenance authority, secret-file handling, local-only maintenance, `--keep-within`, migration/rollback, real acceptance, and post-merge runtime-target governance are all mapped to explicit tasks.
- Placeholder scan: no `TBD`, `TODO`, or unspecified implementation step remains.
- Interface consistency: producer variables are consistently `RESTIC_REPOSITORY`, `RESTIC_PASSWORD_FILE`, `RESTIC_REST_USERNAME`, `RESTIC_REST_PASSWORD_FILE`; maintenance variables are consistently `RESTIC_MAINTENANCE_REPOSITORY`, `RESTIC_PASSWORD_FILE`, `BACKUP_KEEP_WITHIN`.
- Scope: full delivery documentation (`DEPLOYMENT.md`, `USER_GUIDE.md`, `ADMIN_GUIDE.md`, `BACKUP_RESTORE.md`, `TROUBLESHOOTING.md`) remains a separate documentation-only PR after M-03 converges, as required by the approved design.
