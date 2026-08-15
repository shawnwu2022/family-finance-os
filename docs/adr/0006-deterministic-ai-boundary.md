# ADR-0006: 确定性财务引擎与 LLM 严格分层

**Status:** Accepted

## Decision
所有关键金额、预算、债务、目标、Scenario 由 Finance Engine 计算。LLM 只接收结构化 Tool Result 做解释。

## Consequence
AI 响应必须记录 Tool Calls/Data As-of；Tool 失败时不得编造。
