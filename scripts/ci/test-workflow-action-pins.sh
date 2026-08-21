#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "workflow action pin contract failed: $*" >&2
  exit 1
}

remote_uses=0
checkout_uses=0
checkout_private=0
while IFS= read -r workflow; do
  while IFS= read -r line; do
    action="$(sed -E 's/^[[:space:]]*-[[:space:]]*uses:[[:space:]]*([^[:space:]#]+).*/\1/' <<<"$line")"
    [[ "$action" != "$line" ]] || continue
    [[ "$action" == ./* ]] && continue
    remote_uses=$((remote_uses + 1))
    if [[ ! "$action" =~ ^[^@]+@[0-9a-f]{40}$ ]]; then
      fail "$workflow contains mutable or non-SHA action reference: $action"
    fi
    if [[ "$action" == actions/checkout@* ]]; then
      checkout_uses=$((checkout_uses + 1))
    fi
  done <"$workflow"
  checkout_private=$((checkout_private + $(grep -Ec '^[[:space:]]+persist-credentials:[[:space:]]+false[[:space:]]*$' "$workflow" || true)))
done < <(find .github/workflows -maxdepth 1 -type f -name '*.yml' -print | sort)

(( remote_uses > 0 )) || fail "no remote GitHub Actions references found"
(( checkout_uses > 0 )) || fail "no actions/checkout references found"
(( checkout_private == checkout_uses )) || fail "every checkout step must set persist-credentials: false"

echo "Workflow action pin contract OK: remote_uses=$remote_uses checkout_uses=$checkout_uses"
