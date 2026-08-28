# 安全与隐私

## V1 威胁模型

保护目标：账本数据、债务/收入/资产信息、交易附件、API Token、LLM API Key、浏览器登录凭据、TOTP/恢复码、会话与 CSRF 状态。

主要风险：
- VPS 暴露面；
- 弱密码、凭据或会话泄漏；
- Docker/组件过期；
- 数据库公网开放；
- 反向代理来源伪造；
- LLM 第三方数据暴露；
- Prompt Injection；
- 误操作/AI 写操作；
- 备份不可恢复。

## V1 控制

1. Caddy 是唯一公网入口；Caddy 只负责 TLS 与反向代理，不参与 Finance 用户身份认证；
2. PostgreSQL 不绑定 host 公网端口；
3. ezBookkeeping / Finance Core 只在确定性的私有 Docker network 内接受反代；
4. Finance Core 负责浏览器用户认证与 household authorization，登录采用密码 + TOTP，密码使用 Argon2id，TOTP 为强制二次因子；
5. 浏览器会话由 Finance Core 服务端持有，cookie 固定为 `__Host-finance_session; Secure; HttpOnly; SameSite=Strict; Path=/`；unsafe API 还要求 CSRF token；
6. 浏览器提交的 `household_id` 不是授权来源；家庭范围只能来自已认证 session context；
7. Finance 登录按源 IP 与 normalized username 双维度限流。只有 TCP peer 属于参考部署固定 Caddy `172.30.0.10/32` 时才读取其转发客户端 IP，任意直连请求不能通过伪造 `X-Forwarded-For` 绕过限流；
8. MCP 使用独立 Bearer token 边界，浏览器 cookie 不能认证 `/mcp`，MCP Bearer 也不能认证浏览器 API；MCP 默认关闭；
9. ezBookkeeping 启用 2FA、关闭常态公开注册、限制登录失败次数、将 API token 来源限制为 Finance Core `172.30.0.30`，trusted proxy 仅允许 Caddy `172.30.0.10/32`；
10. `FINANCE_AUTH_KEY_FILE`、`FINANCE_ADMIN_PASSWORD_FILE`、`EBK_API_TOKEN_FILE` 和 ezBookkeeping signing secret 都通过仓库外私有文件挂载；普通 `.env` 不保存这些 secret；
11. `.env` 及 secret files 均要求 group/other 无权限；部署 preflight 对文件类型、权限、仓库外路径和 steady-state 注册状态 fail closed；
12. 主机防火墙只开放 22（受控来源）/80/443；SSH key-only，关闭密码登录（如运维条件允许）；
13. 每日创建可恢复备份，并通过独立 backup/maintenance host 维持 append-only 离站恢复边界；
14. 依赖和镜像按计划升级；
15. AI 写操作默认禁用，确定性计算与授权逻辑不交给 LLM 决定。

### Finance 登录与恢复边界

首次管理员由离线 bootstrap 创建；bootstrap 只读取 `FINANCE_ADMIN_PASSWORD_FILE`，不会把明文密码打印到日志。管理员第一次浏览器登录必须完成 TOTP enrollment，并把一次性返回的 recovery codes 保存到独立密码管理器或等价受保护介质。数据库只保存 TOTP 密文、恢复码 hash、session token hash 与必要的加密 CSRF 状态，不保存这些材料的明文。

密码重置与 TOTP 重置是服务器侧离线维护操作，不暴露成公网 HTTP 管理接口。密码或 TOTP 被重置时，既有浏览器 sessions 会被撤销。

## Prompt Injection

任何以下字段：
- 商户名；
- 交易备注；
- OCR 文本；
- PDF/合同文本；
- 网页抓取文本；

都必须放在结构化 `data` 中，不能拼接到 system/developer prompt 的指令区域。

## 第三方 LLM 隐私

V1 采取最小披露：只有完成当前建议所需的结构化字段发送给云模型。截图识别如果直接使用 ezBookkeeping 的第三方 VLM，要在配置/使用说明中接受其隐私权衡。

V1.2 增加 Strict Privacy：原图留在自有网络，本地 OCR/脱敏后才把必要结构发送到云端。

## 未来家庭 RBAC

不要以“一个家庭都互相信任”做永久假设。V1.4 引入成员权限后，Finance Tool 的每次查询先做服务端 authorization，LLM 永远不能自行决定可见范围。
