#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "OpenClaw compose container refresh contract failed: $*" >&2
  exit 1
}

provisioner="scripts/acceptance/openclaw-ephemeral-release-acceptance.sh"
[[ -f "$provisioner" ]] || fail "provisioner is missing"
bash -n "$provisioner" || fail "provisioner shell syntax is invalid"

grep -Fq 'resolve_postgres_cid()' "$provisioner" \
  || fail "provisioner must resolve the current PostgreSQL container dynamically"
grep -Fq '"${compose[@]}" ps -q postgres' "$provisioner" \
  || fail "PostgreSQL resolver must use the active Compose project"

diagnostic_block="$(sed -n '/^emit_agent_audit_diagnostics()/,/^}/p' "$provisioner")"
query_block="$(sed -n '/^query_audit_count()/,/^}/p' "$provisioner")"

[[ "$diagnostic_block" == *'resolve_postgres_cid'* ]] \
  || fail "agent audit diagnostics must refresh the PostgreSQL container id"
[[ "$query_block" == *'resolve_postgres_cid'* ]] \
  || fail "final audit verification must refresh the PostgreSQL container id"

if grep -Fq '"$postgres_cid"' <<<"$diagnostic_block"; then
  fail "agent audit diagnostics must not use the startup PostgreSQL container id"
fi
if grep -Fq '"$postgres_cid"' <<<"$query_block"; then
  fail "final audit verification must not use the startup PostgreSQL container id"
fi

grep -Fq 'unavailable=no-postgres-container' <<<"$diagnostic_block" \
  || fail "diagnostics must degrade safely if the current PostgreSQL container cannot be resolved"
grep -Fq 'current PostgreSQL container id is unavailable' <<<"$query_block" \
  || fail "final audit verification must fail closed if PostgreSQL cannot be resolved"

echo "OpenClaw compose container refresh contract OK"
