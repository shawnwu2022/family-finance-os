# 架构设计

## 1. 架构原则

1. 模块化单体优先；
2. 云端主用满足随时随地；
3. ezBookkeeping 负责交易事实；
4. Finance Core 负责家庭规划事实；
5. deterministic before generative；
6. P40/OpenClaw 都是可插拔 Adapter；
7. 数据正确性高于自动化程度；
8. 每个新基础设施组件必须能回答“删掉它会失去什么当前需求”。

## 2. Finance Core 实现边界

- Go 1.26.6 模块化单体；HTTP 使用 `net/http`，日志使用 `log/slog`；
- PostgreSQL 访问采用 `pgx/v5 + sqlc`，迁移使用 goose SQL migrations；
- 不使用 GORM 或大型 Web framework；
- Money 使用 `int64` 最小货币单位，APR/FX/percentage 使用 `apd/v3` 精确十进制；
- Vue/PWA 在实现阶段构建静态资源并优先 `go:embed` 到同一 Finance Core binary；
- 月报是 PostgreSQL 中的不可变 JSONB 产物，按 household/period 唯一并保存 SHA-256 内容哈希；`job_runs` 只记录调度执行状态，不能充当报表存储；
- Python 只属于后续可选 AI Worker，Rust 不属于 V1。

## 3. V1 容器网络

```mermaid
flowchart LR
  INTERNET((Internet)) --> CADDY
  subgraph public_net
    CADDY[Caddy]
  end
  subgraph app_net
    EBK[ezBookkeeping]
    CORE[finance-core]
    PG[(postgres)]
  end
  CADDY --> EBK
  CADDY --> CORE
  EBK --> PG
  CORE --> PG
  CORE --> EBK
```

只有 Caddy 映射主机 80/443。`postgres`、`ezbookkeeping`、`finance-core` 不对公网直接映射端口。

## 4. 读请求数据流

```text
用户 → Finance UI → Finance Core
                  ├→ Planning DB
                  └→ ezBookkeeping HTTP API
                        ↓
                  Normalize / Validate
                        ↓
                  Finance Engine
                        ↓
              Structured Tool Result
                        ↓
                     LLM
                        ↓
                 Explanation
```

## 5. 记账数据流

V1 用户直接在 ezBookkeeping PWA 中完成交易写入。Finance Core 不代理所有交易写入，避免多一层故障和维护。

## 6. 未来写入数据流

```text
AI → TransactionProposal
   → schema validation
   → policy check
   → 用户确认
   → ezBookkeeping API
   → audit record
```

## 7. 失败降级

| 故障 | V1 行为 |
|---|---|
| LLM 不可用 | 仍可记账、查看 deterministic Dashboard/指标 |
| ezBookkeeping API 不可用 | Finance Core 显示账本不可达；不编造最新数字 |
| Finance Core 不可用 | ezBookkeeping 仍可独立记账 |
| P40 不可用 | V1 无影响；未来走 CPU/cloud fallback |
| 本地备份机不在线 | 云主服务继续；备份任务记录失败并重试/告警 |
| PostgreSQL 不可用 | 明确故障，不返回旧数据伪装成功 |

## 8. 为什么两个 Web 入口暂时可接受

V1 允许：
- `book.example.com`：记账；
- `finance.example.com`：Dashboard/AI/规划。

这是主动用“少开发”换取“快上线”。只有真实使用证明切换入口是高频摩擦，才进入统一移动端阶段。
