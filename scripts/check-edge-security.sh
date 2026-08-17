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

# Enforce host port exposure: only the caddy service may publish ports.
if grep -Eq '^[[:space:]]*network_mode:[[:space:]]*["'"']?host["'"']?[[:space:]]*$' compose.yaml; then
  fail "network_mode host is forbidden"
fi

current_service=""
in_services=0
caddy_ports=0
while IFS= read -r line; do
  if [[ "$line" == "services:" ]]; then
    in_services=1
    continue
  fi
  if (( in_services == 1 )) && [[ "$line" =~ ^[^[:space:]] ]]; then
    in_services=0
  fi
  if (( in_services == 1 )) && [[ "$line" =~ ^[[:space:]]{2}([A-Za-z0-9_-]+):[[:space:]]*$ ]]; then
    current_service="${BASH_REMATCH[1]}"
    continue
  fi
  if (( in_services == 1 )) && [[ "$line" =~ ^[[:space:]]{4}ports:[[:space:]]*$ ]]; then
    [[ "$current_service" == "caddy" ]] || fail "host port exposure is forbidden for service $current_service"
    caddy_ports=1
  fi
done < compose.yaml
(( caddy_ports == 1 )) || fail "caddy must publish the HTTPS edge ports"

while IFS= read -r port_line; do
  port="$(sed -E 's/^[[:space:]]*-[[:space:]]*["'"']?([^"'"']+)["'"']?[[:space:]]*$/\1/' <<<"$port_line")"
  case "$port" in
    80:80|443:443|443:443/udp) ;;
    *) fail "caddy may publish only host ports 80 and 443, found: $port" ;;
  esac
done < <(awk '
  /^  caddy:/ { in_caddy = 1; next }
  in_caddy && /^    ports:/ { in_ports = 1; next }
  in_ports && /^      - / { print; next }
  in_ports { exit }
' compose.yaml)

echo "Finance edge security contract OK"
