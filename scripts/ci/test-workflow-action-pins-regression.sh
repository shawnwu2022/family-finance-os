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

cat >"$workdir/.github/workflows/test.yml" <<'EOF_DECOY'
name: insecure-checkout-decoy-fixture
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
EOF_DECOY

if run_checker >"$workdir/decoy.out" 2>&1; then
  fail "checker accepted checkout without step-local persist-credentials:false because another action supplied a decoy value"
fi

cat >"$workdir/.github/workflows/test.yml" <<'EOF_NAMED'
name: insecure-named-step-fixture
on:
  workflow_dispatch:
jobs:
  verify:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
        with:
          persist-credentials: false
      - name: Mutable second checkout must not be ignored
        uses: actions/checkout@v4
EOF_NAMED

if run_checker >"$workdir/named.out" 2>&1; then
  fail "checker ignored a mutable remote action when uses: was a property of a named step"
fi

cat >"$workdir/.github/workflows/test.yml" <<'EOF_SECURE'
name: secure-checkout-fixture
on:
  workflow_dispatch:
jobs:
  verify:
    runs-on: ubuntu-24.04
    steps:
      - name: Checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
        with:
          persist-credentials: false
      - name: Set up Node
        uses: actions/setup-node@820762786026740c76f36085b0efc47a31fe5020
        with:
          node-version: '24.19.0'
EOF_SECURE

run_checker >"$workdir/secure.out" 2>&1 || fail "checker rejected secure named action steps"

echo "Workflow action pin parser regressions OK"
