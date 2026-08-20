# V1 Production Acceptance Evidence

> 本文件是 production release 的证据索引，不保存原始财务数据、完整账户号、API key、token、密码、SSH private key 或未脱敏日志。
>
> 状态只允许：`PASS`、`FAIL`、`NOT RUN`、`BLOCKED`。没有真实执行证据时不得填写 `PASS`。

## Evidence model

- **Runtime target commit** 是实际被自动化与真实环境验收的应用/部署 payload。当前固定为 `c297c206c1a57ba997c81c2f2c51e3152982c9e9`。
- 只修改本证据文件、README、LICENSE 或其它纯治理元数据的后续 commit **不重新定义 runtime target**；否则记录证据本身会造成无限 SHA 递归。
- 任何影响应用代码、依赖、数据库 schema、Docker/Compose/Caddy、CI/acceptance 逻辑或运行时配置契约的变更，都必须选择新的 runtime target，并重新执行适用 gate。
- Issue #26 是当前 production acceptance 执行清单；本文件是最终 release decision 的证据总账。

## Release Candidate

| 字段 | 值 |
|---|---|
| Runtime target commit | `c297c206c1a57ba997c81c2f2c51e3152982c9e9` |
| Finance Core version/tag | `pre-release / not tagged` |
| ezBookkeeping version | `1.6.1` |
| PostgreSQL major | `18` |
| Production host | `REDACTED / NOT RUN` |
| Acceptance operator | `NOT RUN` |
| Acceptance completed at | `NOT RUN` |
| Production release decision | `BLOCKED` |

## A. CI / Reproducible Gates

所有自动化结果均来自同一个 runtime target `c297c206c1a57ba997c81c2f2c51e3152982c9e9` 的 `main` push。完整 repository-native gate 由 CI run `32396818970` 执行；独立安全/Agent gate 分别为 MCP Security `32396818913`、Edge Security `32396818948`、OpenClaw Release Acceptance `32396818874`。

| Gate | Status | Evidence |
|---|---|---|
| migration up/down/up | PASS | CI `32396818970` / `make verify` |
| sqlc generated source clean | PASS | CI `32396818970` / `make verify` |
| gofmt / go vet / go test | PASS | CI `32396818970` / `make verify` |
| govulncheck | PASS | CI `32396818970` / `make verify` |
| PostgreSQL + domain integrations | PASS | CI `32396818970` / `make verify` |
| scheduler restart/idempotency integration | PASS | CI `32396818970` / `make verify` automated integration |
| backup restore drill | PASS | CI `32396818970` / disposable restore drill; **not** real off-host recovery |
| go race | PASS | CI `32396818970` / `make verify` |
| Go binary build | PASS | CI `32396818970` / `make verify` |
| frontend npm ci + unit | PASS | CI `32396818970` / `make verify` |
| PWA contract + typecheck + build | PASS | CI `32396818970` / `make verify` |
| container build | PASS | CI `32396818970` / `make verify` |
| production operations contract | PASS | CI `32396818970` / `make verify` |
| edge exposure/security workflow | PASS | Edge Security `32396818948` |
| MCP security workflow | PASS | MCP Security `32396818913` |
| Real OpenClaw ephemeral acceptance | PASS | OpenClaw `32396818874`; `openclaw_mcp_live_smoke=PASS`, exactly 12 tools, 401/403 negative probes, read+simulation audit count = 1 each, `openclaw_release_acceptance=PASS` |

Automated PASS proves the repository-defined contracts on the exact runtime target. It does **not** replace the real production host, real ledger, off-host disaster recovery, real external Advisor provider, or real-phone acceptance below.

## B. Real Ledger / Complete Month

| Gate | Status | Evidence / Notes |
|---|---|---|
| Real Chinese statement imported through ezBookkeeping | NOT RUN | Do not commit source statement; record statement type, date range, import result count only |
| At least one complete natural month present | NOT RUN | Record month only, e.g. `YYYY-MM` |
| Dashboard generated for that month | NOT RUN | Sanitized screenshot/hash only |
| Monthly report generated for that month | NOT RUN | Sanitized report ID/hash only |
| Income manual reconciliation ≤ 0.01 CNY | NOT RUN | Record expected/actual/delta with sensitive labels removed |
| Expense manual reconciliation ≤ 0.01 CNY | NOT RUN | Record expected/actual/delta |
| Net Cashflow manual reconciliation ≤ 0.01 CNY | NOT RUN | Record expected/actual/delta |
| Net Worth manual reconciliation ≤ 0.01 CNY | NOT RUN | Record expected/actual/delta |
| transfer/refund/reimbursement/credit repayment semantics checked | NOT RUN | Record sampled transaction IDs as irreversible hashes |

## C. Safe-to-Spend / Debt / Scenario

| Gate | Status | Evidence / Notes |
|---|---|---|
| Safe-to-Spend components traceable | NOT RUN | Sanitized component totals |
| No fixed-expense/debt double-count | NOT RUN | Independent calculation reference |
| Equal-payment debt sample ≤ 0.01 CNY | NOT RUN | Spreadsheet/calculator version + delta |
| Equal-principal / extra-payment sample ≤ 0.01 CNY | NOT RUN | Spreadsheet/calculator version + delta |
| Extra payment invariant | NOT RUN | Principal/payoff comparison |
| Liquidity floor invariant | NOT RUN | Scenario input/output summary |

## D. Real LLM / Fallback

| Gate | Status | Evidence / Notes |
|---|---|---|
| Real external provider/model Advisor request | NOT RUN | Ephemeral local Ollama/OpenClaw acceptance does not substitute; record provider family, exact model ID, timestamp; no API key |
| Tool trace and audit hashes recorded | NOT RUN | production Advisor audit IDs/hashes only |
| Critical amounts map to Tool Results | NOT RUN | Sanitized production trace mapping |
| LLM outage deterministic fallback | NOT RUN | Record blocked-provider method and endpoint results |
| No fabricated fresh advice while provider unavailable | NOT RUN | Sanitized response/result |

## E. Backup / Disaster Recovery

| Gate | Status | Evidence / Notes |
|---|---|---|
| Real production backup created | NOT RUN | Backup timestamp and SHA256SUMS hash |
| Restic SFTP snapshot created | NOT RUN | Snapshot ID only |
| `restic check` passed | NOT RUN | Timestamp + sanitized result |
| Off-host restore from real snapshot | NOT RUN | Restore host identifier redacted |
| Restored ezBookkeeping/Finance Core healthy | NOT RUN | health/readiness result |
| Restored key data sampled | NOT RUN | counts/ranges/hashes only |
| RTO recorded | NOT RUN | Duration |

## F. Scheduler Restart

| Gate | Status | Evidence / Notes |
|---|---|---|
| Restart after successful monthly run creates no duplicate | NOT RUN | automated integration passed, but real deployment restart evidence still required |
| Interrupted `running` recovery exercised | NOT RUN | before/after controlled production-like status only |
| Retried run succeeds or fails with controlled error code | NOT RUN | no raw provider/secret-bearing error text |

## G. Mobile / Edge / Secret Hygiene

| Gate | Status | Evidence / Notes |
|---|---|---|
| PWA installed on real phone over HTTPS | NOT RUN | Device family/browser version, no device ID |
| Finance Caddy Basic Auth checked on production edge | NOT RUN | PASS/FAIL only |
| ezBookkeeping 2FA enabled | NOT RUN | PASS/FAIL only |
| Only Caddy exposes host 80/443 on production host | NOT RUN | `docker compose ps` / socket summary, sanitized |
| `.env`/tokens/keys/statements absent from Git | NOT RUN | final secret-hygiene review reference; automated repository checks are supporting evidence only |
| Logs/evidence contain no plaintext secrets | NOT RUN | final review reference |

## H. Release Governance

| Gate | Status | Evidence / Notes |
|---|---|---|
| Production acceptance tracker exists | PASS | GitHub Issue #26 |
| Repository license selected | PASS | MIT; governance PR adds top-level `LICENSE` |
| `main` branch protection / ruleset | BLOCKED | Current connector exposes repository admin rights but no branch-protection/ruleset mutation action; must be configured in repository settings before final release |
| Final security review | NOT RUN | Perform after release-governance changes settle; remediate validated P0/P1 findings before release |

## Final Decision

Production release remains **BLOCKED** until every required real-environment gate above is `PASS`, branch protection is configured, the final security review has no unresolved P0/P1 finding, there are no P0/P1 data-correctness defects, and no unexplained financial delta exceeds 0.01 CNY.
