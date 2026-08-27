#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT_DIR"

fail() {
  echo "Finance edge security contract failed: $*" >&2
  exit 1
}

# Only Caddy may expose host ports. Any direct application/database exposure fails closed.
port_services="$(awk '
  /^services:/ { in_services=1; next }
  in_services && /^  [A-Za-z0-9_-]+:/ { service=$1; sub(/:$/, "", service) }
  in_services && /^    ports:/ { print service }
' compose.yaml | sort -u)"
[[ "$port_services" == "caddy" ]] || fail "host port exposure must be limited to caddy (found: ${port_services:-none})"

if grep -Eq '^[[:space:]]*network_mode:[[:space:]]*host([[:space:]]|$)' compose.yaml; then
  fail "network_mode host is forbidden"
fi
for target in '80:80' '443:443' '443:443/udp'; do
  grep -Fq "\"$target\"" compose.yaml || fail "caddy host port exposure is missing $target"
done

finance_block="$({
  awk '
    /^\{\$FINANCE_DOMAIN\} \{/ { in_finance=1; depth=0 }
    in_finance {
      print
      opens=gsub(/\{/, "{")
      closes=gsub(/\}/, "}")
      depth += opens - closes
      if (depth == 0) exit
    }
  ' Caddyfile
} || true)"
[[ -n "$finance_block" ]] || fail "FINANCE_DOMAIN site block is missing"
if grep -Eq '(^|[[:space:]])basic_auth([[:space:]]|$)' <<<"$finance_block"; then
  fail "Caddy must not authenticate Finance users"
fi
grep -Fq '@mcp path /mcp' <<<"$finance_block" || fail "Finance /mcp route is missing"
reverse_proxy_count="$(grep -c 'reverse_proxy finance-core:8000' <<<"$finance_block" || true)"
[[ "$reverse_proxy_count" -ge 2 ]] || fail "Finance MCP and browser routes must both proxy to finance-core"

for file in Caddyfile Caddyfile.acceptance compose.yaml .env.example; do
  if grep -Eq 'FINANCE_AUTH_(USER|HASH)' "$file"; then
    fail "$file contains legacy Caddy Finance credentials"
  fi
done

if grep -Eq '^[[:space:]]*EBK_API_TOKEN:' compose.yaml || grep -Eq '^EBK_API_TOKEN=' .env.example; then
  fail "ezBookkeeping API token must not be stored as an environment value"
fi
if grep -Eq '^[[:space:]]*EBK_SECURITY_SECRET_KEY:' compose.yaml || grep -Eq '^EBK_SECURITY_SECRET_KEY=' .env.example; then
  fail "ezBookkeeping signing secret must not be stored as an environment value"
fi

echo "Finance edge security contract OK"
