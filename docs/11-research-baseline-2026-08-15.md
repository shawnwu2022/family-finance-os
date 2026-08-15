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

## Go / Finance Core

### Go
- Go 1.26 于 2026-02-10 发布；
- **Go 1.26.6 于 2026-08-13 发布**，包含 `go` command、TLS、XML、template、net/http 等安全修复和 runtime/compiler bug fixes；
- 本项目 Finance Core 以 **Go 1.26.6** 作为 2026-08-15 生产/CI 基线。
- Source: https://go.dev/doc/devel/release
- Source: https://go.dev/dl/

### pgx
- pgx `v5` 为当前稳定 major；项目说明支持当前受支持的 Go/PostgreSQL 版本；
- changelog 当前稳定发布记录到 **5.10.0（2026-06-03）**，包含针对恶意/受损 PostgreSQL server 的协议和认证 hardening；
- V1 数据访问采用 `github.com/jackc/pgx/v5`，不采用 GORM。
- Source: https://github.com/jackc/pgx
- Source: https://github.com/jackc/pgx/blob/master/CHANGELOG.md

### sqlc
- 当前最新 release：**v1.31.1（2026-04-22）**；
- 用途：从显式 PostgreSQL SQL 生成强类型 Go query code；
- V1 生成目标使用 `pgx/v5`。
- Source: https://github.com/sqlc-dev/sqlc/releases
- Source: https://docs.sqlc.dev/

### goose
- 当前 release：**v3.27.3（2026-07-22）**；
- V1 采用 SQL migrations；不运行单独 migration service。
- Source: https://github.com/pressly/goose/releases

### apd
- `github.com/cockroachdb/apd/v3` 提供 arbitrary-precision decimal，当前 latest release 为 **v3.2.3（2026-03-23）**；
- Money 本身仍采用 `int64` 最小货币单位；APR/FX/percentage 等精确十进制才使用 apd。
- Source: https://github.com/cockroachdb/apd

### V1 Go 设计约束
- HTTP 优先标准库 `net/http`；
- logging 使用 `log/slog`；
- 不使用大型 Web framework 或 ORM；
- Finance Core 构建为单 binary；
- 前端后续使用 Vue 3/TypeScript/Vite/PWA，并优先 `go:embed` 进同一个 Finance Core binary；
- 业务核心不依赖 Python runtime。

## Python / Tesla P40 / 本地 AI

Python 不是 V1 Finance Core 运行时。只有进入本地隐私 AI 阶段时，才增加可选 `finance-ai-worker`：
- PaddleOCR / PyTorch / OpenCV / Transformers / VLM；
- PII redaction；
- P40 batch OCR / local inference。

NVIDIA Legacy CUDA GPU 列表中 Tesla P40 Compute Capability 6.1，因此 P40 不能成为核心可用性依赖。
- Source: https://developer.nvidia.com/cuda/gpus/legacy

PaddleOCR 仍快速演进；V1.2 开始前必须重新查当前安装要求并做 CPU/P40 benchmark，不根据旧 CUDA 文档锁死实现。

## Rust

V1 不引入 Rust。只有未来出现可测量的 native privacy agent、WASM、crypto/system component 需求时重新评估，避免为家庭级 HTTP/SQL/业务规则系统提前承担额外语言与生态成本。

## OpenClaw

在接入阶段重新核验官方 security model。规划原则不变：OpenClaw 是外部 Client/Channel Adapter，不是家庭财务权限边界，也不持有超出必要范围的写权限。

Source: https://docs.openclaw.ai/gateway/security

## 模型供应商

模型名和定价变化极快，因此 Finance Core 只使用模型角色（fast/planner/reviewer/vision）和 provider adapter；每次生产切换模型前重新核验供应商官方文档与计费。不要以本文件列出的任何某个模型名称作为永久推荐。

详细角色映射与评测方法见 `docs/12-model-strategy-2026-08-15.md`。
