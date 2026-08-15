# ADR-0007: Finance Core 采用 Go-first，Python 降级为可选 AI Worker

**Status:** Accepted

## Context

Finance Core 的主要职责是 HTTP/API、家庭财务领域模型、PostgreSQL、预算/债务/目标/Scenario 确定性计算、AI Tool orchestration 与长期自托管运行。它不是量化研究 Notebook、模型训练服务或 GPU 推理服务。

项目要求长期低运维、强类型、单机可靠和清晰的财务计算边界。当前 Python 骨架仅有 `/healthz`，尚无业务代码，因此迁移成本接近零。

## Decision

- Finance Core 主语言：**Go 1.26.6 baseline**；
- HTTP：Go 标准库 `net/http`；
- 日志：`log/slog`；
- PostgreSQL：后续 V1 使用 `pgx/v5 + sqlc`；
- migrations：goose SQL migrations；
- Money：`int64` 最小货币单位；
- APR/FX/百分比：后续使用 `cockroachdb/apd/v3` arbitrary-precision decimal；
- Python 不进入 V1 核心运行链路，仅在 V1.2+ 作为可选 OCR/VLM/PII/P40 worker；
- Rust 不进入 V1，只有未来出现明确 native/crypto/WASM 需求再重新评估。

## Consequences

### Positive

- Finance Core 可构建为单 binary，运行依赖和容器显著简化；
- 强类型领域接口更利于财务规则审计、table tests、fuzz/property tests；
- 不依赖大型 Agent/ORM framework；
- P40/Python AI worker 故障不会影响记账、预算、债务与 Dashboard；
- 将来替换 LLM、账本 Adapter 或移动端不会推翻 Finance Engine。

### Negative

- OCR、GPU、复杂数据科学任务的 Python 生态不能直接嵌入 Finance Core；需要时通过独立可选 Worker 接入；
- 部分高级统计/研究功能需要额外 Python analysis worker，而不是在核心进程中完成。
