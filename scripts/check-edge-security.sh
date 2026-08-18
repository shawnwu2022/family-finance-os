#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "edge security contract failed: $*" >&2
  exit 1
}

finance_block="$({
  awk '
    /^\{\$FINANCE_DOMAIN\} \{/ {
      in_finance = 1
      depth = 0
    }
    in_finance {
      print
      opens = gsub(/\{/, "{")
      closes = gsub(/\}/, "}")
      depth += opens - closes
      if (depth == 0) {
        exit
      }
    }
  ' Caddyfile
} || true)"

[[ -n "$finance_block" ]] || fail "FINANCE_DOMAIN site block is missing"
grep -Eq '^[[:space:]]*@mcp[[:space:]]+path[[:space:]]+/mcp[[:space:]]*$' <<<"$finance_block" || fail "FINANCE_DOMAIN must define exact @mcp path /mcp matcher"
grep -Eq '^[[:space:]]*handle[[:space:]]+@mcp[[:space:]]*\{' <<<"$finance_block" || fail "FINANCE_DOMAIN must route @mcp through a dedicated handle block"
grep -Eq '^[[:space:]]*handle[[:space:]]*\{' <<<"$finance_block" || fail "FINANCE_DOMAIN must keep a fallback handle for authenticated Finance UI/API"
grep -Eq '^[[:space:]]*basic_auth([[:space:]]|$)' <<<"$finance_block" || fail "FINANCE_DOMAIN fallback must enforce basic_auth"
grep -Fq '{$FINANCE_AUTH_USER}' <<<"$finance_block" || fail "FINANCE_DOMAIN auth username must come from FINANCE_AUTH_USER"
grep -Fq '{$FINANCE_AUTH_HASH}' <<<"$finance_block" || fail "FINANCE_DOMAIN auth hash must come from FINANCE_AUTH_HASH"

mcp_block="$({
  awk '
    /^[[:space:]]*handle[[:space:]]+@mcp[[:space:]]*\{/ {
      in_mcp = 1
      depth = 0
    }
    in_mcp {
      print
      opens = gsub(/\{/, "{")
      closes = gsub(/\}/, "}")
      depth += opens - closes
      if (depth == 0) {
        exit
      }
    }
  ' Caddyfile
} || true)"
[[ -n "$mcp_block" ]] || fail "MCP edge handle block is missing"
grep -Fq 'reverse_proxy finance-core:8000' <<<"$mcp_block" || fail "MCP edge handle must reverse_proxy finance-core:8000"
if grep -Eq '^[[:space:]]*basic_auth([[:space:]]|$)' <<<"$mcp_block"; then
  fail "MCP edge handle must not consume Authorization with Caddy basic_auth"
fi

fallback_block="$({
  awk '
    /^[[:space:]]*handle[[:space:]]*\{/ {
      in_fallback = 1
      depth = 0
    }
    in_fallback {
      print
      opens = gsub(/\{/, "{")
      closes = gsub(/\}/, "}")
      depth += opens - closes
      if (depth == 0) {
        exit
      }
    }
  ' Caddyfile
} || true)"
[[ -n "$fallback_block" ]] || fail "Finance fallback handle block is missing"
grep -Eq '^[[:space:]]*basic_auth([[:space:]]|$)' <<<"$fallback_block" || fail "Finance fallback handle must enforce basic_auth"
grep -Fq 'reverse_proxy finance-core:8000' <<<"$fallback_block" || fail "Finance fallback handle must reverse_proxy finance-core:8000"

grep -Fq 'FINANCE_AUTH_USER: ${FINANCE_AUTH_USER:?' compose.yaml || fail "Compose must require FINANCE_AUTH_USER"
grep -Fq 'FINANCE_AUTH_HASH: ${FINANCE_AUTH_HASH:?' compose.yaml || fail "Compose must require FINANCE_AUTH_HASH"
grep -Fq 'FINANCE_AUTH_USER=' .env.example || fail ".env.example must document FINANCE_AUTH_USER"
grep -Fq 'FINANCE_AUTH_HASH=' .env.example || fail ".env.example must document FINANCE_AUTH_HASH"

for key in MCP_ENABLED MCP_TOKEN_FILE MCP_HOUSEHOLD_ID MCP_ALLOWED_ORIGINS MCP_REQUEST_TIMEOUT MCP_MAX_CONCURRENT MCP_REQUESTS_PER_MINUTE MCP_MAX_BODY_BYTES; do
  grep -Fq "${key}:" compose.yaml || fail "finance-core Compose environment must pass ${key}"
  grep -Fq "${key}=" .env.example || fail ".env.example must document ${key}"
done
grep -Fq './secrets:/run/secrets:ro' compose.yaml || fail "finance-core must mount the MCP secret directory read-only"
grep -Fq 'secrets/*' .gitignore || fail ".gitignore must ignore secret files"
grep -Fq '!secrets/.gitkeep' .gitignore || fail ".gitignore must retain only secrets/.gitkeep"
if grep -Eq '^[[:space:]]*MCP_TOKEN:' compose.yaml || grep -Eq '^MCP_TOKEN=' .env.example; then
  fail "literal MCP bearer token environment variables are forbidden"
fi

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
if grep -Eq '^[[:space:]]*network_mode:[[:space:]]*.*host.*$' compose.yaml; then
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

port_count=0
while IFS= read -r port_line; do
  port="${port_line#*- }"
  port="${port#\"}"
  port="${port%\"}"
  case "$port" in
    80:80|443:443|443:443/udp) ;;
    *) fail "caddy may publish only host ports 80 and 443, found: $port" ;;
  esac
  port_count=$((port_count + 1))
done < <(awk '
  /^  caddy:/ { in_caddy = 1; next }
  in_caddy && /^    ports:/ { in_ports = 1; next }
  in_ports && /^      - / { print; next }
  in_ports { exit }
' compose.yaml)
(( port_count == 3 )) || fail "caddy must publish 80/tcp, 443/tcp, and 443/udp"

echo "Finance edge security contract OK"
