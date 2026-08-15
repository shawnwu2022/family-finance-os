# ADR-0003: 云服务器为 V1 主节点

**Status:** Accepted

## Decision
ezBookkeeping、Finance Core、PostgreSQL 部署在公网云服务器 behind Caddy。本地服务器用于备份。

## Why
“随时随地可记账/查询/分析”是硬要求，本地断电或家庭宽带不能成为关键依赖。
