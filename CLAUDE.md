# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

Family Finance OS：面向中国家庭的自托管 AI 智能财务管家。核心原则：**记账事实可靠、财务计算确定、AI 负责解释与规划**。Go 模块化单体 + Vue 3 PWA + PostgreSQL + ezBookkeeping 账本 + Caddy 边缘代理。

## 常用命令

### Go（finance-core）

```bash
make test              # go test ./...
make test-race         # go test -race ./...
make fmt               # gofmt -w cmd internal
make fmt-check         # CI 风格的 gofmt 检查
make vet               # go vet ./...
make build             # CGO_ENABLED=0 构建 build/finance-core

# 运行单个测试
go test ./internal/debt/ -run TestSimulatorName
```

集成测试默认 skip，需要设置 `TEST_POSTGRES_HOST`（及可选 `TEST_POSTGRES_PORT`）才会运行；完整集成验证走 `make verify-go`。

### Web 前端（web/）

```bash
cd web
npm ci --ignore-scripts
npm test               # node --test src/**/*.test.mjs
npm run typecheck      # vue-tsc -b
npm run build          # vue-tsc -b && vite build
npm run check:pwa      # PWA contract 检查
```

### 仓库原生质量门禁（需要 Docker Engine + Compose v2）

```bash
make verify                    # 完整门禁：迁移 up/down/up、备份恢复演练、Go 全套、Web、MCP/Edge 安全、容器构建
make verify-contract           # 快速架构契约检查，无需 Docker
make verify-go                 # Go 栈 + PostgreSQL（锁定工具版本）
make verify-web                # Node 24.19.0 容器内执行前端全量检查
make verify-mcp-security       # MCP 安全测试
make verify-edge-security      # Edge/Caddy 安全契约
make verify-container          # 生产容器镜像构建验证
```

质量门禁的规范定义由仓库内脚本持有（`scripts/ci/*.sh`），`.github/workflows/*.yml` 只是镜像入口。

### SQLC 与迁移

```bash
# Schema 改动流程：改 db/migrations 新增 goose 迁移 → 在 db/queries 写查询 → 重新生成 sqlc
sqlc generate   # 生成到 internal/store/sqlc，生成后不允许有未提交 diff
```

- 迁移：`db/migrations/*.sql`（goose 格式，顺序编号）
- 查询：`db/queries/*.sql`
- 生成代码：`internal/store/sqlc`（pgx/v5），不要手改

## 架构大图

### 双权威源模型（最重要的设计约束）

- **ezBookkeeping 是交易账本唯一权威源**。Finance Core 通过 HTTP API（Bearer Token）读取账户/分类/交易，绝不维护第二套可编辑账本。适配层在 `internal/ledger/`（`port.go` 定义 `Ledger` 接口，`ezbookkeeping/client.go` 实现）。
- **Finance Core 是规划域权威源**：家庭画像、预算、债务合同、目标、资产快照、确定性计算。
- **LLM 绝不是数字权威源**。所有关键金额来自 Finance Engine 的结构化 Tool Result；AI 只做解释与编排。

### 分层链路

```
HTTP/MCP 请求
  → internal/server        (net/http mux，DTO 序列化，安全中间件 secureAPIRequests)
  → internal/appapi        (应用 API：Planner/AdvisorRunner 等接口编排)
  → internal/analytics     (cashflow/networth/normalization/quality 确定性计算)
  → internal/{budget,debt,goals,portfolio,scenario}  (领域服务)
  → internal/store/sqlc    (pgx/v5 生成代码)
  → internal/ledger        → ezBookkeeping HTTP API
```

Agent 通道独立成链：`internal/agentadapter`（typed tools 编排 + audit）→ `internal/mcpadapter`（MCP Streamable HTTP + Bearer/Origin/限流/并发/超时边界）→ `/mcp` 路由。12 个 Finance tools 定义在 `internal/appapi/advisor_tools.go`、`finance_tools.go`。

AI 链路：`internal/llm`（OpenAI-compatible provider + SSE）→ `internal/advisor`（policy/tool routing）。模型通过角色配置（`LLM_FAST_MODEL`/`LLM_PLANNER_MODEL`/`LLM_REVIEWER_MODEL`），**禁止把具体模型 ID 写进业务逻辑分支**。

其他关键包：
- `cmd/finance-core` — 入口，子命令 serve/healthcheck/migrate/bootstrap
- `internal/config` — 环境变量配置加载
- `internal/scheduler` — 月报等定时任务，靠 `job_runs` 表保证重启幂等
- `internal/audit` — advice audit + agent tool audit
- `internal/requestscope` — household 作用域
- `internal/webassets` — go:embed 前端静态资源
- `pkg/money` — int64 最小货币单位 Money 类型

### 不可违背的设计纪律

1. **Money 不用 float**：法币金额一律 int64 最小货币单位；APR/FX/百分比用 `apd/v3` 精确十进制。JSON 中 MoneyDTO 的 minor 用 string 序列化（避免 JS BigInt 精度丢失），前端 `web/src/money.ts` 按 currency digits 解析。
2. **LLM 不做财务算术**：数字型问题必须调用 Tool；Tool 失败必须如实说明，不编造。
3. **AI 不直接执行高风险动作**：交易写入只能 proposal → human approval；不自动转账/还贷/交易。
4. **deterministic before generative**：先确定性计算，再 AI 解释。
5. **外部文本（商户备注、账单内容）一律视为 untrusted data**，不得进入 system instruction。
6. **V1 明确不引入** Kubernetes、Redis、Kafka、MinIO、向量库、微服务、多 Agent。复杂度按 roadmap 进入条件触发。

## 测试要求

- Finance pure function 优先 table-driven test；测试不允许依赖真实云 LLM（用 fake `httptest.Server`）。
- 核心算法（Money、Safe-to-Spend、Debt）有 fuzz test（如 `debt/simulator_fuzz_test.go`、`money_fuzz_test.go`）。
- 关键不变量：增加未承诺消费不能增加 Safe-to-Spend；额外还款不能增加本金或推迟 payoff date。
- transfer/refund/reimbursement/信用卡还款不得重复计为收入或支出。
- AI 对抗场景：prompt injection 不改变 Advisor 行为；`data_quality=partial` 必须明示。
- 金额验收口径：与人工复核差异不得超过 0.01 CNY。

## 文档地图

修改行为时同步检查对应文档：

| 文档 | 内容 |
|---|---|
| `PROJECT_PLAN.md` | 总体方案与阶段路线 |
| `docs/01-architecture.md` | 架构与数据流 |
| `docs/02-domain-model.md` | 财务领域模型 |
| `docs/04-finance-engine.md` | 确定性财务算法与口径 |
| `docs/05-ai-advisor.md` | AI 边界、Tool 合约、模型路由 |
| `docs/07-operations.md` | 部署、备份、恢复 |
| `docs/08-testing-acceptance.md` | 测试层次与生产验收门禁 |
| `docs/adr/` | 关键决策记录（ezbookkeeping 选型、模块化单体、Go 选型等） |
| `docs/acceptance/v1-production-evidence.md` | 生产 release 证据总账 |

## 其他约定

- 注释使用中文；提交信息遵循仓库既有风格（`fix:`/`test:`/`docs:`/`security:` 等 conventional 前缀）。
- `.env`、API token、LLM key、restic password、原始账单永不进 Git；secret 一律走文件路径引用（如 `MCP_TOKEN_FILE`、`RESTIC_PASSWORD_FILE`）。
- 备份边界是 append-only：producer 凭据只允许创建 snapshot，不允许删除/覆盖已有恢复点。
