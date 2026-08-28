# V1 Production Acceptance Evidence

> 本文件是 production release 的证据索引，不保存原始财务数据、完整账户号、API key、token、密码、TOTP secret、recovery code、Session token、SSH private key 或未脱敏日志。
>
> 状态只允许：`PASS`、`FAIL`、`NOT RUN`、`BLOCKED`。没有真实执行证据时不得填写 `PASS`。

## Evidence model

- **Runtime target commit** 是当前 `main` 上准备进入真实生产验收的已合并应用/部署 payload。当前已登记基线仍固定为 `12bee8c7ef9e78be6c4d39c2bee6f99177afca1f`；新的 runtime/security PR 在合并并完成治理更新前不得提前覆盖该值。
- PR #46 是 application-native Finance authentication 的候选实现。其自动化 CI/Auth/Edge/MCP/Runtime/OpenClaw 证据只证明候选代码线；**在 PR #46 合并到 `main` 并完成 exact-head/final-main 验证前，不得把它写成 production runtime target，也不得把真实生产认证项目改成 PASS。**
- PR #35 的 exact validated head 为 `0e7bd554bec3f7fe589a832aaee4a595c64be843`，validated runtime tree 为 `064599732156817a7600e85d8c3dd4d23906803c`；该 head 的 CI、MCP Security、Edge Security、OpenClaw Release Acceptance（含 Real OpenClaw）全部通过后，以 `expected_head_sha` guard 合并为上述已登记 runtime target。
- PR #33 的 Finance API CSRF / unsafe-method 与 external LLM transport remediation 已包含在该 runtime tree。
- PR #31 的 runtime-image hardening exact validated head 为 `bec6d2354f72c319f3fadb40a5732ed7c841c638`。
- 只修改本证据文件、README、LICENSE 或其它纯治理元数据的后续 commit **不重新定义 runtime target**；否则记录证据本身会造成无限 SHA 递归。
- 任何影响应用代码、依赖、数据库 schema、Docker/Compose/Caddy、CI/acceptance 逻辑或运行时配置契约的变更，都必须选择新的 runtime target，并重新执行适用 gate。
- 公共/第三方 runtime image 的 CVE 扫描按项目策略**不作为 mandatory blocking PR/release gate**。但若通过任何渠道已经发现并确认存在**未处理的高严重度可达漏洞**，仍必须阻塞 V1 release。
- Issue #26 是当前 production acceptance 执行清单；本文件是最终 release decision 的证据总账。

## Release Candidate

| 字段 | 值 |
|---|---|
| Runtime target commit (`main`) | `12bee8c7ef9e78be6c4d39c2bee6f99177afca1f`（待 PR #46 合并后的治理更新替换） |
| Validated runtime tree | `064599732156817a7600e85d8c3dd4d23906803c`（上一已登记基线） |
| Validated PR exact head | `0e7bd554bec3f7fe589a832aaee4a595c64be843`（上一已登记基线） |
| Finance Core version/tag | `pre-release / not tagged` |
| ezBookkeeping version | `1.6.1` |
| PostgreSQL major | `18` |
| Caddy version | `2.11.4` |
| Production host | `REDACTED / NOT RUN` |
| Acceptance operator | `NOT RUN` |
| Acceptance completed at | `NOT RUN` |
| Production release decision | `BLOCKED` |

## A. CI / Reproducible Gates

本节保存“已登记 runtime target”的历史自动化证据。PR #46 的 application-native-auth 自动化结果在合并前属于候选证据；合并后必须以最终 exact head、merge/main SHA 和对应 workflow run 做一次治理更新，不能在这里预先宣告 PASS。

上一已登记 runtime target 的自动化证据：

- CI `32646849476`: PASS
- MCP Security `32646849464`: PASS
- Edge Security `32646849457`: PASS
- OpenClaw Release Acceptance `32646849461`: PASS — static contracts + Real OpenClaw acceptance
- Runtime Images Security `32470278120`: PASS — PR #31 unchanged runtime-image surface; hardened third-party runtime build + real-container smoke only

| Gate | Status | Evidence |
|---|---|---|
| migration up/down/up | PASS | CI `32646849476` / previous registered runtime target |
| sqlc generated source clean | PASS | CI `32646849476` |
| gofmt / go vet / go test | PASS | CI `32646849476` |
| govulncheck (first-party Go dependency/code gate) | PASS | CI `32646849476` |
| PostgreSQL + domain integrations | PASS | CI `32646849476` |
| scheduler restart/idempotency integration | PASS | CI `32646849476` automated integration |
| backup restore drill | PASS | CI `32646849476` / disposable restore drill; **not** real off-host recovery |
| go race | PASS | CI `32646849476` |
| Go binary build | PASS | CI `32646849476` |
| frontend npm ci + unit | PASS | CI `32646849476` |
| PWA contract + typecheck + build | PASS | CI `32646849476` |
| Finance Core container build | PASS | CI `32646849476` |
| production operations contract | PASS | previous registered runtime target |
| append-only off-site backup repository contract | PASS | repository contract only; not proof of real backup-host deployment |
| third-party hardened runtime build + smoke | PASS | Runtime Images Security `32470278120` |
| public/third-party runtime image CVE scan | NOT RUN | Not mandatory by project policy; confirmed reachable HIGH/CRITICAL remains release-blocking |
| edge exposure/security workflow | PASS | Edge Security `32646849457` |
| MCP security workflow | PASS | MCP Security `32646849464` |
| Real OpenClaw ephemeral acceptance | PASS | OpenClaw `32646849461` |
| Finance API cross-origin unsafe-method protection | PASS | PR #33 control in previous registered runtime target |
| external LLM plaintext transport rejection | PASS | PR #33 control in previous registered runtime target |
| LLM HTTPS→HTTP redirect downgrade protection | PASS | PR #33 control in previous registered runtime target |
| Finance application-native authentication gate | BLOCKED | Candidate PR #46 must be merged and final exact-head/main evidence recorded before this row can become PASS |

Automated PASS proves repository-defined contracts on a specific validated runtime tree. It does **not** replace the real production host, real ledger, real Finance TOTP enrollment, real ezBookkeeping owner 2FA enrollment, real append-only backup service, off-host disaster recovery, real external Advisor provider, or real-phone acceptance below。

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
| Repository-side append-only recovery boundary | PASS | repository evidence only, not proof of real backup-host deployment |
| Real production backup created | NOT RUN | Backup timestamp and SHA256SUMS hash |
| Authenticated append-only REST off-site snapshot created | NOT RUN | Snapshot ID only; producer must use `rest:https://...` |
| Producer credential cannot delete/overwrite existing recovery points | NOT RUN | Record destructive-operation rejection; no secret or raw repository contents |
| Authorized maintenance retention/prune authority verified | NOT RUN | Run from independent backup/maintenance host; sanitized result only |
| `restic check` passed | NOT RUN | Authorized maintenance/recovery context; timestamp + sanitized result |
| Off-host restore from real snapshot | NOT RUN | Restore host/environment identifier redacted |
| Restored ezBookkeeping/Finance Core healthy | NOT RUN | health/readiness result |
| Restored key data sampled | NOT RUN | counts/ranges/hashes only |
| RTO recorded | NOT RUN | Duration |

## F. Scheduler Restart

| Gate | Status | Evidence / Notes |
|---|---|---|
| Restart after successful monthly run creates no duplicate | NOT RUN | automated integration is supporting evidence; real deployment restart still required |
| Interrupted `running` recovery exercised | NOT RUN | controlled production-like status only |
| Retried run succeeds or fails with controlled error code | NOT RUN | no raw provider/secret-bearing error text |

## G. Mobile / Authentication / Edge / Secret Hygiene

| Gate | Status | Evidence / Notes |
|---|---|---|
| PWA installed on real phone over HTTPS | NOT RUN | Device family/browser version, no device ID |
| Finance app-native password + mandatory TOTP login verified on production edge | NOT RUN | Record timestamp, deployed commit/version, operator and PASS/FAIL only; dashboard must remain unavailable before TOTP completion |
| Finance logout / revoked-session / CSRF boundary verified on production | NOT RUN | Record sanitized status-code assertions only; no Session or CSRF material |
| Finance household authorization verified on production | NOT RUN | Browser query/body override must not escape the authenticated Session household |
| Browser Session and MCP Bearer separation verified | NOT RUN | Browser cookie must not authenticate `/mcp`; MCP Bearer must not authenticate browser API |
| **ezBookkeeping owner 2FA enrollment confirmed** | NOT RUN | `EBK_AUTH_ENABLE_TWO_FACTOR=true` only enables the feature and is **not** proof that the owner enrolled it; record a dated redacted assertion |
| ezBookkeeping steady-state registration disabled | NOT RUN | PASS/FAIL only; do not record credentials |
| Only Caddy exposes host 80/443 on production host | NOT RUN | `docker compose ps` / socket summary, sanitized |
| Application secret files are outside Git and private | NOT RUN | Verify Finance auth key/admin password/EBK token/EBK signing secret and optional MCP token permissions without recording values |
| `.env`/tokens/keys/statements absent from Git | NOT RUN | final real-environment secret-hygiene review reference |
| Logs/evidence contain no plaintext secrets | NOT RUN | include password/TOTP/recovery/Session/API token checks in final review |

## H. Release Governance

| Gate | Status | Evidence / Notes |
|---|---|---|
| Production acceptance tracker exists | PASS | GitHub Issue #26 |
| Repository license selected | PASS | MIT; PR #27 merged |
| Runtime-image hardening | PASS | PR #31 historical validated evidence |
| Runtime security remediation | PASS | PR #33 historical validated evidence |
| Append-only off-site backup boundary | PASS | PR #35 historical validated repository evidence |
| Application-native Finance authentication | BLOCKED | PR #46 candidate must pass final exact-head gates, merge to `main`, and receive governance evidence refresh |
| `main` branch protection / ruleset | PASS | Repository Ruleset active; GitHub API reports `main` as protected |
| Final first-party repository/security review | BLOCKED | Must be rerun on PR #46 final head / merged runtime before release governance can mark it PASS |
| Known high-severity reachable vulnerabilities | PASS | No unresolved known reachable HIGH/CRITICAL finding is currently carried; any newly confirmed reachable finding returns release to BLOCKED |

## Final Decision

Production release remains **BLOCKED** until the application-native authentication candidate is merged and its final automated evidence is recorded, every required real-environment gate above is `PASS`, there are no unresolved P0/P1 security or data-correctness defects, no known unhandled high-severity reachable vulnerability remains, and no unexplained financial delta exceeds 0.01 CNY。公共/第三方 runtime image CVE 扫描本身不是 mandatory blocking gate，但已知且确认可达的高严重度漏洞仍然是 release blocker。
