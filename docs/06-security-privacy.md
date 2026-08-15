# 安全与隐私

## V1 威胁模型

保护目标：账本数据、债务/收入/资产信息、交易附件、API Token、LLM API Key、登录凭据。

主要风险：
- VPS 暴露面；
- 弱密码/凭据泄漏；
- Docker/组件过期；
- 数据库公网开放；
- LLM 第三方数据暴露；
- Prompt Injection；
- 误操作/AI 写操作；
- 备份不可恢复。

## V1 控制

1. Caddy 是唯一公网入口；
2. PostgreSQL 不绑定 host 公网端口；
3. ezBookkeeping / Finance Core 只在 Docker network 内接受反代；
4. ezBookkeeping 开启 2FA；
5. `EBK_SECURITY_SECRET_KEY` 上线前生成；
6. API Token 仅供 Finance Core 使用；
7. `.env` 权限 600；不提交 Git；
8. 主机防火墙只开放 22（受控来源）/80/443；
9. SSH key-only，关闭密码登录（如运维条件允许）；
10. 每日备份异地；
11. 依赖和镜像按计划升级；
12. AI 写操作默认禁用。

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
