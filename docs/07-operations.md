# V1 运维手册

## 1. 主机最低前提

### 生产 VPS

- Linux 云服务器；
- Docker Engine + Docker Compose plugin；
- 公网只开放 80/443 给 Caddy；
- ezBookkeeping 与 Finance Core 各有一个 DNS A/AAAA 记录；
- 系统时钟同步；
- 宿主机安装 `restic`，用于创建加密离站备份；
- 如启用 MCP，Bearer secret 以仓库外私有文件提供。

### 独立 Backup / Maintenance Host

生产发布还要求一个与生产 VPS 分离的恢复域：

- 持久化保存 restic repository；
- 运行带认证的 `rest-server`；
- 对生产凭据启用 `--append-only`；
- 推荐同时启用 `--private-repos` 做 producer 隔离；
- 通过 HTTPS 暴露 REST backend；
- 系统时钟同步，maintenance retention 以该受信任时钟为准；
- 安装 `restic`、`rest-server`、`openssl`、提供 `htpasswd` 的系统包，以及用于 TLS 终止的 Caddy（或已经受验证的等价 HTTPS reverse proxy）；
- 只有该主机的受保护维护上下文拥有本地 repository 的完整删除/重写权限；
- 生产 VPS 不得拥有 backup host 文件系统、full-access REST endpoint 或其它可以删除全部离站恢复点的凭据。

当前参考 rest-server 版本为 `0.14.0`；部署时必须记录实际版本。rest-server 当前官方语义中，`--append-only` 允许创建新备份但禁止删除/修改已有备份；`--private-repos` 要求用户名与 URL 的第一个 repository path component 对应。

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
13. 在独立 backup host 上部署并初始化 append-only restic REST repository，见第 5 节；
14. 在生产 VPS 配置 REST producer 凭据并再次执行 `./scripts/preflight.sh`；
15. 执行第一次 `./scripts/backup.sh`；
16. 从生产凭据验证 destructive operation 被拒绝；
17. 在 backup host 执行 `restic check` 和 maintenance dry-run；
18. 在独立恢复环境执行真实 snapshot restore；
19. 把真实执行证据记录到 `docs/acceptance/v1-production-evidence.md`。

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

启用离站备份时有两个不同 secret：

- `RESTIC_PASSWORD_FILE`：restic repository 的加密密码；生产 VPS 创建 snapshot 时需要它，backup/maintenance host 做恢复和维护时也需要它；
- `RESTIC_REST_PASSWORD_FILE`：生产 producer 登录 append-only REST endpoint 的认证密码。它只用于 REST 认证，不等于 repository 加密密码。

`RESTIC_REST_USERNAME` 不是 secret，可以位于 `.env`；REST 密码本身不得写入 `.env`、repository URL、命令行、Git 或验收证据。**不得在 `.env` 或宿主环境预设原生 `RESTIC_REST_PASSWORD`**；`scripts/preflight.sh` 和 `scripts/backup.sh` 会拒绝它。`scripts/backup.sh` 只在调用 restic 子进程时从 `RESTIC_REST_PASSWORD_FILE` 读取并临时注入原生 `RESTIC_REST_PASSWORD`。

两个 password file 都必须：

- 位于 Git 仓库外；
- 是普通文件；
- 对执行备份/维护的账号可读；
- group / other 权限均关闭，推荐 `0600`。

`scripts/preflight.sh` 在部署前校验这些条件，`scripts/backup.sh` 在每次真正使用 secret 前再次校验文件类型、可读性和 group/other 权限，防止 preflight 后权限漂移仍继续备份。

不要把以下内容加入 Git：

- `.env`；
- restic repository password file；
- rest-server producer password file；
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

## 5. Append-only 加密离站备份

仓库提供两个职责明确分离的脚本：

- `scripts/backup.sh`：**生产 VPS producer**；只创建本地 backup payload 并向 append-only REST backend 新增 restic snapshot；
- `scripts/backup-maintenance.sh`：**独立 backup/maintenance host**；只针对本机 repository 路径执行 retention / prune / check。

生产 VPS 不执行 `restic forget`、`restic prune` 或完整 repository `restic check`。这是安全边界，不是运维偏好：即使生产 VPS 被攻陷，其 producer credential 也不应足以删除已有离站恢复点。

### 5.1 生产 producer 做什么

每次执行 `scripts/backup.sh` 会：

- 用 PostgreSQL 管理角色分别对 Finance Core 和 ezBookkeeping 数据库执行 custom-format `pg_dump -Fc`；
- 用 `pg_restore --list` 检查 dump 可读；
- 打包 `data/ezbookkeeping-storage`；
- 为两份 dump 和 storage archive 生成 `SHA256SUMS`；
- 如果配置了离站 backup，则通过 `rest:https://...` 向 append-only REST backend 执行 `restic snapshots` 和 `restic backup`；
- 独立清理生产 VPS 上过期的本地 staging backup。

`BACKUP_RETENTION_DAYS` 只影响生产 VPS 自己的 staging 目录；它没有任何离站 repository 删除权限。

### 5.2 Backup host：rest-server append-only endpoint

V1 推荐在独立 backup host 上运行 `rest-server 0.14.0` 或部署时明确记录的更新版本，并启用：

```text
--append-only
--private-repos
```

认证必须开启。不得使用 `--no-auth` 暴露生产 endpoint。生产 `RESTIC_REPOSITORY` 必须始终使用 `rest:https://...`。

本手册采用的标准部署是：rest-server 仅监听 `127.0.0.1:8000`，由同机 Caddy 负责公网 TLS。**HTTPS reverse proxy is a prerequisite**；如果 backup host 已有受验证的等价反向代理，可以替换 Caddy，但必须满足相同的 HTTPS、证书、loopback upstream 和端口暴露约束。

若 producer 用户名为：

```text
family-finance-prod
```

则在 `--private-repos` 模式下，repository URL 的第一层 path 与用户名保持一致，例如：

```text
rest:https://backup.example.com/family-finance-prod/
```

生产 `preflight.sh` 会验证这个关系；username/path 不一致会在真正访问 rest-server 前直接失败。

backup host 的持久化 data root、认证文件、TLS 私钥和 repository 目录都不得暴露给生产 VPS。

### 5.3 Repository、producer 账号与 rest-server 初始化

Repository 初始化属于 backup administrator 的 full-access 操作，**在 backup host 本地完成**，不要让生产 producer 负责创建 full-access repository。

以下示例假定已经按官方 release 校验并安装 `rest-server 0.14.0` 到 `/usr/local/bin/rest-server`，系统存在专用 `restic` 用户/组，并安装了提供 `htpasswd` 的工具。Debian/Ubuntu 通常由 `apache2-utils` 提供 `htpasswd`。

先创建受保护目录和 repository encryption password：

```bash
sudo install -d -o restic -g restic -m 0700 /srv/restic/family-finance-prod
sudo install -d -o restic -g restic -m 0700 /etc/family-finance
sudo -u restic sh -c 'umask 077; openssl rand -base64 48 > /etc/family-finance/restic-password'

sudo -u restic env \
  RESTIC_REPOSITORY=/srv/restic/family-finance-prod \
  RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password \
  restic init
```

为生产 producer 生成**独立的 REST Basic Auth password**，并写入 bcrypt htpasswd database。输入重定向必须在 `restic` 身份的 shell 内执行；不能把 `< /etc/family-finance/...` 留给普通管理员 shell：

```bash
sudo -u restic sh -c 'umask 077; openssl rand -base64 48 > /etc/family-finance/rest-server-producer-password'
sudo -u restic sh -c 'htpasswd -B -i -c /etc/family-finance/rest-server.htpasswd family-finance-prod < /etc/family-finance/rest-server-producer-password'
sudo chmod 0600 /etc/family-finance/rest-server.htpasswd
sudo chown restic:restic /etc/family-finance/rest-server.htpasswd
```

将 `/etc/family-finance/rest-server-producer-password` 的内容通过受保护渠道**一次性**复制到生产 VPS 的 `/etc/family-finance/rest-server-password`，生产文件由实际执行 `backup.sh` 的账号持有并设为 `0600`。确认生产凭据已落地后，删除 backup host 上这份 producer plaintext；backup host 长期只需保留 bcrypt htpasswd hash：

```bash
sudo rm -f /etc/family-finance/rest-server-producer-password
```

repository encryption password 也需要安全复制一份到生产 VPS 的独立 `RESTIC_PASSWORD_FILE`，因为 producer 创建加密 snapshot 时需要它；不要把 backup-host 文件系统权限或 full-access maintenance authority 一并复制到生产机。

#### systemd service

rest-server 本体只绑定 loopback。创建 `/etc/systemd/system/rest-server.service`：

```ini
[Unit]
Description=Family Finance append-only rest-server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=restic
Group=restic
UMask=0077
ExecStart=/usr/local/bin/rest-server --path /srv/restic --listen 127.0.0.1:8000 --append-only --private-repos --htpasswd-file /etc/family-finance/rest-server.htpasswd
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/srv/restic
ReadOnlyPaths=/etc/family-finance/rest-server.htpasswd

[Install]
WantedBy=multi-user.target
```

然后：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now rest-server
sudo systemctl status --no-pager rest-server
```

#### HTTPS reverse proxy

backup DNS（例如 `backup.example.com`）先指向 backup host，公网防火墙只允许 Caddy 所需的 80/443；不得开放 rest-server 的 8000/tcp。若没有现成的受验证 HTTPS proxy，使用 Caddy。示例 `/etc/caddy/Caddyfile`：

```caddyfile
backup.example.com {
    @root path /
    respond @root "ok" 200
    reverse_proxy 127.0.0.1:8000
}
```

应用并验证：

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl enable --now caddy
sudo systemctl reload caddy
curl -fsS https://backup.example.com/
```

最后一条必须通过正常的公网 DNS 和受信任证书返回 `ok`。它只验证 TLS/反代入口；真正的 restic repository 路径仍由 rest-server 的 htpasswd 认证保护。然后从生产 VPS 通过 `rest:https://backup.example.com/family-finance-prod/` 做第一次真实 producer smoke。

无论使用 Caddy 还是已受验证的等价 HTTPS proxy，生产端 `RESTIC_REPOSITORY` 都必须是 `rest:https://...`；不得为了省事使用 `--no-auth`、公网明文 HTTP，或把 8000/tcp 直接暴露到 Internet。

启动后检查日志，必须看到 append-only/private-repositories 已启用；并确认服务使用 `/etc/family-finance/rest-server.htpasswd` 成功加载认证数据。初始化完成后，backup host 本地 full-access maintenance context 应能执行：

```bash
sudo -u restic env \
  RESTIC_REPOSITORY=/srv/restic/family-finance-prod \
  RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password \
  restic snapshots
```

### 5.4 生产 VPS producer 配置

在生产 VPS 的 `.env` 中只保存非 secret 的 repository URL 和 username；两个密码通过仓库外私有文件提供。`RESTIC_REPOSITORY` URL 本身不得包含 `user:password@host` 形式的 userinfo：

```env
RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-prod/
RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password
RESTIC_REST_USERNAME=family-finance-prod
RESTIC_REST_PASSWORD_FILE=/etc/family-finance/rest-server-password
BACKUP_RETENTION_DAYS=14
```

`--private-repos` 模式下，`RESTIC_REPOSITORY` 的第一层 repository path 必须和 `RESTIC_REST_USERNAME` 完全一致；`preflight.sh` 会拒绝不一致配置。不要额外设置 `RESTIC_REST_PASSWORD`。

创建 password file 后，确保文件由实际执行 producer backup 的账号可读且 group/other 无权限：

```bash
sudo chmod 0600 /etc/family-finance/restic-password
sudo chmod 0600 /etc/family-finance/rest-server-password
./scripts/preflight.sh
```

旧变量：

```text
BACKUP_KEEP_DAILY
BACKUP_KEEP_WEEKLY
BACKUP_KEEP_MONTHLY
```

在生产 producer 上已经废弃，preflight 会拒绝它们。离站 retention 不再由生产 VPS 控制。

首次 producer backup：

```bash
./scripts/backup.sh
```

成功标准是本地 dump/archive/checksum 创建成功，并且 append-only endpoint 接受新的 `restic backup` snapshot。生产脚本不负责 prune/check。

### 5.5 必须验证生产凭据不能删除已有 snapshot

生产上线验收必须从生产 producer 上尝试一个**不会泄露 secret 的受控 destructive probe**，确认 append-only endpoint 拒绝删除/覆盖已有 repository data。

不要在证据文件记录密码、URL 中的凭据或原始财务数据。只记录：

- 时间；
- sanitized snapshot ID；
- destructive operation 被拒绝；
- HTTP/restic 脱敏错误类别或退出码。

若生产凭据能成功执行删除/覆盖，M-03 为 FAIL，生产 release 必须保持 BLOCKED。

### 5.6 Backup host maintenance

离站 retention、prune、完整 check 只在独立 backup host 上执行。该主机使用**本地 repository path**，不使用生产 REST producer credential。因为 `/etc/family-finance` 由 `restic:restic` 以 `0700` 持有，maintenance 必须在 `restic` 身份下执行：

```bash
sudo -u restic env \
  RESTIC_MAINTENANCE_REPOSITORY=/srv/restic/family-finance-prod \
  RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password \
  BACKUP_KEEP_WITHIN=2y \
  bash scripts/backup-maintenance.sh
```

如果上述变量已经通过受保护的 systemd `Environment=` / `EnvironmentFile=` 注入，实际执行体必须保持为：

```bash
sudo -u restic bash scripts/backup-maintenance.sh
```

不要以普通管理员账号直接运行该脚本，也不要放宽 `/etc/family-finance` 权限来迁就错误的执行身份。

脚本执行：

```text
restic snapshots
restic forget --group-by '' --keep-within <duration> --prune
restic check
```

producer 每次备份的是不同 UTC 时间戳目录。如果使用 restic 默认的 `host,paths` grouping，每个 timestamp path 会形成独立 snapshot group，使 retention 无法按预期淘汰历史 snapshot。因此 producer backup 和 maintenance forget 都显式使用 `--group-by ''`，把这个专用 repository 的 Family Finance snapshots 作为一个 retention set。

V1 append-only retention 只允许基于 `--keep-within` 的保护窗口，不使用 `--keep-daily`、`--keep-weekly`、`--keep-monthly` 或 `--keep-last` 等计数型 producer retention。原因是受攻陷 producer 可以生成带恶意时间戳/数量的 snapshot；restic 官方对 append-only repository 明确建议使用 `--keep-within`，并由独立安全 client 执行 `forget/prune`。

restic 的 duration-based `--keep-within` 会忽略相对于 maintenance host 当前时间位于未来的 snapshot；这些未来 snapshot 本身不会被自动删除，也不会作为合法 retention cutoff。该安全性质依赖 maintenance host 时钟可信，因此 backup/maintenance host 必须保持系统时间同步，并在 destructive maintenance 前检查异常 snapshot 数量和时间。

默认：

```text
BACKUP_KEEP_WITHIN=2y
```

第一次真实 destructive maintenance 前，以 `restic` 身份运行 dry-run 并人工检查异常 snapshot 数量/时间：

```bash
sudo -u restic env RESTIC_REPOSITORY=/srv/restic/family-finance-prod \
  RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password \
  restic forget --group-by '' --keep-within 2y --dry-run
```

确认结果符合预期后，再调度 `scripts/backup-maintenance.sh`。维护窗口应避开生产 backup 时间，因为 prune 会锁定 repository。

### 5.7 从旧 SFTP 模型迁移

如果已有旧 restic/SFTP repository，迁移时不要原地替换后立即删除旧副本：

1. 部署独立 append-only rest-server；
2. 在 backup host 本地初始化新的 REST repository；
3. 创建并验证带认证的 producer account，确认 rest-server 以 `--append-only --private-repos` 启动；
4. 在生产 VPS 配置新的 `rest:https://...` producer 凭据；
5. 执行 `./scripts/preflight.sh`；
6. 执行 `./scripts/backup.sh` 并确认新 snapshot；
7. 从生产凭据证明 destructive operation 被拒绝；
8. 在 backup host 执行 `restic check` 和 `forget --group-by '' --keep-within 2y --dry-run`；
9. 在独立环境完成真实 snapshot restore；
10. 只有新边界和恢复证据全部通过后，才按旧 repository 自己的保留策略退役 SFTP 副本。

仓库代码变更不会自动删除、转换或迁移现有 SFTP repository。

### 5.8 调度

生产 producer 建议每天由 systemd timer 或 cron 运行：

```bash
./scripts/backup.sh
```

对**非零退出码**发通知。不要把完整 backup 日志上传到第三方日志系统；日志只需记录时间、退出码、snapshot ID/脱敏摘要。

Backup host maintenance 与生产 producer 使用不同调度、不同权限。不要把 full-access maintenance credential 复制到生产 VPS。

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

CI 会用 disposable PostgreSQL 自动执行同一恢复脚本；这证明脚本和 dump/restore 链路可工作，但**不能替代生产 backup 的真实异机恢复演练**。

### 6.2 异机灾备恢复

生产 V1 发布前至少执行一次异机恢复：

1. 在独立恢复主机安装兼容版本的 restic、Docker Engine 和 Docker Compose；
2. 使用受保护的 recovery/maintenance authority 从真实 repository 恢复一个 snapshot 到临时目录；生产 append-only credential 不是 full-access 恢复管理凭据；
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

保持所选 major 的受支持 minor。升级前：backup → 阅读 release notes → 拉新镜像 → 重启 → 健康检查 → restore drill。

### PostgreSQL major

不自动跟随。单独建立升级任务、backup 和 `pg_upgrade`/dump-restore 演练。

### ezBookkeeping

固定具体版本。升级前查看 release notes，尤其 breaking changes、API 字段要求、PWA/代理行为变化；升级后重新做真实账单导入 smoke。

### Finance Core

每次部署必须先通过 CI；数据库迁移使用 goose SQL migrations，迁移脚本进入 Git。

### rest-server / restic

- rest-server 固定已验证版本；升级前阅读 changelog，重点检查 append-only、private-repos、认证和 TLS 行为；
- restic client 升级前检查 repository format/backend compatibility；
- 升级后重新验证 producer 可以 append、不能 destructive delete，并在 maintenance host 执行 `restic check`。

## 8. 监控 V1

不部署 Prometheus/Grafana。先用：

- Docker healthcheck；
- `docker compose ps`；
- Caddy/应用日志；
- 磁盘使用率定时检查；
- 生产 `backup.sh` 退出码/通知；
- 最近 append-only restic snapshot 时间；
- maintenance host 最近 `restic check` 结果；
- backup host repository 磁盘容量；
- scheduler `job_runs` 中的 failed 状态。

只有实际需要趋势图/告警集中化时再加监控栈。
