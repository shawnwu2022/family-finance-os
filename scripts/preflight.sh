#!/usr/bin/env bash
set -euo pipefail

for cmd in docker openssl; do
  command -v "$cmd" >/dev/null || { echo "Missing command: $cmd" >&2; exit 1; }
done

docker compose version >/dev/null

if [[ ! -f .env ]]; then
  echo "Create .env from .env.example before deployment." >&2
  exit 1
fi

if grep -Eq 'REPLACE_WITH|example\.com' .env; then
  echo "ERROR: .env still contains deployment placeholders." >&2
  exit 1
fi

for key in FINANCE_AUTH_USER FINANCE_AUTH_HASH; do
  if ! grep -Eq "^${key}=[^[:space:]]+" .env; then
    echo "ERROR: ${key} must be set for the public Finance Core edge." >&2
    exit 1
  fi
done

docker compose config >/dev/null

echo "Preflight OK"
