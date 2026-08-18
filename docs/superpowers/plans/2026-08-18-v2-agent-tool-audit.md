# V2 Agent Tool Audit Implementation Plan

**Goal:** Make every externally eligible Agent Adapter call fail-closed on audit persistence, storing only canonical hashes and sanitized metadata before MCP transport is exposed.

**Architecture:** Keep deterministic dispatch in `agentadapter.Service`. Add `agentadapter.AuditedService` as the only service type intended for external protocols. It writes an attempt before calling the base service, withholds successful business data until completion is persisted, and returns `audit_unavailable` if either persistence boundary fails. PostgreSQL persistence lives in `internal/audit` and implements an interface owned by `internal/agentadapter`, avoiding any database dependency in the adapter package.

**No external endpoint is added in this plan.**

## Task 1 — Fail-closed protocol-neutral audit wrapper

Create:
- `internal/agentadapter/audit.go`
- `internal/agentadapter/audit_test.go`

Locked metadata:

```go
type CallMetadata struct {
    Protocol        string
    ProtocolVersion string
    ClientName      string
    ClientVersion   string
}

type AuditAttempt struct {
    CreatedAt       time.Time
    PrincipalKind   string
    HouseholdID     int64
    Protocol        string
    ProtocolVersion string
    ClientName      string
    ClientVersion   string
    ToolName        ToolName
    InputSHA256     string
}

type AuditSuccess struct {
    OutputSHA256 string
    DataAsOf     *time.Time
    DurationMS   int64
}

type AuditFailure struct {
    ErrorCode  ErrorCode
    DurationMS int64
}

type AuditRecorder interface {
    Start(context.Context, AuditAttempt) (int64, error)
    CompleteSuccess(context.Context, int64, AuditSuccess) error
    CompleteFailure(context.Context, int64, AuditFailure) error
}
```

Wrapper:

```go
func NewAudited(service *Service, recorder AuditRecorder, now func() time.Time) (*AuditedService, error)
func (s *AuditedService) Call(ctx context.Context, principal Principal, metadata CallMetadata, name ToolName, arguments json.RawMessage) (Result, error)
```

TDD requirements:
1. `Start` failure => `CodeAuditUnavailable`; base backend call count remains zero.
2. Successful base call + `CompleteSuccess` failure => business result is withheld; `CodeAuditUnavailable`.
3. Successful base call + completion success => result is disclosed and `AuditID` equals the persisted record ID rendered as an opaque string.
4. Base service failure => `CompleteFailure` gets the stable adapter error code; original safe adapter error is returned when failure audit succeeds.
5. Failure audit persistence failure => return `CodeAuditUnavailable` instead of the base error.
6. Audit messages/hashes never contain bearer credentials or raw payloads; recorder receives only metadata and SHA-256 values.
7. Valid JSON is canonicalized before hashing using `json.Decoder.UseNumber` + exactly-one-value + `json.Marshal`; map key order therefore does not change the hash.
8. Invalid JSON uses `SHA256("invalid-json\x00" || raw)`; invalid JSON has no canonical logical representation.
9. Output hash is over `Result.Data`, not the envelope, avoiding `AuditID` recursion.
10. Duration is never negative; inject `now` for deterministic tests.

Use `audit_<base36 id>` for the external `Result.AuditID`; callers treat it as opaque and it does not expose raw financial data.

## Task 2 — Agent audit state-machine schema

Create:
- `db/migrations/00007_agent_tool_audit.sql`
- `db/queries/agent_tool_audit.sql`

Table `agent_tool_audits`:

```text
id BIGINT identity primary key
created_at timestamptz not null
principal_kind text not null
household_id bigint not null references households(id)
protocol text not null
protocol_version text not null
client_name text null
client_version text null
tool_name text not null
input_sha256 char(64) not null
output_sha256 char(64) null
data_as_of timestamptz null
status text not null: running | success | error
error_code text null
duration_ms bigint null
```

Constraints:
- non-empty trimmed principal/protocol/protocol_version/tool;
- hashes lower-hex 64 chars;
- `running`: output/data-as-of/error/duration all NULL;
- `success`: output hash present, error NULL, duration >= 0; data-as-of may be NULL (purchase simulation has no freshness timestamp);
- `error`: output NULL, non-empty error code, duration >= 0;
- no raw input/result/error-message columns.

Queries:
- `CreateAgentToolAuditAttempt :one`
- `CompleteAgentToolAuditSuccess :one` with `WHERE id=$1 AND status='running'`
- `CompleteAgentToolAuditFailure :one` with same state guard
- `GetAgentToolAudit :one` for integration tests.

TDD: first add an integration test requiring attempt/success/failure state transitions and forbidden-column checks; verify RED before migration/query/recorder implementation.

## Task 3 — PostgreSQL recorder

Create:
- `internal/audit/agent_postgres.go`
- `internal/audit/agent_postgres_test.go`

`AgentPostgresRecorder` implements `agentadapter.AuditRecorder` with sqlc queries.

Rules:
- validate record metadata before SQL;
- `Start` returns positive ID;
- completion uses state-guarded UPDATE; zero/no row is an error;
- nullable client/data-as-of mapped with pgtype;
- duration overflow rejected before converting to database types;
- no transaction is needed for one-row state transitions;
- database errors are wrapped for server-side logs only.

Integration test proves:
1. running row after Start contains only hashes/metadata;
2. success transition persists output hash/as-of/duration;
3. error transition persists only stable error code/duration;
4. second completion on the same record fails;
5. schema has no raw content columns;
6. FK rejects nonexistent household.

After query/migration changes, run `sqlc generate` and commit the exact generated source set.

## Task 4 — CI audit integration gate

Modify `.github/workflows/ci.yml`:
- add an explicit `Agent tool audit integration` step after Advice audit integration:

```bash
go test ./internal/audit -run TestAgentPostgresRecorder -v
```

Keep all existing V1 gates.

## Task 5 — Exact final verification

For the exact final commit require:
- migration up/down/up success;
- sqlc generated source clean;
- `go test ./internal/agentadapter ./internal/audit -v` success;
- explicit Agent tool audit integration success;
- `go test -race ./internal/agentadapter ./internal/audit` success;
- `go test ./...` success;
- govulncheck success;
- Docker/Web/Edge Security unchanged and green;
- no MCP SDK, `/mcp`, Caddy route, token config, or new host port yet.

Only after this plan is green may MCP transport work begin.
