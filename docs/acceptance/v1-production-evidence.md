# V1 Production Acceptance Evidence

> 本文件是 production release 的证据索引，不保存原始财务数据、完整账户号、API key、token、密码、SSH private key 或未脱敏日志。
>
> 状态只允许：`PASS`、`FAIL`、`NOT RUN`、`BLOCKED`。没有真实执行证据时不得填写 `PASS`。

## Evidence model

- **Runtime target commit** 是当前 `main` 上准备进入真实生产验收的应用/部署 payload。当前固定为 `cbfba3268a9c747d874d84845910dca1f2c5657d`。
- PR #33 的 exact validated head 为 `7f01fd69b00312830b7dedabd235a1b0141b1896`，validated runtime tree 为 `65c4ad9486151249903444dddfa7beff9205a1a3`；该 head 的全部适用自动化 gate 通过后，以 exact-head guard 合并为上述 `main` runtime target。
- PR #31 的 runtime-image hardening exact validated head 为 `bec6d2354f72c319f3fadb40a5732ed7c841c638`。PR #33 未修改 Dockerfile、Compose、Caddy、依赖或 runtime-image 定义，因此其既有 build + real-container smoke 证据继续适用于未变化的 runtime-image surface。
- 只修改本证据文件、README、LICENSE 或其它纯治理元数据的后续 commit **不重新定义 runtime target**；否则记录证据本身会造成无限 SHA 递归。
- 任何影响应用代码、依赖、数据库 schema、Docker/Compose/Caddy、CI/acceptance 逻辑或运行时配置契约的变更，都必须选择新的 runtime target，并重新执行适用 gate。
- 公共/第三方 runtime image 的 CVE 扫描按项目策略**不作为 mandatory blocking PR/release gate**。但若通过任何渠道已经发现并确认存在**未处理的高严重度可达漏洞**，无论来源是 first-party 还是 third-party，仍必须阻塞 V1 release，直到完成修复或形成符合项目安全标准的处置结论。
- 不强制公共镜像扫描不等于忽略已知漏洞；不可变镜像/模块输入、依赖审计、repository-native 测试、真实容器 smoke、first-party security gate 与真实生产验收仍是必要证据。
- Issue #26 是当前 production acceptance 执行清单；本文件是最终 release decision 的证据总账。

## Release Candidate

| 字段 | 值 |
|---|---|
| Runtime target commit (`main`) | `cbfba3268a9c747d874d84845910dca1f2c5657d` |
| Validated runtime tree | `65c4ad9486151249903444dddfa7beff9205a1a3` |
| Validated PR exact head | `7f01fd69b00312830b7dedabd235a1b0141b1896` |
| Finance Core version/tag | `pre-release / not tagged` |
| ezBookkeeping version | `1.6.1` |
| PostgreSQL major | `18` |
| Caddy version | `2.11.4` |
| Production host | `REDACTED / NOT RUN` |
| Acceptance operator | `NOT RUN` |
| Acceptance completed at | `NOT RUN` |
| Production release decision | `BLOCKED` |

## A. CI / Reproducible Gates

本节当前应用/安全边界自动化证据来自 PR #33 exact head `7f01fd69b00312830b7dedabd235a1b0141b1896`。该 exact head 的 CI、MCP Security、Edge Security、OpenClaw Release Acceptance（含 Real OpenClaw）全部 PASS 后，以 exact-head guard 合并为 `main` runtime target `cbfba3268a9c747d874d84845910dca1f2c5657d`。

PR #33 未触发 path-scoped Runtime Images Security，因为它没有修改 Dockerfile、Compose、Caddy、依赖或 runtime-image 定义；未变化的 runtime-image surface 继续引用 PR #31 exact-head build + real-container smoke 证据。

- CI `32502408867`: PASS
- MCP Security `32502409066`: PASS
- Edge Security `32502408730`: PASS
- OpenClaw Release Acceptance `32502408707`: PASS — static contracts + Real OpenClaw acceptance
- Runtime Images Security `32470278120`: PASS — PR #31 unchanged runtime-image surface; hardened third-party runtime build + real-container smoke only

| Gate | Status | Evidence |
|---|---|---|
| migration up/down/up | PASS | CI `32502408867` / `make verify` |
| sqlc generated source clean | PASS | CI `32502408867` / `make verify` |
| gofmt / go vet / go test | PASS | CI `32502408867` / `make verify` |
| govulncheck (first-party Go dependency/code gate) | PASS | CI `32502408867` / `make verify` |
| PostgreSQL + domain integrations | PASS | CI `32502408867` / `make verify` |
| scheduler restart/idempotency integration | PASS | CI `32502408867` / `make verify` automated integration |
| backup restore drill | PASS | CI `32502408867` / disposable restore drill; **not** real off-host recovery |
| go race | PASS | CI `32502408867` / `make verify` |
| Go binary build | PASS | CI `32502408867` / `make verify` |
| frontend npm ci + unit | PASS | CI `32502408867` / `make verify` |
| PWA contract + typecheck + build | PASS | CI `32502408867` / `make verify` |
| Finance Core container build | PASS | CI `32502408867` / `make verify` |
| production operations contract | PASS | CI `32502408867` / `make verify` |
| third-party hardened runtime build + smoke | PASS | Runtime Images Security `32470278120`; PR #33 did not change this surface |
| public/third-party runtime image CVE scan | NOT RUN | Not mandatory by project policy. If a HIGH/CRITICAL vulnerability is nevertheless discovered and confirmed reachable, it remains release-blocking until handled |
| edge exposure/security workflow | PASS | Edge Security `32502408730` |
| MCP security workflow | PASS | MCP Security `32502409066` |
| Real OpenClaw ephemeral acceptance | PASS | OpenClaw `32502408707`; static acceptance contracts + real Ollama/OpenClaw/MCP run completed successfully |
| Finance API cross-origin unsafe-method protection | PASS | PR #33 exact head; POST/PUT/PATCH/DELETE shared request boundary; RED→GREEN regression coverage in CI `32502408867` |
| external LLM plaintext transport rejection | PASS | PR #33 exact head; external `http://` rejected, only IP-literal loopback HTTP permitted |
| LLM HTTPS→HTTP redirect downgrade protection | PASS | PR #33 exact head; redirect policy enforced per request, including caller-supplied client; RED→GREEN regression coverage in CI `32502408867` |

Automated PASS proves the repository-defined contracts on the exact validated runtime tree. It does **not** replace the real production host, real ledger, off-host disaster recovery, real external Advisor provider, or real-phone acceptance below。

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
| Restic off-site snapshot created | NOT RUN | Snapshot ID only |
| `restic check` passed | NOT RUN | Run from an authorized maintenance/recovery context; timestamp + sanitized result |
| Off-host restore from real snapshot | NOT RUN | Restore host identifier redacted |
| Restored ezBookkeeping/Finance Core healthy | NOT RUN | health/readiness result |
| Restored key data sampled | NOT RUN | counts/ranges/hashes only |
| RTO recorded | NOT RUN | Duration |
| Production host cannot destroy all off-site recovery points | NOT RUN | Require separated destructive retention/prune authority or equivalent immutable/append-only recovery boundary; production-host compromise must not be sufficient to delete every recovery point |

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
| `.env`/tokens/keys/statements absent from Git | NOT RUN | final real-environment secret-hygiene review reference; repository `.dockerignore`/security contracts are supporting evidence only |
| Logs/evidence contain no plaintext secrets | NOT RUN | final review reference |

## H. Release Governance

| Gate | Status | Evidence / Notes |
|---|---|---|
| Production acceptance tracker exists | PASS | GitHub Issue #26, updated to current runtime target `cbfba3268a9c747d874d84845910dca1f2c5657d` |
| Repository license selected | PASS | MIT; PR #27 merged |
| Runtime-image hardening | PASS | PR #31 merged exact validated head `bec6d2354f72c319f3fadb40a5732ed7c841c638`; unchanged by PR #33 |
| Runtime security remediation | PASS | PR #33 merged exact validated head `7f01fd69b00312830b7dedabd235a1b0141b1896` as `cbfba3268a9c747d874d84845910dca1f2c5657d` after RED→GREEN tests, required checks and resolved review findings |
| `main` branch protection / ruleset | PASS | Repository Ruleset active; GitHub API reports `main` as `protected: true` |
| Repository hygiene | PASS | Runtime/spike PR count is 0 and stale runtime/work/spike branches were removed. Evidence-only governance PR/branches are transient and removed after merge |
| Final first-party repository/security review | PASS | Repository-wide security review + PR #33 remediation/review follow-up; no unresolved review thread; exact-head CI/MCP/Edge/OpenClaw gates PASS |
| Known high-severity reachable vulnerabilities | PASS | No unresolved known reachable HIGH/CRITICAL finding is carried in the current release ledger; if one becomes known through any source, release returns to BLOCKED until handled |

## Final Decision

Production release remains **BLOCKED** until every required real-environment gate above is `PASS`, there are no unresolved P0/P1 security or data-correctness defects, no known unhandled high-severity reachable vulnerability remains, and no unexplained financial delta exceeds 0.01 CNY。公共/第三方 runtime image CVE 扫描本身不是 mandatory blocking gate，但已知且确认可达的高严重度漏洞仍然是 release blocker。
