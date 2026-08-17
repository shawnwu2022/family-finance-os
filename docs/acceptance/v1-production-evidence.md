# V1 Production Acceptance Evidence

> 本文件是 V1 release tag 的证据索引，不保存原始财务数据、完整账户号、API key、token、密码、SSH private key 或未脱敏日志。
>
> 状态只允许：`PASS`、`FAIL`、`NOT RUN`、`BLOCKED`。没有真实执行证据时不得填写 `PASS`。

## Release Candidate

| 字段 | 值 |
|---|---|
| Target commit | `TBD` |
| Finance Core version/tag | `TBD` |
| ezBookkeeping version | `1.6.1` |
| PostgreSQL major | `18` |
| Production host | `REDACTED / NOT RUN` |
| Acceptance operator | `NOT RUN` |
| Acceptance completed at | `NOT RUN` |
| V1 release decision | `BLOCKED` |

## A. CI / Reproducible Gates

CI 结果必须来自同一个 target commit。合并前 PR 结果可作为候选证据；正式 V1 tag 前应把 target commit 和最终 workflow run 记录到本表。

| Gate | Status | Evidence |
|---|---|---|
| migration up/down/up | NOT RUN | final target commit workflow required |
| sqlc generated source clean | NOT RUN | final target commit workflow required |
| gofmt / go vet / go test | NOT RUN | final target commit workflow required |
| govulncheck | NOT RUN | final target commit workflow required |
| PostgreSQL + domain integrations | NOT RUN | final target commit workflow required |
| scheduler restart/idempotency integration | NOT RUN | final target commit workflow required |
| backup restore drill | NOT RUN | final target commit workflow required |
| go race | NOT RUN | final target commit workflow required |
| Go binary build | NOT RUN | final target commit workflow required |
| frontend npm ci + unit | NOT RUN | final target commit workflow required |
| PWA contract + typecheck + build | NOT RUN | final target commit workflow required |
| container build | NOT RUN | final target commit workflow required |
| production operations contract | NOT RUN | final target commit workflow required |
| edge exposure/security workflow | NOT RUN | final target commit workflow required |

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
| Real provider/model Advisor request | NOT RUN | Record provider family, exact model ID, timestamp; no API key |
| Tool trace and audit hashes recorded | NOT RUN | audit IDs/hashes only |
| Critical amounts map to Tool Results | NOT RUN | Sanitized trace mapping |
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
| Restart after successful monthly run creates no duplicate | NOT RUN | household/job/scheduled_for values may be hashed if sensitive |
| Interrupted `running` recovery exercised | NOT RUN | before/after controlled status only |
| Retried run succeeds or fails with controlled error code | NOT RUN | no raw provider/secret-bearing error text |

## G. Mobile / Edge / Secret Hygiene

| Gate | Status | Evidence / Notes |
|---|---|---|
| PWA installed on real phone over HTTPS | NOT RUN | Device family/browser version, no device ID |
| Finance Caddy Basic Auth checked | NOT RUN | PASS/FAIL only |
| ezBookkeeping 2FA enabled | NOT RUN | PASS/FAIL only |
| Only Caddy exposes host 80/443 | NOT RUN | `docker compose ps` / socket summary, sanitized |
| `.env`/tokens/keys/statements absent from Git | NOT RUN | secret-hygiene review reference |
| Logs/evidence contain no plaintext secrets | NOT RUN | review reference |

## Final Decision

V1 release tag remains **BLOCKED** until every required gate above is `PASS`, all CI gates correspond to the exact release commit, there are no P0/P1 data-correctness defects, and no unexplained financial delta exceeds 0.01 CNY.
