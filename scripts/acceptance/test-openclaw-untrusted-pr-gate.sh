#!/usr/bin/env bash
set -euo pipefail

fail() {
  echo "OpenClaw untrusted-PR gate contract failed: $*" >&2
  exit 1
}

workflow=".github/workflows/openclaw-release-acceptance.yml"
[[ -f "$workflow" ]] || fail "release workflow is missing"

acceptance_if="$(awk '
  /^[[:space:]]+acceptance:[[:space:]]*$/ { in_job=1; next }
  in_job && /^[[:space:]]{4}if:[[:space:]]/ { sub(/^[[:space:]]{4}if:[[:space:]]*/, ""); print; exit }
  in_job && /^[[:space:]]{2}[A-Za-z0-9_-]+:[[:space:]]*$/ { exit }
' "$workflow")"

[[ -n "$acceptance_if" ]] || fail "Real OpenClaw acceptance job must have an explicit if condition"
grep -Fq "github.event_name == 'workflow_dispatch'" <<<"$acceptance_if" || fail "manual workflow_dispatch must remain allowed"
grep -Fq "github.event_name == 'push'" <<<"$acceptance_if" || fail "main push release path must remain allowed"
grep -Fq "contains(github.event.head_commit.message, '[openclaw-full]')" <<<"$acceptance_if" || fail "main push must remain gated by [openclaw-full]"
grep -Fq "github.event_name == 'pull_request'" <<<"$acceptance_if" || fail "same-repository PR acceptance path must remain allowed"
grep -Fq 'github.event.pull_request.head.repo.full_name == github.repository' <<<"$acceptance_if" || fail "fork pull requests must not execute the expensive Real OpenClaw job"

echo "OpenClaw untrusted-PR gate contract OK"
