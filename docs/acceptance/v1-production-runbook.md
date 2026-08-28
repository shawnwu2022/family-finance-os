# V1 Production Acceptance Runbook

本文用于执行 **真实生产环境** 的 V1 release acceptance。它不替代 `docs/acceptance/v1-production-evidence.md`，也不允许把 CI、fixture 或模拟结果写成真实生产 `PASS`。

## 1. 前提

在 application host 上：

- checkout 必须指向准备验收的 release candidate commit；
- `.env` 和所有 secret file 已按 `docs/07-operations.md` 配置，且不在 Git；
- ezBookkeeping 与 Finance Core 已通过 Caddy 的真实 HTTPS 域名提供服务；
- `docker compose` 能看到当前生产 project；
- evidence 目录必须位于仓库外，并由运维账户保护。

真实账单、完整账户号、密码、TOTP secret、recovery code、Session、API token、LLM key、restic password、SSH private key 均不得写入 evidence 目录或 Git。

## 2. 自动化生产采集

先创建仓库外 evidence 根目录，例如：

```bash
sudo install -d -m 0700 /var/lib/family-finance/acceptance
```

执行：

```bash
sudo env \
  V1_PRODUCTION_EVIDENCE_DIR=/var/lib/family-finance/acceptance \
  FINANCE_PUBLIC_URL=https://finance.example.com \
  EBK_PUBLIC_URL=https://book.example.com \
  bash scripts/acceptance/v1-production-live.sh
```

如需要固定运行标识，可额外设置：

```text
V1_PRODUCTION_RUN_ID=2026-08-28-prod-rc1
```

runner 会 fail-closed，并自动验证/采集：

- 当前 tracked checkout 无修改；
- repository-defined `scripts/preflight.sh`；
- repository-defined edge static contract；
- `docker compose ps` 运行拓扑；
- 运行中只有 Caddy 发布 host ports，且包含 80/tcp、443/tcp、443/udp；
- Finance `/healthz` 通过真实 HTTPS；
- Finance `/readyz` 通过真实 HTTPS；
- ezBookkeeping 根页面通过真实 HTTPS；
- Finance PWA manifest 与 service worker 可通过 HTTPS 获取；
- 未认证访问 Finance Dashboard 返回 401；
- 所有采集文件生成 `SHA256SUMS`。

runner 只会把上述可客观自动判断的项目记录为 `PASS`。任何需要真实用户凭据、财务事实、物理手机或独立恢复环境的项目，都会保留为 `NOT_RUN`。

## 3. runner 输出

每次运行创建一个独立目录：

```text
<V1_PRODUCTION_EVIDENCE_DIR>/<run-id>/
  metadata.tsv
  status.tsv
  SHA256SUMS
  raw/
    preflight.txt
    edge-static.txt
    compose-ps.txt
    runtime-ports.tsv
    finance-healthz.txt
    finance-readyz.txt
    ezbookkeeping-root.html
    manifest.webmanifest
    sw.js
```

提交到 Git 的只能是**脱敏后的结论、时间、版本/commit、hash 和 PASS/FAIL/NOT_RUN 状态**。不要直接提交 `raw/`。

## 4. 仍需人工或独立环境执行的门禁

runner 明确不会自动完成以下 release blockers：

- Finance 真实 password → TOTP → Session → Dashboard → logout；
- CSRF、household authorization、Session revoke、Browser/MCP credential separation 的生产验证；
- ezBookkeeping owner 已真正完成 2FA enrollment；
- 至少一种真实中国账单导入；
- 至少一个完整自然月的 Income / Expense / Net Cashflow / Net Worth 独立 reconciliation，未解释差异 ≤ 0.01 CNY；
- Safe-to-Spend / Debt / Scenario 独立计算交叉验证；
- 真实 production Advisor provider/model 与 outage fallback；
- 真实 backup、append-only destructive denial、maintenance、`restic check`；
- 独立主机/环境 restore 与 RTO；
- Scheduler production restart / interrupted-run recovery；
- 真手机通过 HTTPS 安装并使用 PWA；
- 最终 secret hygiene review。

这些项目只能在真实执行后更新 `docs/acceptance/v1-production-evidence.md`。

## 5. 发布判定

完成一次 runner 不代表 production-ready。V1 tag 仍必须满足 `docs/08-testing-acceptance.md` 与 `docs/acceptance/v1-production-evidence.md` 的全部 required gates，并且不存在未解决 P0/P1 数据正确性或认证/授权缺陷、已知未处理的高严重度可达漏洞、secret 泄露或 > 0.01 CNY 的未解释财务差异。
