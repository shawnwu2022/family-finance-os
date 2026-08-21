#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "workflow action pin contract failed: $*" >&2
  exit 1
}

leading_indent() {
  sed -E 's/^([[:space:]]*).*/\1/' <<<"$1"
}

extract_action() {
  local line="$1"
  local indent_text trimmed
  indent_text="$(leading_indent "$line")"
  trimmed="${line#$indent_text}"
  sed -nE 's/^(-[[:space:]]+)?uses:[[:space:]]*([^[:space:]#]+).*/\2/p' <<<"$trimmed"
}

find_step_start() {
  local uses_index="$1"
  local -n workflow_lines="$2"
  local uses_line="${workflow_lines[$uses_index]}"
  local uses_indent_text
  uses_indent_text="$(leading_indent "$uses_line")"
  local uses_indent=${#uses_indent_text}
  local trimmed="${uses_line#$uses_indent_text}"
  local i line indent_text indent candidate

  if [[ "$trimmed" == -[[:space:]]uses:* ]]; then
    printf '%s\n' "$uses_index"
    return 0
  fi

  for ((i = uses_index - 1; i >= 0; i--)); do
    line="${workflow_lines[$i]}"
    indent_text="$(leading_indent "$line")"
    indent=${#indent_text}
    candidate="${line#$indent_text}"

    if [[ -z "$candidate" || "$candidate" == \#* ]]; then
      continue
    fi
    if (( indent < uses_indent )); then
      if [[ "$candidate" == "-" || "$candidate" == -[[:space:]]* ]]; then
        printf '%s\n' "$i"
        return 0
      fi
      return 1
    fi
  done
  return 1
}

checkout_has_private_credentials() {
  local workflow="$1"
  local step_index="$2"
  local -n workflow_lines="$3"
  local step_line="${workflow_lines[$step_index]}"
  local step_indent_text
  step_indent_text="$(leading_indent "$step_line")"
  local step_indent=${#step_indent_text}
  local with_indent=$((step_indent + 2))
  local input_indent=$((with_indent + 2))
  local in_with=0
  local found=0
  local i line indent_text indent trimmed

  for ((i = step_index + 1; i < ${#workflow_lines[@]}; i++)); do
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

    if (( indent == with_indent )) && [[ "$trimmed" == "with:" ]]; then
      in_with=1
      continue
    fi
    if (( in_with == 1 && indent <= with_indent )); then
      in_with=0
    fi
    if (( in_with == 1 && indent == input_indent )) && [[ "$trimmed" =~ ^persist-credentials:[[:space:]]+false([[:space:]]*(#.*)?)?$ ]]; then
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
    action="$(extract_action "${lines[$i]}")"
    [[ -n "$action" ]] || continue
    [[ "$action" == ./* ]] && continue

    remote_uses=$((remote_uses + 1))
    if [[ ! "$action" =~ ^[^@]+@[0-9a-f]{40}$ ]]; then
      fail "$workflow contains mutable or non-SHA action reference: $action"
    fi

    if [[ "$action" == actions/checkout@* ]]; then
      checkout_uses=$((checkout_uses + 1))
      if ! step_index="$(find_step_start "$i" lines)"; then
        fail "$workflow actions/checkout reference is not inside a workflow step"
      fi
      checkout_has_private_credentials "$workflow" "$step_index" lines
    fi
  done
done < <(find .github/workflows -maxdepth 1 -type f -name '*.yml' -print | sort)

(( remote_uses > 0 )) || fail "no remote GitHub Actions references found"
(( checkout_uses > 0 )) || fail "no actions/checkout references found"

echo "Workflow action pin contract OK: remote_uses=$remote_uses checkout_uses=$checkout_uses"
