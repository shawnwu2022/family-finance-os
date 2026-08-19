#!/usr/bin/env bash
set -euo pipefail

SOURCE_ROOT="${CI_SOURCE_ROOT:-/src}"
[[ -f "$SOURCE_ROOT/go.mod" ]] || {
  echo "MCP security verification requires repository source mounted at $SOURCE_ROOT" >&2
  exit 1
}

WORK_ROOT="$(mktemp -d /tmp/family-finance-mcp-security.XXXXXX)"
cleanup() {
  rm -rf "$WORK_ROOT"
}
trap cleanup EXIT

cp -a "$SOURCE_ROOT/." "$WORK_ROOT/"
cd "$WORK_ROOT"

[[ "$(go env GOVERSION)" == "go1.26.6" ]] || {
  echo "expected Go 1.26.6, got $(go env GOVERSION)" >&2
  exit 1
}

mcp_version="$(go list -m -f '{{.Version}}' github.com/modelcontextprotocol/go-sdk)"
if [[ "$mcp_version" != "v1.6.1" ]]; then
  echo "expected stable MCP Go SDK v1.6.1, got $mcp_version" >&2
  exit 1
fi
if [[ "$mcp_version" == *-* ]]; then
  echo "pre-release MCP SDK versions are forbidden: $mcp_version" >&2
  exit 1
fi

go test ./internal/mcpadapter -run 'NewSecureHTTPHandler|SecureHTTPHandler' -v
go test ./internal/agentadapter -run 'TestEncodeBackendResultMaps|TestAuditedCallPreservesTimeoutWhenRequestCancelsBeforeFailureAuditCompletion' -v
go test -race ./internal/mcpadapter -run 'NewSecureHTTPHandler|SecureHTTPHandler' -v

echo "MCP security verification OK"
