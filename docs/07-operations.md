# V1 运维手册

本文只描述当前 application-native authentication 架构。Finance Core 自己负责浏览器登录、TOTP、session、CSRF 与 household authorization；Caddy 只做 TLS / HTTP 反向代理。ezBookkeeping 保持独立账号体系和强制 2FA。

## 1. 生产主机前提

### 1.1 Application Host

- Linux VPS；
- Docker Engine + Docker Compose plugin；
- 公网只由 Caddy 监听 80/tcp、443/tcp、443/udp；
- `book.<domain>` 指向 ezBookkeeping，`finance.<domain>` 指向 Finance Core；
- PostgreSQL、ezBookkeeping、Finance Core 不映射宿主端口；
- 系统时钟同步；
- 宿主机安装 `openssl`、`restic`；
- 生产 secret 放在 Git 仓库外的专用目录；
- 如启用 MCP，Bearer token 同样使用仓库外私有文件。

参考 Compose 使用固定私有网段 `172.30.0.0/24`：Caddy `172.30.0.10`、ezBookkeeping `172.30.0.20`、Finance Core `172.30.0.30`、PostgreSQL `172.30.0.40`。Finance Core 的 `FINANCE_TRUSTED_PROXY_CIDR` 在参考 Compose 中固定为 `172.30.0.10/32`；这是登录限流的客户端 IP 解析边界，不是身份认证机制，也不应通过 `.env` 放宽。

### 1.2 独立 Backup / Maintenance Host

生产发布还要求一个与 application host 分离的恢复域：

- 持久化保存 restic repository；
- 运行带认证的 `rest-server`；
- 对生产 producer 凭据启用 `--append-only`；
- 推荐同时启用 `--private-repos`；
- 通过 HTTPS 暴露 REST backend；
- 系统时钟同步；
- 安装 `restic`、`rest-server`、`openssl`、提供 `htpasswd` 的系统包，以及 TLS reverse proxy；
- 只有受保护 maintenance 身份拥有 repository 删除 / 重写权限；
- application host 不得持有可以删除全部离站恢复点的凭据。

当前参考 `rest-server` 版本为 `0.14.0`；实际部署必须记录真实版本。

## 2. Secret 模型

`.env` 只保存域名、数据库配置、非 secret 运行参数和 **secret 文件路径**。以下四项不得把明文值放入 `.env`：

- `FINANCE_AUTH_KEY_HOST_FILE`：Finance TOTP / CSRF 等加密材料使用的 32-byte key；容器内路径固定为 `/run/secrets/finance-auth-key`；
- `FINANCE_ADMIN_PASSWORD_HOST_FILE`：bootstrap 管理员初始密码；容器内路径 `/run/secrets/finance-admin-password`；
- `EBK_API_TOKEN_HOST_FILE`：Finance Core 专用 ezBookkeeping API token；容器内路径 `/run/secrets/ezbookkeeping-api-token`；
- `EBK_SECURITY_SECRET_KEY_HOST_FILE`：ezBookkeeping signing secret；容器内路径 `/run/secrets/ezbookkeeping-secret-key`。

参考 runtime UID：Finance Core `65532`，ezBookkeeping `1000`。secret 文件推荐 `0600`，group/other 必须为 0。最终 `scripts/preflight.sh` 会检查普通文件、非空、权限、仓库外路径和可读性；如果文件已按 runtime UID 收紧，使用具有检查权限的运维身份执行最终 preflight，例如：

```bash
sudo ./scripts/preflight.sh
```

不要把 `.env`、secret files、数据库 dump、附件、原始支付宝/微信/银行账单或带凭据的验收日志提交到 Git。

## 3. 首次部署

### 3.1 准备 `.env` 与基础 secret

```bash
cp .env.example .env
chmod 600 .env
sudo install -d -m 0700 /etc/family-finance/secrets
```

编辑 `.env`，至少填写真实域名、PostgreSQL 管理密码、Finance/ezBookkeeping 独立数据库密码以及四个 `*_HOST_FILE` 路径。保持：

```text
EBK_USER_ENABLE_REGISTER=false
FINANCE_ADMIN_USERNAME=finance
```

生成基础 secret。以下命令避免把 secret 本身放进 shell 参数：

```bash
sudo sh -c 'umask 077; openssl rand -hex 16 | tr -d "\n" > /etc/family-finance/secrets/finance-auth-key'
sudo sh -c 'umask 077; openssl rand -base64 36 | tr -d "\n" > /etc/family-finance/secrets/finance-admin-password'
sudo sh -c 'umask 077; openssl rand -base64 32 | tr -d "\n" > /etc/family-finance/secrets/ezbookkeeping-secret-key'

sudo chown 65532:65532 /etc/family-finance/secrets/finance-auth-key
sudo chown 65532:65532 /etc/family-finance/secrets/finance-admin-password
sudo chown 1000:1000 /etc/family-finance/secrets/ezbookkeeping-secret-key
sudo chmod 600 /etc/family-finance/secrets/finance-auth-key \
  /etc/family-finance/secrets/finance-admin-password \
  /etc/family-finance/secrets/ezbookkeeping-secret-key
```

`EBK_API_TOKEN_HOST_FILE` 此时可以只配置目标路径，文件要等 ezBookkeeping owner 建立后再写入。因此**首次 bootstrap 的这一阶段不要运行最终 preflight**；先验证 Compose 语法：

```bash
sudo docker compose config >/dev/null
```

### 3.2 启动账本入口

```bash
mkdir -p data/ezbookkeeping-storage backups
sudo chown -R 1000:1000 data/ezbookkeeping-storage
sudo docker compose up -d postgres ezbookkeeping caddy
```

先验证 `https://<EBK_DOMAIN>/` 可访问。Finance 域此时尚未启动是正常状态。

生产 steady-state 不开放公开注册。首个 owner 推荐用 ezBookkeeping CLI 创建，避免临时扩大公网注册面：

```bash
sudo docker compose exec ezbookkeeping /ezbookkeeping/ezbookkeeping userdata user-add \
  --username '<OWNER>' \
  --email '<OWNER_EMAIL>' \
  --nickname '<OWNER_NAME>' \
  --password '<INTERACTIVE_OR_PROTECTED_VALUE>' \
  --default-currency CNY
```

如果实际操作环境无法安全避免密码进入 shell history，应改用受保护交互流程，不要把上述示例中的占位符直接替换为真实 secret 并长期留在 history。

打开 ezBookkeeping，使用 owner 登录并立即完成 2FA enrollment。上线验收必须人工确认 owner 2FA 已启用。

### 3.3 生成 Finance 专用 ezBookkeeping API token

在已认证的 ezBookkeeping owner 会话中创建一枚只供 Finance Core 使用的 API token。不要把 token 写进 `.env`。将它一次性写入 `EBK_API_TOKEN_HOST_FILE`，例如：

```bash
sudo sh -c 'umask 077; cat > /etc/family-finance/secrets/ezbookkeeping-api-token'
# 粘贴 token 后按 Ctrl-D；不要把 token 放进命令参数或 Git。
sudo chown 65532:65532 /etc/family-finance/secrets/ezbookkeeping-api-token
sudo chmod 600 /etc/family-finance/secrets/ezbookkeeping-api-token
```

参考 Compose 将 API token 的网络来源限制为 Finance Core `172.30.0.30`；不要为了手工调试把生产白名单放宽到 Caddy 或公网客户端。

### 3.4 最终 preflight、迁移与 Finance bootstrap

四个 application secret 文件都存在后执行最终 preflight：

```bash
sudo ./scripts/preflight.sh
```

确认 `.env` 中的家庭 bootstrap 参数：

```text
FINANCE_BOOTSTRAP_NAME
FINANCE_BOOTSTRAP_CURRENCY
FINANCE_BOOTSTRAP_TIMEZONE
FINANCE_BOOTSTRAP_LIQUIDITY_FLOOR_MINOR
FINANCE_ADMIN_USERNAME
```

然后执行：

```bash
sudo docker compose up --build finance-bootstrap
sudo docker compose logs finance-bootstrap
```

`finance-bootstrap` 依赖 `finance-migrate`，迁移失败会阻止 bootstrap。bootstrap 是幂等的，会创建/复用家庭并创建/复用 Finance 管理员；初始密码来自 `FINANCE_ADMIN_PASSWORD_HOST_FILE`，不应出现在日志中。

保存 bootstrap 输出的 `household_id` 只用于运维和可选 MCP `MCP_HOUSEHOLD_ID`；浏览器 UI 不应要求用户输入它。

启动 Finance Core：

```bash
sudo docker compose up -d --build finance-core
curl -fsS https://<FINANCE_DOMAIN>/healthz
curl -fsS https://<FINANCE_DOMAIN>/readyz
```

`/healthz` 是最小公开健康端点，不需要浏览器登录。业务 API 仍受 Finance Core session 认证保护。

### 3.5 Finance 首次登录

1. 打开 `https://<FINANCE_DOMAIN>/`；
2. 用户名使用 `FINANCE_ADMIN_USERNAME`；
3. 密码来自 `FINANCE_ADMIN_PASSWORD_HOST_FILE` 中的初始管理员密码；
4. 首次登录必须完成 TOTP enrollment；
5. 把一次性返回的 recovery codes 保存到独立密码管理器或其它受保护介质；
6. 完成 TOTP 后进入家庭总览；
7. 验证退出登录后原 session 不再可用。

Finance Core 的浏览器 session 使用 `__Host-finance_session`，unsafe API 还要求 CSRF token。MCP Bearer 与浏览器 session 是两个独立认证域。

## 4. 登录凭据维护

密码重置、TOTP 重置属于服务器侧维护命令，不提供公网管理 HTTP endpoint。执行前先做可恢复备份，并记录操作时间和原因。重置密码或 TOTP 后应验证旧 session 已撤销，再用新凭据登录并重新保存恢复材料。

管理员初始密码文件用于 bootstrap/维护，不代表浏览器 session；不要用反向代理凭据替代应用认证。

## 5. 日常运行

```bash
sudo docker compose ps
sudo docker compose logs --tail=200 finance-core
sudo docker compose logs --tail=200 ezbookkeeping
sudo docker compose up -d --build
```

公网暴露面必须始终满足：

- Caddy：80/tcp、443/tcp、443/udp；
- PostgreSQL：无 host port；
- ezBookkeeping：无 host port；
- Finance Core：无 host port；
- 禁止 `network_mode: host`。

参考网络的安全约束：

- ezBookkeeping trusted proxy：仅 `172.30.0.10/32`；
- ezBookkeeping API token 来源：仅 `172.30.0.30`；
- Finance `FINANCE_TRUSTED_PROXY_CIDR`：仅 `172.30.0.10/32`；
- MCP 默认关闭；启用时使用独立外部 Bearer secret 文件。

每次升级前后至少执行：

```bash
make verify
```

应用认证改动还必须执行专用认证安全门禁（Task 7 引入后）：

```bash
make verify-auth-security
```

## 6. Append-only 加密离站备份

仓库将职责分成两个脚本：

- `scripts/backup.sh`：application host producer，只创建本地 payload 并向 append-only REST backend 新增 snapshot；
- `scripts/backup-maintenance.sh`：独立 backup/maintenance host，执行 retention / prune / check。

application host 不执行离站 repository 的 `restic forget`、`restic prune` 或 full-access maintenance。这是安全边界：生产主机被攻陷时，producer credential 也不应能删除已有离站恢复点。

### 6.1 Producer 配置

`.env` 中只配置：

```text
RESTIC_REPOSITORY=rest:https://backup.example.com/family-finance-prod/
RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password
RESTIC_REST_USERNAME=family-finance-prod
RESTIC_REST_PASSWORD_FILE=/etc/family-finance/rest-server-password
```

两个密码文件必须位于仓库外、是普通文件、group/other 无权限。不得在 `.env`、URL、命令行或 Git 中保存 `RESTIC_REST_PASSWORD` 明文。

生产 `RESTIC_REPOSITORY` 必须使用 `rest:https://`，第一层 repository path 必须与 `RESTIC_REST_USERNAME` 一致，以匹配 `--private-repos`。

运行：

```bash
sudo ./scripts/preflight.sh
sudo ./scripts/backup.sh
```

`backup.sh` 会：

- 对 Finance 与 ezBookkeeping 数据库执行 `pg_dump -Fc`；
- 用 `pg_restore --list` 验证 dump；
- 打包 ezBookkeeping storage；
- 生成 `SHA256SUMS`；
- 配置离站时通过 restic HTTPS REST backend 创建新 snapshot；
- 仅清理 application host 自己的 staging backup。

`BACKUP_RETENTION_DAYS` 只影响本地 staging，不授予任何离站删除能力。

### 6.2 Backup host

推荐在独立主机运行 `rest-server 0.14.0` 或部署时记录的更新版本，并启用：

```text
--append-only
--private-repos
```

认证必须开启。不得用 `--no-auth` 暴露生产 endpoint。

本手册的标准部署是：rest-server 只监听 `127.0.0.1:8000`，由同机 Caddy 或等价受验证代理负责 TLS。**HTTPS reverse proxy is a prerequisite**。

初始化 repository：

```bash
sudo install -d -o restic -g restic -m 0700 /srv/restic/family-finance-prod
sudo install -d -o restic -g restic -m 0700 /etc/family-finance
sudo -u restic sh -c 'umask 077; openssl rand -base64 48 > /etc/family-finance/restic-password'
sudo -u restic env \
  RESTIC_REPOSITORY=/srv/restic/family-finance-prod \
  RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password \
  restic init
```

创建 producer HTTP Basic credential（这里只属于 backup REST endpoint，不属于 Finance 浏览器认证）：

```bash
sudo -u restic sh -c 'umask 077; openssl rand -base64 48 > /etc/family-finance/rest-server-producer-password'
sudo -u restic sh -c 'htpasswd -B -i -c /etc/family-finance/rest-server.htpasswd family-finance-prod < /etc/family-finance/rest-server-producer-password'
sudo chmod 0600 /etc/family-finance/rest-server.htpasswd
sudo chown restic:restic /etc/family-finance/rest-server.htpasswd
```

把 producer password 和 repository encryption password 通过受保护渠道分别复制到 application host 的 `RESTIC_REST_PASSWORD_FILE` / `RESTIC_PASSWORD_FILE`。确认复制完成后，backup host 上临时 producer plaintext 可以删除；长期保留 bcrypt htpasswd hash 即可。

启动 HTTPS endpoint 后验证：

```bash
curl -fsS https://backup.example.com/
```

### 6.3 Maintenance

所有 destructive maintenance 只在 backup host 的授权账户执行。先 dry-run：

```bash
sudo -u restic env RESTIC_REPOSITORY=/srv/restic/family-finance-prod \
  RESTIC_PASSWORD_FILE=/etc/family-finance/restic-password \
  restic forget --keep-daily 14 --keep-weekly 8 --keep-monthly 12 --dry-run
```

再运行仓库维护脚本：

```bash
sudo -u restic bash scripts/backup-maintenance.sh
```

必须验证 application host 的 append-only producer credential 无法删除历史 snapshot，而 maintenance 身份可以执行受控 retention。

## 7. Restore drill

生产发布前必须在独立恢复环境做真实恢复，不接受“备份命令成功”替代 restore 证据。

基本检查：

```bash
restic snapshots
restic check
```

从目标 snapshot restore 到隔离目录后，使用仓库 `scripts/restore-drill.sh` / 对应恢复流程验证：

- Finance dump checksum；
- ezBookkeeping dump checksum；
- storage archive checksum；
- `pg_restore` 到隔离 scratch databases；
- 关键表存在；
- 恢复后的应用数据可读。

生产验收证据只记录日期、版本、snapshot id/hash、脱敏结果，不保存原始财务内容。

## 8. 认证架构升级 / 回滚顺序

已有旧部署升级到 application-native auth 时必须保持 fail-closed，顺序如下：

1. 先备份并完成一次 restore 可用性确认；
2. 部署 auth migration、Finance Core 新版本和 secret 文件，但暂时保留旧 edge 认证保护；
3. 执行 migration 与幂等 bootstrap，创建 Finance 管理员；
4. 在仅受控访问范围验证 Finance Core 自身会拒绝无 session 业务 API；
5. 使用管理员密码完成首次登录、TOTP enrollment，并保存 recovery codes；
6. 验证 logout、CSRF、household authorization、session expiry/revocation；
7. 验证 ezBookkeeping owner 2FA、注册关闭、API token 来源限制；
8. 只有 application-native auth 验证通过后，才移除旧 edge 身份层；
9. 重新运行完整 CI / auth / MCP / edge / runtime 安全门禁；
10. 任何一步失败都保持或恢复外层保护，不得留下无认证业务 API。

回滚时，数据库 auth migration 和应用版本必须按已验证的兼容路径处理；不要仅回滚反向代理配置并假定数据层自动兼容。

## 9. Production acceptance 证据

CI 通过只证明仓库门禁，不等于 production-ready。正式发布至少需要把以下真实证据更新到 `docs/acceptance/v1-production-evidence.md`：

- 真实完整月份账本对账；
- Finance 登录：密码 → TOTP → session → logout 的真实 smoke；
- ezBookkeeping owner 2FA 已启用的人工确认；
- 浏览器 / PWA 移动端验证；
- secret hygiene 与公网暴露面检查；
- append-only producer destructive operation 被拒绝；
- backup host maintenance / `restic check`；
- 独立恢复环境 restore 成功。

只有这些 evidence 完成后才能讨论 production-ready 标记。
