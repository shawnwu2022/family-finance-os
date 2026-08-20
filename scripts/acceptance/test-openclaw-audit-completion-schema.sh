#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "OpenClaw audit completion contract failed: $*" >&2
  exit 1
}

provisioner="scripts/acceptance/openclaw-ephemeral-release-acceptance.sh"
[[ -f "$provisioner" ]] || fail "provisioner is missing"
bash -n "$provisioner" || fail "provisioner shell syntax is invalid"

grep -Fq "status = 'success'" "$provisioner" || fail "audit query must require success status"
grep -Fq 'output_sha256 IS NOT NULL' "$provisioner" || fail "audit query must require a persisted output hash"
grep -Fq 'duration_ms IS NOT NULL' "$provisioner" || fail "audit query must require persisted completion duration"
if grep -Fq 'completed_at' "$provisioner"; then
  fail "audit query must not reference nonexistent completed_at"
fi

echo "OpenClaw audit completion contract OK"
