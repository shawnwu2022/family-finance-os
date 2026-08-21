#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "production supply-chain contract failed: $*" >&2
  exit 1
}

check_dockerfile() {
  local file="$1"
  while IFS= read -r line; do
    [[ "$line" =~ ^FROM[[:space:]]+([^[:space:]]+) ]] || continue
    image="${BASH_REMATCH[1]}"
    [[ "$image" =~ @sha256:[0-9a-f]{64}$ ]] || fail "$file contains mutable FROM reference: $image"
  done <"$file"
}

check_compose() {
  local file="$1"
  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]+image:[[:space:]]+([^[:space:]#]+) ]] || continue
    image="${BASH_REMATCH[1]}"
    [[ "$image" =~ @sha256:[0-9a-f]{64}$ ]] || fail "$file contains mutable image reference: $image"
  done <"$file"
}

check_dockerfile Dockerfile
check_dockerfile ci/go.Dockerfile
check_compose compose.yaml
check_compose compose.ci.yaml

grep -Fq 'npm audit --audit-level=high' scripts/ci/web-verify.sh \
  || fail "web verifier must reject high/critical npm vulnerabilities"

echo "Production supply-chain contract OK"
