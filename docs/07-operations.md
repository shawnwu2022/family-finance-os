# V1 运维手册

## 1. 主机最低前提

- Linux 云服务器；
- Docker Engine + Docker Compose plugin；
- 公网只开放 80/443 给 Caddy；
- 两个 DNS A/AAAA 记录；
- 系统时钟同步；
- 独立的 SFTP 备份目标，使用 SSH key 认证；
- 主机安装 `restic`，用于加密离站备份。

资源不在文档中硬写“最低 CPU/RAM”，因为真实负载受数据、LLM、附件影响。家庭 V1 先在现有 VPS 运行并监控 RSS/CPU/磁盘，只有出现压力再扩容。

## 2. 首次部署顺序

1. `cp .env.example .env`；
2. 填域名、PostgreSQL 管理密码、两个应用数据库独立密码及 ezBookkeeping Secret；
3. 为 Finance Core 公网入口设置 `FINANCE_AUTH_USER`，并交互式生成 Caddy bcrypt hash：

   ```bash
   docker run --rm -it caddy:2.11.4-alpine caddy hash-password --algorithm bcrypt
   ```

   把输出以**单引号**写入 `.env`，例如 `FINANCE_AUTH_HASH='$2a$...'`，不要保存明文密码；
4. `chmod 600 .env`，然后执行 `./scripts/preflight.sh`；
5. `mkdir -p data/ezbookkeeping-storage backups`；
6. `sudo chown -R 1000:1000 data/ezbookkeeping-storage`；
7. `docker compose up -d postgres ezbookkeeping caddy`；此时先保证 ezBookkeeping 可通过 HTTPS 访问。Finance 域在 finance-core 尚未启动时暂时不可用是正常的；
8. 打开 ezBookkeeping，创建管理员并启用 2FA；
9. 在 ezBookkeeping 开启 API Token 后生成 Finance Core 专用 Token；
10. 写入 `.env` 的 `EBK_API_TOKEN`；
11. `docker compose up -d --build finance-core`；
12. 访问 `https://<FINANCE_DOMAIN>/healthz`，应先出现 Caddy Basic Auth，认证后返回健康状态；再检查 `/readyz`；
13. 配置 restic/SFTP；
14. 执行第一次备份；
15. 使用该备份执行 `scripts/restore-drill.sh`；
16. 把真实执行证据记录到 `docs/acceptance/v1-production-evidence.md`。

Caddy 不依赖 finance-core 启动，这是刻意设计的首次部署 bootstrap 边界：必须先能访问 ezBookkeeping 才能生成 `EBK_API_TOKEN`。Finance Core 仍只在内部 Docker network 上被 Caddy 反向代理，没有新增 host 端口。

### 数据库角色隔离

V1 仍然只运行一个 PostgreSQL 实例，但 Finance Core 与 ezBookkeeping 使用不同数据库和不同登录角色。`POSTGRES_USER` 仅用于初始化与备份，不下发给应用容器。这样不增加新的基础设施，同时避免任一应用数据库凭据天然拥有另一财务域数据库的访问权限。

## 3. Secret

通用随机 Secret 生成：

```bash
openssl rand -base64 32
```

Finance Core 的公网 Basic Auth 密码不要写进 `.env`；`.env` 只存 Caddy 生成的 bcrypt hash，且该值应使用单引号包裹，避免 hash 中的 `$` 被 Compose 解释。

```bash
chmod 600 .env
```

不要把以下内容加入 Git：

- `.env`；
- restic password file；
- SSH private key；
- PostgreSQL dump；
- ezBookkeeping storage；
- 原始支付宝/微信/银行账单；
- 含账户号、身份证、手机号、API key、token 的验收日志。

验收文档只保存脱敏摘要、hash、时间、版本与结果，不保存原始财务数据。

## 4. 日常操作

```bash
docker compose ps
docker compose logs --tail=200 finance-core
docker compose logs --tail=200 ezbookkeeping
docker compose pull
docker compose up -d --build
```

公网暴露面应始终满足：

- Caddy：宿主 80/tcp、443/tcp、443/udp；
- PostgreSQL：无宿主端口；
- ezBookkeeping：无宿主端口；
- Finance Core：无宿主端口；
- 所有应用服务禁止 `network_mode: host`。

CI 的 `scripts/check-edge-security.sh` 会把上述约束作为硬门禁。

## 5. 加密备份

仓库提供 `scripts/backup.sh`。每次执行会：

- 用 PostgreSQL 管理角色分别对 Finance Core 和 ezBookkeeping 数据库执行 custom-format `pg_dump -Fc`；
- 用 `pg_restore --list` 检查 dump 可读；
- 打包 `data/ezbookkeeping-storage`；
- 为两份 dump 和 storage archive 生成 `SHA256SUMS`；
- 如果配置了 restic，则把整个时间戳目录加密写入 SFTP repository；
- 执行 restic retention/prune/check；
- 独立清理生产 VPS 上过期的本地 staging backup。

### 5.1 restic/SFTP 初始化

为备份服务账号配置 SSH host alias、private key 和固定 `known_hosts`。示例：

```sshconfig
Host family-finance-backup
    HostName backup.example.net
    User backup
    IdentityFile /etc/family-finance/backup_ed25519
    IdentitiesOnly yes
```

创建仓库外的 password file：

```bash
sudo install -d -m 0700 /etc/family-finance
openssl rand -base64 48 | sudo tee /etc/family-finance/restic-password >/dev/null
sudo chmod 0600 /etc/family-finance/restic-password
```

在 `.env` 中设置：

```env
RESTIC_REPOSITORY=sftp:family-finance-backup:/srv/restic/family-finance-os
RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password
BACKUP_KEEP_DAILY=14
BACKUP_KEEP_WEEKLY=8
BACKUP_KEEP_MONTHLY=12
BACKUP_RETENTION_DAYS=14
```

首次只初始化一次：

```bash
set -a; source .env; set +a
restic init
restic snapshots
```

`RESTIC_PASSWORD_FILE` 必须位于仓库外。`scripts/backup.sh` 会拒绝仓库内 password file，也会拒绝已废弃的 `BACKUP_REMOTE`/raw rsync 路线。配置了 restic 后，任何远端备份、prune 或 `restic check` 失败都会让任务返回非零，不允许静默降级为“只有本地副本”。

### 5.2 执行与调度

手工执行：

```bash
./scripts/backup.sh
```

生产建议每天由 systemd timer 或 cron 运行，并对**非零退出码**发通知。不要把备份日志完整上传到第三方日志系统；日志只需记录时间、退出码、snapshot ID/脱敏摘要。

## 6. 恢复演练

### 6.1 同机隔离恢复验证

`scripts/restore-drill.sh` 不覆盖生产库。它会：

1. 校验 `SHA256SUMS`；
2. 创建两个唯一命名的 scratch databases；
3. 对两份 custom-format dump 执行真实 `pg_restore`；
4. 验证恢复后存在 public tables；
5. 检查 storage tar 路径，拒绝绝对路径/`..` traversal；
6. 解包到临时目录并验证 `ezbookkeeping-storage`；
7. 自动删除 scratch databases 和临时目录。

执行：

```bash
./scripts/restore-drill.sh backups/<UTC_TIMESTAMP>
```

CI 会用 disposable PostgreSQL 自动执行同一恢复脚本；这证明脚本和 dump/restore 链路可工作，但**不能替代生产备份的真实恢复演练**。

### 6.2 异机灾备恢复

生产 V1 发布前至少执行一次异机恢复：

1. 在独立主机安装同 major PostgreSQL 和相同应用版本；
2. 从 restic 恢复一个真实 snapshot 到临时目录；
3. 校验 `SHA256SUMS`；
4. 恢复两份数据库和 ezBookkeeping storage；
5. 拉起 ezBookkeeping、Finance Core、Caddy；
6. 验证 `/healthz`、`/readyz`；
7. 抽查账户数、交易时间范围、最近月 Dashboard/月报；
8. 验证 scheduler 重启后不会重复生成同一计划任务；
9. 记录 RTO、恢复 snapshot ID、软件版本、执行人和脱敏结果。

异机演练结果写入验收证据文件；没有真实执行记录时不得标记 V1 restore gate 通过。

## 7. 升级策略

### PostgreSQL minor

保持所选 major 的受支持 minor。升级前：备份 → 阅读 release notes → 拉新镜像 → 重启 → 健康检查 → restore drill。

### PostgreSQL major

不自动跟随。单独建立升级任务、备份和 `pg_upgrade`/dump-restore 演练。

### ezBookkeeping

固定具体版本。升级前查看 release notes，尤其 breaking changes、API 字段要求、PWA/代理行为变化；升级后重新做真实账单导入 smoke。

### Finance Core

每次部署必须先通过 CI；数据库迁移使用 goose SQL migrations，迁移脚本进入 Git。

## 8. 监控 V1

不部署 Prometheus/Grafana。先用：

- Docker healthcheck；
- `docker compose ps`；
- Caddy/应用日志；
- 磁盘使用率定时检查；
- 备份脚本退出码/通知；
- 最近 restic snapshot 时间与 `restic check` 结果；
- scheduler `job_runs` 中的 failed 状态。

只有实际需要趋势图/告警集中化时再加监控栈。
