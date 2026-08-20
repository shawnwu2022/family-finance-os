#!/usr/bin/env bash
set -euo pipefail
umask 077

fail() {
  echo "Ollama schema compatibility probe failed: $*" >&2
  exit 1
}

for command_name in docker curl jq; do
  command -v "$command_name" >/dev/null || fail "$command_name is required"
done

ollama_image="ollama/ollama:0.32.5"
ollama_model="qwen3.5:4b"
container="family-finance-ollama-schema-${GITHUB_RUN_ID:-$$}-${RANDOM}"
workdir="$(mktemp -d /tmp/family-finance-ollama-schema.XXXXXX)"

cleanup() {
  set +e
  docker rm -f "$container" >/dev/null 2>&1 || true
  rm -rf "$workdir"
}
trap cleanup EXIT INT TERM

docker run -d --name "$container" --pull always \
  -p 127.0.0.1:11434:11434 "$ollama_image" >"$workdir/container-id" \
  || fail "Ollama container failed to start"

ready=0
for _ in $(seq 1 60); do
  if curl --silent --show-error --fail --connect-timeout 2 --max-time 5 \
    http://127.0.0.1:11434/api/tags >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
[[ "$ready" == "1" ]] || fail "Ollama native API did not become ready"

if ! docker exec "$container" ollama pull "$ollama_model" \
  >"$workdir/pull.stdout" 2>"$workdir/pull.stderr"; then
  fail "Ollama acceptance model pull failed"
fi

prompt='Acceptance check. You MUST call the only available tool, finance__get_household_overview, exactly once. Do not answer from memory or prior knowledge. Only after that tool succeeds, reply exactly FINANCE_MCP_READ_OK. If the tool is unavailable or fails, do not output that marker.'
description='Get the current deterministic Finance Core household overview. Preserve quality and warning metadata.'
expected_tool='finance__get_household_overview'

run_probe() {
  local label="$1"
  local normalized="$2"
  local request="$workdir/${label}-request.json"
  local response="$workdir/${label}-response.ndjson"

  if [[ "$normalized" == "1" ]]; then
    jq -n \
      --arg model "$ollama_model" \
      --arg prompt "$prompt" \
      --arg tool "$expected_tool" \
      --arg description "$description" '
{
  model: $model,
  messages: [{role: "user", content: $prompt}],
  stream: true,
  think: false,
  tools: [
    {
      type: "function",
      function: {
        name: $tool,
        description: $description,
        parameters: {
          type: "object",
          additionalProperties: false,
          properties: {}
        }
      }
    }
  ]
}
' >"$request"
  else
    jq -n \
      --arg model "$ollama_model" \
      --arg prompt "$prompt" \
      --arg tool "$expected_tool" \
      --arg description "$description" '
{
  model: $model,
  messages: [{role: "user", content: $prompt}],
  stream: true,
  think: false,
  tools: [
    {
      type: "function",
      function: {
        name: $tool,
        description: $description,
        parameters: {
          type: "object",
          additionalProperties: false
        }
      }
    }
  ]
}
' >"$request"
  fi

  if ! curl --silent --show-error --fail --connect-timeout 5 --max-time 300 \
    --header 'Content-Type: application/json' \
    --data-binary @"$request" \
    http://127.0.0.1:11434/api/chat >"$response"; then
    fail "${label} native Ollama request failed"
  fi

  if ! jq -s -e --arg tool "$expected_tool" '
    [.[].message.tool_calls[]?] as $calls
    | ($calls | length == 1)
      and ($calls[0].function.name == $tool)
      and (($calls[0].function.arguments // {}) == {})
  ' "$response" >/dev/null; then
    fail "${label} schema did not produce the required native tool call"
  fi
}

run_probe baseline 0
printf 'ollama_baseline_empty_schema_probe=PASS\n'
run_probe openclaw-normalized 1
printf 'ollama_openclaw_empty_schema_probe=PASS\n'
