#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "Finance frame-protection contract failed: $*" >&2
  exit 1
}

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
cd "$ROOT_DIR"

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
grep -Eq '^[[:space:]]*X-Frame-Options[[:space:]]+"?DENY"?[[:space:]]*$' <<<"$finance_block" || fail "Finance edge must deny framing with X-Frame-Options"
grep -Fq "Content-Security-Policy \"frame-ancestors 'none'\"" <<<"$finance_block" || fail "Finance edge must set CSP frame-ancestors 'none'"

echo "Finance frame-protection contract OK"
