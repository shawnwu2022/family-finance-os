#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

fail() {
  echo "repository-native CI contract failed: $*" >&2
  exit 1
}

required=(
  ci/go.Dockerfile
  compose.ci.yaml
  scripts/ci/verify.sh
  scripts/ci/go-stack-verify.sh
  scripts/ci/go-verify.sh
  scripts/ci/web-verify.sh
  scripts/ci/mcp-security.sh
  scripts/ci/edge-security.sh
  scripts/ci/restore-verify.sh
  scripts/ci/container-verify.sh
  scripts/ci/test-preflight-secret-permissions.sh
  scripts/ci/test-workflow-action-pins.sh
  scripts/ci/test-workflow-action-pins-regression.sh
)
for path in "${required[@]}"; do
  [[ -f "$path" ]] || fail "missing repository-native CI file: $path"
done

while IFS= read -r script; do
  bash -n "$script" || fail "shell syntax error: $script"
done < <(find scripts/ci -type f -name '*.sh' -print | sort)

for target in verify verify-contract verify-go verify-web verify-mcp-security verify-edge-security verify-container; do
  grep -Eq "^${target}:" Makefile || fail "Makefile target is missing: ${target}"
done

grep -Fq 'bash scripts/ci/go-stack-verify.sh' Makefile || fail "verify-go must delegate to the standalone Go stack verifier"
grep -Fq 'bash scripts/ci/container-verify.sh' Makefile || fail "verify-container must include production operations verification"

grep -Fq 'make verify' .github/workflows/ci.yml || fail "CI workflow must delegate to make verify"
grep -Fq 'make verify-mcp-security' .github/workflows/mcp-security.yml || fail "MCP Security workflow must delegate to make verify-mcp-security"
grep -Fq 'make verify-edge-security' .github/workflows/edge-security.yml || fail "Edge Security workflow must delegate to make verify-edge-security"

canonical_workflows=(
  .github/workflows/ci.yml
  .github/workflows/mcp-security.yml
  .github/workflows/edge-security.yml
)
if grep -Eq 'actions/setup-(go|node)|go test|npm (ci|test|run)|^[[:space:]]+services:[[:space:]]*$' "${canonical_workflows[@]}"; then
  fail "verification logic leaked back into canonical GitHub Actions workflows"
fi

for workflow in "${canonical_workflows[@]}"; do
  grep -Eq '^[[:space:]]*pull_request:' "$workflow" || fail "$workflow must run automatically for pull requests"
  grep -Eq '^[[:space:]]*push:' "$workflow" || fail "$workflow must run automatically for main pushes"
  grep -Eq '^[[:space:]]*workflow_dispatch:' "$workflow" || fail "$workflow must support manual workflow_dispatch"
done

for target in verify-go verify-mcp-security verify-web verify-edge-security verify-container; do
  grep -Fq "make ${target}" scripts/ci/verify.sh || fail "top-level verifier must delegate to make ${target}"
done
if grep -Eq 'bash scripts/ci/(go-stack-verify|mcp-security|web-verify|edge-security|container-verify)\.sh' scripts/ci/verify.sh; then
  fail "top-level verifier must not bypass canonical Make targets"
fi

grep -Fq 'goose -dir /src/db/migrations' scripts/ci/go-stack-verify.sh || fail "Go stack verifier must run migrations before integration tests"
grep -Fq 'scripts/ci/restore-verify.sh' scripts/ci/go-stack-verify.sh || fail "Go stack verifier must include the backup/restore drill"
grep -Fq 'scripts/test-production-ops.sh' scripts/ci/container-verify.sh || fail "container verifier must preserve the production operations contract"

if grep -Fq 'git diff' scripts/ci/go-verify.sh || grep -Fq 'git ls-files' scripts/ci/go-verify.sh; then
  fail "Go verifier temp workspace must not depend on copied Git metadata"
fi
grep -Fq 'diff -ru "$SOURCE_ROOT/internal/store/sqlc" "$WORK_ROOT/internal/store/sqlc"' scripts/ci/go-verify.sh || fail "Go verifier must compare generated sqlc sources against the read-only checkout"
grep -Fq 'cmp -s "$SOURCE_ROOT/go.mod" "$WORK_ROOT/go.mod"' scripts/ci/go-verify.sh || fail "Go verifier must compare go.mod against the read-only checkout"
grep -Fq 'cmp -s "$SOURCE_ROOT/go.sum" "$WORK_ROOT/go.sum"' scripts/ci/go-verify.sh || fail "Go verifier must compare go.sum against the read-only checkout"
grep -Fq 'go build -buildvcs=false -trimpath' scripts/ci/go-verify.sh || fail "Go verifier temp build must disable VCS stamping"

if grep -Fq '.git' scripts/ci/mcp-security.sh; then
  fail "MCP security verifier must not require Git metadata"
fi
grep -Fq '[[ -f "$SOURCE_ROOT/go.mod" ]]' scripts/ci/mcp-security.sh || fail "MCP security verifier must validate the mounted source tree by go.mod"

bash scripts/ci/test-preflight-secret-permissions.sh
bash scripts/ci/test-workflow-action-pins.sh
bash scripts/ci/test-workflow-action-pins-regression.sh

echo "Repository-native CI contract OK"
