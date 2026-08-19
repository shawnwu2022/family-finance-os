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

if grep -REq 'actions/setup-(go|node)|go test|npm (ci|test|run)|^[[:space:]]+services:[[:space:]]*$' .github/workflows; then
  fail "verification logic leaked back into GitHub Actions"
fi

for workflow in .github/workflows/ci.yml .github/workflows/mcp-security.yml .github/workflows/edge-security.yml; do
  grep -Eq '^[[:space:]]*pull_request:' "$workflow" || fail "$workflow must run automatically for pull requests"
  grep -Eq '^[[:space:]]*push:' "$workflow" || fail "$workflow must run automatically for main pushes"
  grep -Eq '^[[:space:]]*workflow_dispatch:' "$workflow" || fail "$workflow must support manual workflow_dispatch"
done

for phase in \
  'scripts/ci/contract-test.sh' \
  'scripts/ci/go-stack-verify.sh' \
  'scripts/ci/mcp-security.sh' \
  'scripts/ci/web-verify.sh' \
  'scripts/ci/edge-security.sh' \
  'scripts/ci/container-verify.sh'; do
  grep -Fq "$phase" scripts/ci/verify.sh || fail "top-level verifier does not include phase: $phase"
done

grep -Fq 'goose -dir /src/db/migrations' scripts/ci/go-stack-verify.sh || fail "Go stack verifier must run migrations before integration tests"
grep -Fq 'scripts/ci/restore-verify.sh' scripts/ci/go-stack-verify.sh || fail "Go stack verifier must include the backup/restore drill"
grep -Fq 'scripts/test-production-ops.sh' scripts/ci/container-verify.sh || fail "container verifier must preserve the production operations contract"

echo "Repository-native CI contract OK"
