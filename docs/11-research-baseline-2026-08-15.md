# 技术调研基线 — 2026-08-15

> 本文件用于防止后续开发依赖旧记忆。升级/重大设计前重新核验官方来源，并新增新的 dated baseline，而不是覆盖历史判断。

## ezBookkeeping

### 当前稳定版本
- GitHub Releases：v1.6.1，2026-07-20。
- Source: https://github.com/mayswind/ezbookkeeping/releases

### 官方功能表确认
- Mobile interface；
- HTTP API；
- Account reconciliation；
- AI image recognition；
- Web Share Target API Level 2；
- AI Text Content Recognition / Batch Image Recognition；
- MCP；
- 2FA / OIDC / App Lock / Session Management / API Token IP allowlist；
- LLM providers 包括 OpenAI/OpenAI-compatible/OpenAI Responses-compatible/Anthropic/OpenRouter/Ollama/LM Studio/Google AI；
- local filesystem / S3-compatible MinIO / WebDAV storage。
- Source: https://ezbookkeeping.mayswind.net/features/

### 中国账单
- Alipay App Transaction Flow；
- Alipay Web Transaction Flow；
- WeChat Pay Billing；
- JD.com Finance Billing。
- Source: https://ezbookkeeping.mayswind.net/export_and_import/

### API
- Bearer Token：`Authorization: Bearer ${TOKEN}`；
- 可通过 `enable_api_token` / `EBK_SECURITY_ENABLE_API_TOKEN` 开启；
- 时间 API 支持 `X-Timezone-Name: Asia/Shanghai`。
- Source: https://ezbookkeeping.mayswind.net/httpapi/
- Source: https://ezbookkeeping.mayswind.net/configuration/

### Docker
- 官方版本镜像 `mayswind/ezbookkeeping:{version}`；
- local object storage 默认 `/ezbookkeeping/storage/`；
- 官方建议 SQLite 用于测试，正式使用 MySQL/PostgreSQL；
- 容器 UID/GID 1000:1000；
- 生产前必须生成 `secret_key`。
- Source: https://ezbookkeeping.mayswind.net/installation/installation-docker

## PostgreSQL

- 2026-08-13 发布 18.6/17.11/...；
- 18 当前 minor：18.6；支持至 2030-11-14；
- 官方建议始终运行选定 major 的 current minor。
- Source: https://www.postgresql.org/support/versioning/

Docker Official Image：
- `postgres:18.6` 已存在；
- PostgreSQL 18+ 的 Docker `PGDATA` 版本化，volume 应挂到 `/var/lib/postgresql`；
- `POSTGRES_INITDB_ARGS=--data-checksums` 可启用 data page checksums；
- host auth 默认在 14+ 使用 `scram-sha-256`；
- `/docker-entrypoint-initdb.d` 只在空 data dir 首次初始化执行。
- Source: https://hub.docker.com/_/postgres

## Caddy

- Docker Official Image 当前 `2.11.4-alpine`；
- Source: https://hub.docker.com/_/caddy

## Python

- Python 官方 Docker 在本次核验时提供 `3.13.15-slim-bookworm`；同时已有 3.14 稳定线。
- 本项目 V1 选择 3.13 稳定线以减少生态兼容风险；这属于项目决策，不代表 3.14 不稳定。
- Source: https://hub.docker.com/_/python

## Python 主要库（规划时核验）

- FastAPI 0.140.0（2026-07-24）；Source: https://pypi.org/project/fastapi/
- SQLAlchemy 2.0.51 stable；2.1 当时仍为 beta；Source: https://pypi.org/project/SQLAlchemy/
- Pydantic 2.13.4 stable；Source: https://pypi.org/project/pydantic/

锁文件应在实际首次开发环境中生成，并由 Dependabot/Renovate 或人工定期审核，而不是长期信任本文件中的版本。

## Tesla P40 / 本地 AI

- NVIDIA Legacy CUDA GPU 列表中 Tesla P40 Compute Capability 6.1；
- 因此本项目不把 P40 作为 V1 关键基础设施。
- Source: https://developer.nvidia.com/cuda/gpus/legacy

PaddleOCR 在 2026 年仍快速演进（PP-OCRv6/PaddleOCR-VL 等），V1.2 开始前必须重新查当前安装要求并做 CPU/P40 benchmark；不要根据旧版 CUDA/Paddle 文档锁死实现。

## OpenClaw

在接入阶段重新核验官方 security model。规划原则不变：OpenClaw 是外部 Client/Channel Adapter，不是家庭财务权限边界，也不持有超出必要范围的写权限。

Source: https://docs.openclaw.ai/gateway/security

## 模型供应商

模型名和定价变化极快，因此 Finance Core 只使用模型角色（fast/planner/reviewer/vision）和 provider adapter；每次生产切换模型前重新核验供应商官方文档与计费。不要以本文件列出的任何某个模型名称作为永久推荐。


## 2026-08-15 模型候选补充

### DeepSeek
- 官方 API 当前模型：`deepseek-v4-flash`、`deepseek-v4-pro`；两者支持 1M context、JSON Output、Tool Calls；旧 `deepseek-chat` / `deepseek-reasoner` 已于 2026-07-24 停止使用。
- Source: https://api-docs.deepseek.com/zh-cn/
- Source: https://api-docs.deepseek.com/zh-cn/quick_start/pricing/

### Alibaba/Qwen
- 当前官方视觉文档建议 `qwen3.7-plus` 作为起点，场景稳定后可用 `qwen3.6-flash` 降成本；两条模型线覆盖 OCR/视觉理解与结构化输出能力。
- Source: https://help.aliyun.com/zh/model-studio/vision-model/

### Zhipu GLM
- 官方模型概览当前列出 GLM-5.2，1M context，可作为 planner/reviewer 候选之一。
- Source: https://docs.bigmodel.cn/cn/guide/start/model-overview

### OpenAI
- 官方 API 当前模型页列出 GPT-5.6 Sol/Terra/Luna；本项目仅将其作为可插拔 planner/reviewer 候选。
- Source: https://platform.openai.com/overview

详细角色映射与评测方法见 `docs/12-model-strategy-2026-08-15.md`。
