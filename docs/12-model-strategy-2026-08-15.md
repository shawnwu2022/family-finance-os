# AI 模型策略 — 2026-08-15

> 这是**当前候选基线**，不是永久模型名单。生产配置只使用角色 ID；每次切换前重新查官方文档、价格和隐私条款。

## 1. 推荐 V1 角色配置

| Role | 当前优先候选 | 备选 | 原因 |
|---|---|---|---|
| `vision_cheap` | **Qwen3.6-Flash** | Qwen3.7-Plus | 阿里云官方视觉文档明确面向图片分析/OCR；推荐先 qwen3.7-plus 验效果，稳定后可用 qwen3.6-flash 降成本 |
| `chat_fast` | **DeepSeek-V4-Flash** | Qwen3.6/3.7 Flash/Plus、GLM | DeepSeek 官方 API 支持 JSON/Tool Calls、思考/非思考；Flash 面向高性价比 |
| `planner_strong` | **DeepSeek-V4-Pro** | GPT-5.6 Terra/Sol、GLM-5.2、Qwen3.7-Plus | 复杂债务/Scenario 需要强推理和 Tool Calling；国内 API 成本/可用性优先 |
| `reviewer` | **与 planner 不同供应商** | GLM-5.2 / GPT-5.6 / Qwen3.7-Plus | 重大决策的 reviewer 价值来自独立性，不应只是同模型重复一次 |

## 2. 当前一手资料事实

### DeepSeek

截至本基线，官方 API 提供：
- `deepseek-v4-flash`；
- `deepseek-v4-pro`；
- 1M context；
- JSON Output；
- Tool Calls；
- 思考/非思考模式。

旧 `deepseek-chat` / `deepseek-reasoner` 已在 2026-07-24 到期，不应再写入新项目配置。

Sources:
- https://api-docs.deepseek.com/zh-cn/
- https://api-docs.deepseek.com/zh-cn/quick_start/pricing/

### Qwen / Alibaba Model Studio

官方当前视觉理解文档推荐从 `qwen3.7-plus` 开始；场景稳定后可尝试 `qwen3.6-flash` 降低成本。Qwen3.6/3.7 支持视觉、OCR、Function Calling/结构化输出（具体能力按模型文档）。

对于“支付截图 → 结构化交易 JSON”，优先做真实数据集评测：金额、商户、付款账户、时间、退款/优惠/实付区分，不只比较通用 benchmark。

Sources:
- https://help.aliyun.com/zh/model-studio/vision-model/
- https://help.aliyun.com/zh/model-studio/qwen3-6-flash
- https://help.aliyun.com/zh/model-studio/qwen3-6-plus

### GLM

智谱官方模型概览当前列出 GLM-5.2，文档描述 1M context，并支持 reasoning effort。适合作为国内独立 reviewer 候选；上线前仍应基于本项目财务 eval 集实测。

Sources:
- https://docs.bigmodel.cn/cn/guide/start/model-overview
- https://docs.bigmodel.cn/cn/guide/models/text/glm-5.2

### OpenAI

OpenAI 当前 API 文档将 GPT-5.6 Sol 定位为复杂专业推理，Terra 平衡智能与成本，Luna 面向成本敏感高吞吐。可作为 planner/reviewer 的高质量备选，但本项目不会因此绑定 OpenAI。

Source:
- https://platform.openai.com/overview

## 3. 不直接按“模型排行榜”路由

Finance OS 应建立自己的 eval：

### Vision Eval（至少 100~300 张经脱敏真实样本后再决定长期模型）
- 实付金额 exact-match；
- 商户名称；
- 时间；
- 账户/卡尾号；
- 退款识别；
- 优惠前金额 vs 实付；
- 多笔交易截图；
- 微信/支付宝/银行不同页面；
- 低亮度/裁剪/长截图。

### Advisor Eval
- 必须调用正确 Tool；
- 不篡改 Tool 数字；
- 能识别数据不完整；
- 债务方案遵循 Liquidity Floor；
- 能区分事实/假设；
- 不给出越权自动交易指令；
- Prompt Injection resistance；
- 中文解释质量；
- 成本/延迟。

## 4. 推荐路由

```text
截图识别
  → qwen vision role

“这个月花多少？”
  → deterministic tool
  → fast model 做一句解释

“现在买 2 万元设备影响多大？”
  → scenario tool
  → planner model

“20 万应该还房贷还是保留/投资？”
  → liquidity + debt + goal + scenario
  → planner strong
  → 高影响时 reviewer（不同模型）
```

## 5. 成本控制

成本优化优先级：
1. 不把整本流水发送给 LLM；
2. SQL/Finance Engine 做聚合；
3. fast model 处理高频解释；
4. planner 只处理复杂决策；
5. reviewer 只处理重大/低置信度问题；
6. 充分利用 provider cache（若符合接口和隐私要求）；
7. V1.2 再基于实测决定 P40 是否承担 OCR/分类。

不要为了省几分钱把“账务正确性”和“复杂债务建议”降级到未经验证的小模型。
