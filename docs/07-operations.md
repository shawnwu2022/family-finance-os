# V1 运维手册

## 1. 主机最低前提

- Linux 云服务器；
- Docker Engine + Docker Compose plugin；
- 公网 80/443；
- 两个 DNS A/AAAA 记录；
- 系统时钟同步；
- 本地服务器可通过 SSH/rsync 接收备份。

资源不在文档中硬写“最低 CPU/RAM”，因为真实负载受数据、LLM、附件影响。家庭 V1 先在现有 VPS 运行并监控 RSS/CPU/磁盘，只有出现压力再扩容。

## 2. 首次部署顺序

1. `cp .env.example .env`；
2. 填域名、PostgreSQL 管理密码、两个应用数据库独立密码及 ezBookkeeping Secret；
3. `mkdir -p data/ezbookkeeping-storage backups`；
4. `sudo chown -R 1000:1000 data/ezbookkeeping-storage`；
5. `docker compose up -d postgres ezbookkeeping caddy`；
6. 打开 ezBookkeeping，创建管理员并启用 2FA；
7. 在 ezBookkeeping 开启 API Token 后生成专用 Token；
8. 写入 `.env` 的 `EBK_API_TOKEN`；
9. `docker compose up -d --build finance-core`；
10. 验证 `https://finance.example.com/healthz`；
11. 执行第一次备份；
12. 在本地服务器做恢复演练。


### 数据库角色隔离

V1 仍然只运行一个 PostgreSQL 实例，但 Finance Core 与 ezBookkeeping 使用不同数据库和不同登录角色。`POSTGRES_USER` 仅用于初始化与备份，不下发给应用容器。这样不增加新的基础设施，同时避免任一应用数据库凭据天然拥有另一财务域数据库的访问权限。

## 3. Secret

生成：

```bash
openssl rand -base64 32
```

`.env`：

```bash
chmod 600 .env
```

不要把 `.env`、dump、附件加入 Git。

## 4. 日常操作

```bash
docker compose ps
docker compose logs --tail=200 finance-core
docker compose logs --tail=200 ezbookkeeping
docker compose pull
docker compose up -d --build
```

## 5. 备份

仓库提供 `scripts/backup.sh`。它会：
- 使用 PostgreSQL 管理角色对 `finance` 与 `ezbookkeeping` 进行 custom-format `pg_dump`；
- 打包 ezBookkeeping storage；
- 生成 SHA-256；
- 根据配置可选通过 SSH/rsync 传到本地服务器（传输链路由 SSH 加密；目标磁盘静态加密需由本地备份服务器承担）；
- 清理过期日备份。

周/月归档建议由本地服务器再做保留策略，避免生产 VPS 上脚本过度复杂。

## 6. 恢复流程

恢复优先在隔离主机演练：
1. 拉起同 major PostgreSQL；
2. 创建空数据库；
3. `pg_restore --clean --if-exists`；
4. 恢复附件目录；
5. 拉起 ezBookkeeping/Finance Core；
6. 运行健康检查；
7. 抽查账户数量、交易时间范围、关键财务快照；
8. 记录恢复耗时。

## 7. 升级策略

### PostgreSQL minor

官方建议保持所选 major 的当前 minor。升级前：备份 → 阅读 release notes → 拉新镜像 → 重启 → 健康检查。

### PostgreSQL major

不自动跟随。单独建立升级任务、备份和 `pg_upgrade`/dump-restore 演练。

### ezBookkeeping

固定具体版本。升级前查看 release notes，尤其 breaking changes、API 字段要求、PWA/代理行为变化。

### Finance Core

每次部署必须先通过测试；数据库迁移使用 Alembic，迁移脚本进入 Git。

## 8. 监控 V1

不部署 Prometheus/Grafana。先用：
- Docker healthcheck；
- `docker compose ps`；
- Caddy/应用日志；
- 磁盘使用率 cron 检查；
- 备份脚本退出码/通知。

只有实际需要趋势图/告警集中化时再加监控栈。
