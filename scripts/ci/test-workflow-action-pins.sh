#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "workflow action pin contract failed: $*" >&2
  exit 1
}

leading_indent() {
  sed -E 's/^([[:space:]]*).*/\1/' <<<"$1"
}

checkout_has_private_credentials() {
  local workflow="$1"
  local checkout_index="$2"
  local -n workflow_lines="$3"
  local checkout_line="${workflow_lines[$checkout_index]}"
  local step_indent_text
  step_indent_text="$(leading_indent "$checkout_line")"
  local step_indent=${#step_indent_text}
  local with_indent=-1
  local found=0
  local i line indent_text indent trimmed

  for ((i = checkout_index + 1; i < ${#workflow_lines[@]}; i++)); do
    line="${workflow_lines[$i]}"
    indent_text="$(leading_indent "$line")"
    indent=${#indent_text}
    trimmed="${line#$indent_text}"

    if [[ -z "$trimmed" || "$trimmed" == \#* ]]; then
      continue
    fi
    if (( indent <= step_indent )); then
      break
    fi

    if (( with_indent >= 0 && indent <= with_indent )); then
      with_indent=-1
    fi
    if [[ "$trimmed" == "with:" ]]; then
      with_indent=$indent
      continue
    fi
    if (( with_indent >= 0 && indent > with_indent )) && [[ "$trimmed" =~ ^persist-credentials:[[:space:]]+false([[:space:]]*(#.*)?)?$ ]]; then
      found=1
      break
    fi
  done

  (( found == 1 )) || fail "$workflow checkout step must set persist-credentials: false in its own with block"
}

remote_uses=0
checkout_uses=0
while IFS= read -r workflow; do
  mapfile -t lines <"$workflow"
  for ((i = 0; i < ${#lines[@]}; i++)); do
    line="${lines[$i]}"
    action="$(sed -E 's/^[[:space:]]*-[[:space:]]*uses:[[:space:]]*([^[:space:]#]+).*/\1/' <<<"$line")"
    [[ "$action" != "$line" ]] || continue
    [[ "$action" == ./* ]] && continue

    remote_uses=$((remote_uses + 1))
    if [[ ! "$action" =~ ^[^@]+@[0-9a-f]{40}$ ]]; then
      fail "$workflow contains mutable or non-SHA action reference: $action"
    fi
    if [[ "$action" == actions/checkout@* ]]; then
      checkout_uses=$((checkout_uses + 1))
      checkout_has_private_credentials "$workflow" "$i" lines
    fi
  done
done < <(find .github/workflows -maxdepth 1 -type f -name '*.yml' -print | sort)

(( remote_uses > 0 )) || fail "no remote GitHub Actions references found"
(( checkout_uses > 0 )) || fail "no actions/checkout references found"

echo "Workflow action pin contract OK: remote_uses=$remote_uses checkout_uses=$checkout_uses"
