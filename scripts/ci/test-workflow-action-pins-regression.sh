#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "workflow action pin regression failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT
mkdir -p "$workdir/.github/workflows" "$workdir/scripts/ci"
cp "$ROOT_DIR/scripts/ci/test-workflow-action-pins.sh" "$workdir/scripts/ci/test-workflow-action-pins.sh"

run_checker() {
  (cd "$workdir" && bash scripts/ci/test-workflow-action-pins.sh)
}

cat >"$workdir/.github/workflows/test.yml" <<'EOF_INSECURE'
name: insecure-checkout-fixture
on:
  workflow_dispatch:
jobs:
  verify:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
        with:
          node-version: '24.19.0'
          persist-credentials: false
EOF_INSECURE

if run_checker >"$workdir/insecure.out" 2>&1; then
  fail "checker accepted checkout without step-local persist-credentials:false because another action supplied a decoy value"
fi

cat >"$workdir/.github/workflows/test.yml" <<'EOF_SECURE'
name: secure-checkout-fixture
on:
  workflow_dispatch:
jobs:
  verify:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
        with:
          persist-credentials: false
      - uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
        with:
          node-version: '24.19.0'
EOF_SECURE

run_checker >"$workdir/secure.out" 2>&1 || fail "checker rejected a checkout step with its own persist-credentials:false"

echo "Workflow action pin step-binding regression OK"
