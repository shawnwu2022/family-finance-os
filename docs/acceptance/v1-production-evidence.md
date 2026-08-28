# V1 Production Acceptance Evidence

> 本文件是 production release 的证据索引，不保存原始财务数据、完整账户号、API key、token、密码、TOTP secret、recovery code、Session token、SSH private key 或未脱敏日志。
>
> 状态只允许：`PASS`、`FAIL`、`NOT RUN`、`BLOCKED`。没有真实执行证据时不得填写 `PASS`。

## Evidence model

- **Runtime target commit** 是当前 `main` 上准备进入真实生产验收的已合并应用/部署 payload。当前固定为 `c74d586d811ad6da4437f8d8803074eb8799421b`。
- PR #46 的 application-native Finance authentication exact validated head 为 `09574f0ed5e3e5b08254ab11fe2961102e4d4e9d`，validated runtime tree 为 `8d48841f3f82ceeb2ee71656c33846a75111e02b`。该 head 的 CI、Runtime Images Security、Edge Security、MCP Security、OpenClaw Release Acceptance 全部通过后，以 `expected_head_sha` guard 合并为上述 `main` runtime target；合并后的同一 runtime tree 又通过了一轮 `main` push 验证。
- PR #46 同时包含 Finance Core application-native password + mandatory TOTP、server-side Session、CSRF、household authorization、browser/MCP credential separation、trusted-proxy login throttling、second-factor throttling、secret-file deployment、ezBookkeeping hardening，以及 Caddy 退出用户认证职责。
- PR #35 的 append-only backup boundary、PR #33 的 Finance API unsafe-method / external LLM transport remediation、PR #31 的 runtime-image hardening均继续包含在当前 runtime tree；PR #46 的 exact-head 与 post-merge 全量 gate 已重新验证组合后的当前树。
- 只修改本证据文件、README、LICENSE 或其它纯治理元数据的后续 commit **不重新定义 runtime target**；否则记录证据本身会造成无限 SHA 递归。
- 任何影响应用代码、依赖、数据库 schema、Docker/Compose/Caddy、CI/acceptance 逻辑或运行时配置契约的变更，都必须选择新的 runtime target，并重新执行适用 gate。
- 公共/第三方 runtime image 的 CVE 扫描按项目策略**不作为 mandatory blocking PR/release gate**。但若通过任何渠道已经发现并确认存在**未处理的高严重度可达漏洞**，仍必须阻塞 V1 release。
- Issue #26 是当前 production acceptance 执行清单；本文件是最终 release decision 的证据总账。

## Release Candidate

| 字段 | 值 |
|---|---|
| Runtime target commit (`main`) | `c74d586d811ad6da4437f8d8803074eb8799421b` |
| Validated runtime tree | `8d48841f3f82ceeb2ee71656c33846a75111e02b` |
| Validated PR exact head | `09574f0ed5e3e5b08254ab11fe2961102e4d4e9d` |
| Finance Core version/tag | `pre-release / not tagged` |
| ezBookkeeping version | `1.6.1` |
| PostgreSQL major | `18` |
| Caddy version | `2.11.4` |
| Production host | `REDACTED / NOT RUN` |
| Acceptance operator | `NOT RUN` |
| Acceptance completed at | `NOT RUN` |
| Production release decision | `BLOCKED` |

## A. CI / Reproducible Gates

PR #46 exact validated head `09574f0ed5e3e5b08254ab11fe2961102e4d4e9d` 的候选验证：

- CI `33092292158` / #764: PASS
- Runtime Images Security `33092292157` / #60: PASS
- Edge Security `33092292179` / #622: PASS
- MCP Security `33092292159` / #446: PASS
- OpenClaw Release Acceptance `33092292155` / #391: PASS

该 exact head 使用 `expected_head_sha=09574f0ed5e3e5b08254ab11fe2961102e4d4e9d` guard 合并为 `main` runtime target `c74d586d811ad6da4437f8d8803074eb8799421b`。合并后 `main` push 的再次验证：

- CI `33129014410` / #765: PASS
- Runtime Images Security `33129014440` / #61: PASS — hardened image build + real-container smoke
- Edge Security `33129014408` / #623: PASS
- MCP Security `33129014445` / #447: PASS
- OpenClaw Release Acceptance `33129014426` / #392: PASS

| Gate | Status | Evidence |
|---|---|---|
| migration up/down/up | PASS | CI #765 / `make verify` |
| sqlc generated source clean | PASS | CI #765 / `make verify` |
| `go mod tidy` clean | PASS | CI #765 / `make verify` |
| gofmt / go vet / go test | PASS | CI #765 / `make verify` |
| govulncheck (first-party Go dependency/code gate) | PASS | CI #765 / `make verify` |
| PostgreSQL + domain integrations | PASS | CI #765 / `make verify` |
| scheduler restart/idempotency integration | PASS | CI #765 / automated integration |
| backup restore drill | PASS | CI #765 / disposable restore drill; **not** real off-host recovery |
| go race | PASS | CI #765 / `make verify` |
| Go binary build | PASS | CI #765 / `make verify` |
| frontend npm ci + unit | PASS | CI #765 / `make verify` |
| PWA contract + typecheck + build | PASS | CI #765 / `make verify` |
| Finance application-native authentication gate | PASS | CI #765 / repository-native `verify-auth-security`: password → mandatory TOTP enrollment → Session → Dashboard → CSRF/household isolation → logout/revoked 401, plus session expiry/replay/recovery tests |
| Finance API fail-closed without BrowserAuth | PASS | PR #46 Task 8 RED→GREEN regression test; final exact-head CI #764 and post-merge CI #765 |
| password/TOTP/recovery/session persistence and transaction boundaries | PASS | PR #46 integration/race tests; CI #765 |
| login + second-factor throttling | PASS | PR #46 final exact head includes password and TOTP/enrollment throttling tests; CI #764/#765 |
| production operations / secret-file contract | PASS | CI #765; preflight rejects legacy plaintext auth/EBK secret variables and requires external private secret files |
| append-only off-site backup repository contract | PASS | current runtime tree retains repository contract; **not** proof of real backup-host deployment |
| third-party hardened runtime build + smoke | PASS | Runtime Images Security #61 |
| public/third-party runtime image CVE scan | NOT RUN | Not mandatory by project policy; confirmed reachable HIGH/CRITICAL remains release-blocking |
| edge exposure/security workflow | PASS | Edge Security #623 |
| Caddy no longer owns Finance user authentication | PASS | Edge Security #623; Finance Core is fail-closed and Caddy is TLS/reverse-proxy only |
| ezBookkeeping reference deployment hardening contract | PASS | Edge/CI gates: trusted Caddy proxy `/32`, API token limited to Finance Core `.30`, feature 2FA enabled, rate limits nonzero, steady-state registration disabled, secret-file wrapper fail-closed |
| MCP security workflow | PASS | MCP Security #447 |
| Browser Session / MCP Bearer separation | PASS | auth security tests + MCP #447 / CI #765 |
| Real OpenClaw ephemeral acceptance | PASS | OpenClaw #392; static contracts + real Ollama/OpenClaw/MCP run |
| Finance API cross-origin unsafe-method protection | PASS | current tree; CI #765 / auth+CSRF tests |
| external LLM plaintext transport rejection | PASS | current tree; CI #765 |
| LLM HTTPS→HTTP redirect downgrade protection | PASS | current tree; CI #765 |

Automated PASS proves repository-defined contracts on the exact validated runtime tree. It does **not** replace the real production host, real ledger, real Finance TOTP enrollment, real ezBookkeeping owner 2FA enrollment, real append-only backup service, off-host disaster recovery, real external Advisor provider, or real-phone acceptance below。

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
| Repository-side append-only recovery boundary | PASS | current runtime tree retains separate producer and maintenance authority; repository evidence only, not proof of real backup-host deployment |
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
| Browser Session and MCP Bearer separation verified on production | NOT RUN | Browser cookie must not authenticate `/mcp`; MCP Bearer must not authenticate browser API |
| **ezBookkeeping owner 2FA enrollment confirmed** | NOT RUN | `EBK_AUTH_ENABLE_TWO_FACTOR=true` only enables the feature and is **not** proof that the owner enrolled it; record a dated redacted assertion |
| ezBookkeeping steady-state registration disabled on production | NOT RUN | PASS/FAIL only; do not record credentials |
| Only Caddy exposes host 80/443 on production host | NOT RUN | `docker compose ps` / socket summary, sanitized |
| Application secret files are outside Git and private on production host | NOT RUN | Verify Finance auth key/admin password/EBK token/EBK signing secret and optional MCP token permissions without recording values |
| `.env`/tokens/keys/statements absent from Git and deployment artifacts | NOT RUN | final real-environment secret-hygiene review reference |
| Logs/evidence contain no plaintext secrets | NOT RUN | include password/TOTP/recovery/Session/API token checks in final review |

## H. Release Governance

| Gate | Status | Evidence / Notes |
|---|---|---|
| Production acceptance tracker exists | PASS | GitHub Issue #26 |
| Repository license selected | PASS | MIT; PR #27 merged |
| Runtime-image hardening | PASS | PR #31 historical controls retained; current Runtime Images Security #61 passed build + real-container smoke |
| Runtime security remediation | PASS | PR #33 historical controls retained and revalidated by current CI/security gates |
| Append-only off-site backup boundary | PASS | PR #35 historical repository controls retained and revalidated in current tree |
| Application-native Finance authentication | PASS | PR #46 exact head `09574f0e…` all 5 gates PASS; merged with expected-head guard as `c74d586d…`; post-merge `main` CI #765 / Runtime #61 / Edge #623 / MCP #447 / OpenClaw #392 all PASS |
| `main` branch protection / ruleset | PASS | GitHub reports `main` as protected |
| Final first-party repository/security review for auth runtime | PASS | Task 8 security-diff review identified and removed the missing-BrowserAuth unauthenticated fallback via RED→GREEN; final exact-head and post-merge gates PASS |
| Known high-severity reachable vulnerabilities | PASS | No unresolved known reachable HIGH/CRITICAL finding is currently carried; any newly confirmed reachable finding returns release to BLOCKED |

## Final Decision

The application-native authentication runtime is now merged into `main` and its repository-defined automated gates are **PASS**. Production release nevertheless remains **BLOCKED** until every required real-environment gate above is `PASS`, including real-phone HTTPS/PWA use, Finance owner password+TOTP enrollment on the actual edge, ezBookkeeping owner 2FA enrollment, real statement reconciliation, real backup/off-host restore, real external Advisor provider verification, production secret hygiene, and the remaining operational checks. There must also be no unresolved P0/P1 security or data-correctness defect, no known unhandled high-severity reachable vulnerability, and no unexplained financial delta greater than 0.01 CNY。公共/第三方 runtime image CVE 扫描本身不是 mandatory blocking gate，但已知且确认可达的高严重度漏洞仍然是 release blocker。
