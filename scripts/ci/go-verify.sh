#!/usr/bin/env bash
set -euo pipefail

SOURCE_ROOT="${CI_SOURCE_ROOT:-/src}"
[[ -d "$SOURCE_ROOT/.git" || -f "$SOURCE_ROOT/.git" ]] || {
  echo "Go verification requires a git checkout mounted at $SOURCE_ROOT" >&2
  exit 1
}

WORK_ROOT="$(mktemp -d /tmp/family-finance-go-verify.XXXXXX)"
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

sqlc version
goose -version
govulncheck -version

sqlc generate
diff -ru "$SOURCE_ROOT/internal/store/sqlc" "$WORK_ROOT/internal/store/sqlc"

go mod tidy
cmp -s "$SOURCE_ROOT/go.mod" "$WORK_ROOT/go.mod" || {
  echo "go mod tidy changed go.mod" >&2
  diff -u "$SOURCE_ROOT/go.mod" "$WORK_ROOT/go.mod" || true
  exit 1
}
cmp -s "$SOURCE_ROOT/go.sum" "$WORK_ROOT/go.sum" || {
  echo "go mod tidy changed go.sum" >&2
  diff -u "$SOURCE_ROOT/go.sum" "$WORK_ROOT/go.sum" || true
  exit 1
}

mcp_version="$(go list -m -f '{{.Version}}' github.com/modelcontextprotocol/go-sdk)"
if [[ "$mcp_version" != "v1.6.1" ]]; then
  echo "expected stable MCP Go SDK v1.6.1, got $mcp_version" >&2
  exit 1
fi
if [[ "$mcp_version" == *-* ]]; then
  echo "pre-release MCP SDK versions are forbidden in the production dependency graph: $mcp_version" >&2
  exit 1
fi

mapfile -t gofiles < <(find cmd internal -type f -name '*.go' -print)
test -z "$(gofmt -l "${gofiles[@]}")"

go vet ./...
go test ./...
go test ./internal/mcpadapter -v
govulncheck ./...

go test ./internal/store -run TestOpenPostgresIntegration -v
go test ./internal/household -run Integration -v
go test ./internal/budget -run Integration -v
go test ./internal/goals -run Integration -v
go test ./internal/portfolio -run Integration -v
go test ./internal/audit -run Integration -v
go test ./internal/audit -run TestAgentPostgresRecorder -v
go test ./internal/scheduler -run Integration -v
go test ./internal/appapi -run TestPostgresPlannerRoundTripIntegration -v
go test ./cmd/finance-core -run TestBuildApplicationHandlerWithoutLLMIntegration -v
go test ./cmd/finance-core -run MCP -v

go test -race ./...
CGO_ENABLED=0 go build -buildvcs=false -trimpath -o /tmp/finance-core ./cmd/finance-core

echo "Go verification OK"
