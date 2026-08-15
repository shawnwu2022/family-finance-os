# ADR-0001: ezBookkeeping 作为交易账本

**Status:** Accepted

## Decision
V1 使用 ezBookkeeping 作为账户/交易/分类/标签/附件的唯一可编辑权威账本。Finance Core 不创建第二套交易写模型。

## Why
当前官方已经覆盖中国账单导入、移动端、AI 截图、HTTP API、对账；复制造成高开发/数据一致性成本。

## Consequence
Finance Core 必须用 Adapter 隔离 ezBookkeeping API；未来可替换账本，但需要迁移 Adapter。
