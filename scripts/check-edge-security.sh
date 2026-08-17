#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "edge security contract failed: $*" >&2
  exit 1
}

finance_block="$({
  awk '
    /^\{\$FINANCE_DOMAIN\} \{/ { in_finance = 1 }
    in_finance { print }
    in_finance && /^}$/ { exit }
  ' Caddyfile
} || true)"

[[ -n "$finance_block" ]] || fail "FINANCE_DOMAIN site block is missing"
grep -Eq '^[[:space:]]*basic_auth([[:space:]]|$)' <<<"$finance_block" || fail "FINANCE_DOMAIN must enforce basic_auth"
grep -Fq '{$FINANCE_AUTH_USER}' <<<"$finance_block" || fail "FINANCE_DOMAIN auth username must come from FINANCE_AUTH_USER"
grep -Fq '{$FINANCE_AUTH_HASH}' <<<"$finance_block" || fail "FINANCE_DOMAIN auth hash must come from FINANCE_AUTH_HASH"

grep -Fq 'FINANCE_AUTH_USER: ${FINANCE_AUTH_USER:?' compose.yaml || fail "Compose must require FINANCE_AUTH_USER"
grep -Fq 'FINANCE_AUTH_HASH: ${FINANCE_AUTH_HASH:?' compose.yaml || fail "Compose must require FINANCE_AUTH_HASH"
grep -Fq 'FINANCE_AUTH_USER=' .env.example || fail ".env.example must document FINANCE_AUTH_USER"
grep -Fq 'FINANCE_AUTH_HASH=' .env.example || fail ".env.example must document FINANCE_AUTH_HASH"

caddy_service="$({
  awk '
    /^  caddy:/ { in_caddy = 1 }
    in_caddy { print }
    in_caddy && /^networks:/ { exit }
  ' compose.yaml
} || true)"
[[ -n "$caddy_service" ]] || fail "caddy service is missing"
if grep -Eq '^[[:space:]]+-[[:space:]]+finance-core[[:space:]]*$' <<<"$caddy_service"; then
  fail "caddy must not depend_on finance-core; ezBookkeeping must be reachable before EBK_API_TOKEN exists"
fi

echo "Finance edge security contract OK"
