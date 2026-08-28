# V1 Production Acceptance Evidence

> 本文件是 production release 的证据索引，不保存原始财务数据、完整账户号、API key、token、密码、TOTP secret、recovery code、Session token、SSH private key 或未脱敏日志。
>
> 状态只允许：`PASS`、`FAIL`、`NOT RUN`、`BLOCKED`。没有真实执行证据时不得填写 `PASS`。

## Evidence model

- **Runtime target commit** 是当前 `main` 上准备进入真实生产验收的已合并应用/部署 payload。当前固定为 `5d123301caff3e2e7fce0c45084b5d4a0b19ab52`。
- 当前 validated runtime tree 为 `c0afdf90cb344558813cb01c97f887e9dceccec6`，PR #50 exact validated head 为 `2849767088871d0c3de669bbb33ed5d429bf0d83`。
- PR #50 新增 fail-closed 的 `scripts/acceptance/v1-production-live.sh` 和 `docs/acceptance/v1-production-runbook.md`，把可自动判断的真实生产证据采集与必须依赖真实用户/真实财务数据/物理手机/独立恢复环境的人工门禁明确分离。该 acceptance-contract 变更会影响 release 判定，因此重新定义 runtime target。
- PR #46 的 application-native Finance authentication、PR #35 的 append-only backup boundary、PR #33 的 Finance API unsafe-method / external LLM transport remediation、PR #31 的 runtime-image hardening均继续包含在当前 runtime tree。
- 只修改本证据文件、README、LICENSE、Issue 或其它纯治理元数据的后续 commit **不重新定义 runtime target**；否则记录证据本身会造成无限 SHA 递归。
- 任何影响应用代码、依赖、数据库 schema、Docker/Compose/Caddy、CI/acceptance 逻辑或运行时配置契约的变更，都必须选择新的 runtime target，并重新执行适用 gate。
- 公共/第三方 runtime image 的 CVE 扫描按项目策略**不作为 mandatory blocking PR/release gate**。但若任何渠道发现并确认存在**未处理的高严重度可达漏洞**，仍必须阻塞 V1 release。
- Issue #26 是 production acceptance 执行清单；本文件是最终 release decision 的证据总账。

## Release Candidate

| 字段 | 值 |
|---|---|
| Runtime target commit (`main`) | `5d123301caff3e2e7fce0c45084b5d4a0b19ab52` |
| Validated runtime tree | `c0afdf90cb344558813cb01c97f887e9dceccec6` |
| Validated PR exact head | `2849767088871d0c3de669bbb33ed5d429bf0d83` |
| Finance Core version/tag | `pre-release / not tagged` |
| ezBookkeeping version | `1.6.1` |
| PostgreSQL major | `18` |
| Caddy version | `2.11.4` |
| Production host | `REDACTED / NOT RUN` |
| Acceptance operator | `NOT RUN` |
| Acceptance completed at | `NOT RUN` |
| Production release decision | `BLOCKED` |

## A. CI / Reproducible Gates

PR #50 exact validated head `2849767088871d0c3de669bbb33ed5d429bf0d83`：

- CI `33142026844` / #777: PASS
- Edge Security `33142026836` / #635: PASS
- MCP Security `33142026815` / #459: PASS
- OpenClaw Release Acceptance `33142026806` / #404: PASS，包含 Real OpenClaw + local Ollama + MCP acceptance
- Runtime Images Security: 未重新触发。PR #50 未修改其 path filter 覆盖的 Dockerfile、Compose、Caddy 或 runtime-image hardening surface；此前 `main` Runtime Images Security #61 的 hardened image build + real-container smoke 继续适用。

该 exact head 使用 `expected_head_sha=2849767088871d0c3de669bbb33ed5d429bf0d83` guard 合并为 `main` runtime target `5d123301caff3e2e7fce0c45084b5d4a0b19ab52`。合并后 `main` push：

- CI `33143480030` / #778: PASS
- Edge Security `33143480018` / #636: PASS
- MCP Security `33143480005` / #460: PASS
- OpenClaw Release Acceptance `33143480071` / #405: PASS（push 分支执行 repository contracts；Real OpenClaw 证据来自同一 runtime tree 的 exact-head #404）

| Gate | Status | Evidence |
|---|---|---|
| migration up/down/up | PASS | CI #778 / `make verify` |
| sqlc generated source clean | PASS | CI #778 / `make verify` |
| `go mod tidy` clean | PASS | CI #778 / `make verify` |
| gofmt / go vet / go test | PASS | CI #778 / `make verify` |
| govulncheck (first-party Go dependency/code gate) | PASS | CI #778 / `make verify` |
| PostgreSQL + domain integrations | PASS | CI #778 / `make verify` |
| scheduler restart/idempotency integration | PASS | CI #778 / automated integration |
| backup restore drill | PASS | CI #778 / disposable restore drill; **not** real off-host recovery |
| go race | PASS | CI #778 / `make verify` |
| Go binary build | PASS | CI #778 / `make verify` |
| frontend npm ci + unit | PASS | CI #778 / `make verify` |
| PWA contract + typecheck + build | PASS | CI #778 / `make verify` |
| Finance application-native authentication gate | PASS | current tree retains PR #46 auth runtime; CI #778 re-runs repository auth/security verification |
| Finance API fail-closed without BrowserAuth | PASS | PR #46 regression + current CI #778 |
| password/TOTP/recovery/session persistence and transaction boundaries | PASS | PR #46 integration/race tests + current CI #778 |
| login + second-factor throttling | PASS | PR #46 tests + current CI #778 |
| production operations / secret-file contract | PASS | CI #778; repository preflight/production operations contracts |
| V1 production live acceptance runner contract | PASS | PR #50 RED→GREEN + CI #777/#778; runner records only objective automated gates as PASS and preserves real/manual gates as `NOT_RUN` |
| production acceptance runbook present | PASS | `docs/acceptance/v1-production-runbook.md` in runtime target |
| real production live runner execution | NOT RUN | Requires actual application host, real HTTPS origins and external evidence directory |
| append-only off-site backup repository contract | PASS | current runtime tree retains PR #35 producer/maintenance authority split; **not** proof of real backup-host deployment |
| third-party hardened runtime build + smoke | PASS | Runtime Images Security #61; unchanged runtime-image surface |
| public/third-party runtime image CVE scan | NOT RUN | Not mandatory by project policy; confirmed reachable HIGH/CRITICAL remains release-blocking |
| edge exposure/security workflow | PASS | Edge #635 exact-head + #636 post-merge |
| Caddy no longer owns Finance user authentication | PASS | application-native auth boundary retained; Caddy is TLS/reverse-proxy only |
| ezBookkeeping reference deployment hardening contract | PASS | current Edge/CI contracts retain trusted proxy, API-token scope, 2FA feature, rate limits, registration and secret-file hardening |
| MCP security workflow | PASS | MCP #459 exact-head + #460 post-merge |
| Browser Session / MCP Bearer separation | PASS | repository auth/MCP tests; current CI/MCP gates |
| Real OpenClaw ephemeral acceptance | PASS | exact-head OpenClaw #404; real Ollama/OpenClaw/MCP run on validated runtime tree |
| Finance API cross-origin unsafe-method protection | PASS | current CI #778 / auth+CSRF tests |
| external LLM plaintext transport rejection | PASS | current CI #778 |
| LLM HTTPS→HTTP redirect downgrade protection | PASS | current CI #778 |

Automated PASS proves repository-defined contracts on the validated runtime tree. It does **not** replace the real production host, real ledger, real Finance TOTP enrollment, real ezBookkeeping owner 2FA enrollment, real append-only backup service, off-host disaster recovery, real external Advisor provider, or real-phone acceptance below。

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
| Repository-side append-only recovery boundary | PASS | current runtime tree retains separate producer and maintenance authority; repository evidence only |
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
| Finance logout / revoked-session / CSRF boundary verified on production | NOT RUN | Sanitized status-code assertions only |
| Finance household authorization verified on production | NOT RUN | Browser query/body override must not escape authenticated Session household |
| Browser Session and MCP Bearer separation verified on production | NOT RUN | Browser cookie must not authenticate `/mcp`; MCP Bearer must not authenticate browser API |
| **ezBookkeeping owner 2FA enrollment confirmed** | NOT RUN | Feature enablement alone is not proof of owner enrollment |
| ezBookkeeping steady-state registration disabled on production | NOT RUN | PASS/FAIL only; do not record credentials |
| Only Caddy exposes host 80/443 on production host | NOT RUN | Live runner can collect sanitized Compose/runtime evidence once the host is available |
| Finance `/healthz` and `/readyz` verified over production HTTPS | NOT RUN | Live runner supports this check but has not been executed on the real host |
| Finance manifest/service-worker verified over production HTTPS | NOT RUN | Live runner supports this check but has not been executed on the real host |
| Unauthenticated production Dashboard returns 401 | NOT RUN | Live runner supports this check but has not been executed on the real host |
| Application secret files are outside Git and private on production host | NOT RUN | Verify permissions without recording values |
| `.env`/tokens/keys/statements absent from Git and deployment artifacts | NOT RUN | final real-environment secret-hygiene review reference |
| Logs/evidence contain no plaintext secrets | NOT RUN | include password/TOTP/recovery/Session/API-token checks in final review |

## H. Release Governance

| Gate | Status | Evidence / Notes |
|---|---|---|
| Production acceptance tracker exists | PASS | GitHub Issue #26 |
| Repository license selected | PASS | MIT; PR #27 merged |
| Runtime-image hardening | PASS | PR #31 historical controls retained; Runtime Images Security #61 passed build + real-container smoke |
| Runtime security remediation | PASS | PR #33 historical controls retained and revalidated by current CI/security gates |
| Append-only off-site backup boundary | PASS | PR #35 historical repository controls retained |
| Application-native Finance authentication | PASS | PR #46 historical runtime retained; current CI/security gates revalidate repository contracts |
| V1 live production acceptance orchestration | PASS | PR #50 exact head all applicable gates PASS; merged with expected-head guard; `main` CI #778 / Edge #636 / MCP #460 / OpenClaw #405 PASS |
| `main` branch protection / ruleset | PASS | GitHub reports `main` as protected |
| Final first-party repository/security review for current runtime | PASS | PR #50 review added strict credential-free HTTPS-origin validation and preserved human-only gates as `NOT_RUN`; exact-head/post-merge gates PASS |
| Known high-severity reachable vulnerabilities | PASS | No unresolved known reachable HIGH/CRITICAL finding is currently carried; any newly confirmed reachable finding returns release to BLOCKED |

## Final Decision

The current V1 runtime target `5d123301caff3e2e7fce0c45084b5d4a0b19ab52` has passed its repository-defined exact-head and post-merge automated gates. Production release remains **BLOCKED** until every required real-environment gate above is `PASS`, including execution of the live acceptance runner on the actual production host, real-phone HTTPS/PWA use, Finance owner password+TOTP enrollment on the actual edge, ezBookkeeping owner 2FA enrollment, real statement reconciliation, real backup/off-host restore, real external Advisor provider verification, production secret hygiene, scheduler restart/recovery exercises, and the remaining operational checks. There must also be no unresolved P0/P1 security or data-correctness defect, no known unhandled high-severity reachable vulnerability, and no unexplained financial delta greater than 0.01 CNY。公共/第三方 runtime image CVE 扫描本身不是 mandatory blocking gate，但已知且确认可达的高严重度漏洞仍然是 release blocker。
