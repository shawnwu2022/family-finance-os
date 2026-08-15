# ADR-0002: Finance Core 使用模块化单体

**Status:** Accepted

## Decision
V1 所有家庭画像、预算、债务、目标、Scenario、Advisor、Report 在一个 Go Finance Core 模块化单体中按 package 边界隔离。

## Consequence
不使用 Redis/Kafka/独立 worker/MCP service；只有出现可测量的独立扩缩容/故障域需求才拆。
