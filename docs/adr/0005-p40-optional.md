# ADR-0005: Tesla P40 只作为可选 Local AI Worker

**Status:** Accepted

## Decision
V1 不部署 P40 依赖。V1.2 通过 benchmark 决定 OCR/脱敏/小模型是否放 P40。

## Why
P40 属于 legacy CC 6.1；不能让老 GPU 约束整个系统依赖。
