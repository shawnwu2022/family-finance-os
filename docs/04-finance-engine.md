# Finance Engine 计算规范

## 1. 核心原则

- 所有计算函数是纯函数或可复现函数；
- 输入/输出使用明确 schema；
- 每个指标有定义、时间窗口、币种、数据质量；
- 测试包含真实家庭常见边界；
- LLM 不重新计算结果。

## 2. Cashflow

建议基础定义（后续可版本化）：

```text
net_cashflow = recognized_income - recognized_expense
savings_rate = max(net_cashflow, 0) / recognized_income   # recognized_income > 0 时
```

“recognized” 的关键在于过滤 transfer、balance adjustment、借款提款、投资账户内部移动等非经营收入/消费事件。实现前必须用 ezBookkeeping Transaction Type 与真实导入样本建立映射测试。

## 3. Net Worth

```text
net_worth = Σ assets_current_value - Σ liabilities_current_balance
```

投资/房产若不是实时价格，显示 valuation date。不得将陈旧估值伪装成实时净资产。

## 4. Emergency Fund

```text
emergency_months = liquid_emergency_eligible_assets / monthly_essential_burn
```

哪些资产可计入 `liquid_emergency_eligible_assets` 必须由用户策略配置，不默认把股票、房产计入。

## 5. Safe-to-Spend

V1 推荐公式框架：

```text
spendable_pool
= liquid_discretionary_pool
- upcoming_mandatory_expenses
- debt_commitments
- essential_reserve_until_period_end
- emergency_fund_gap_reserved
- hard_goal_contributions
```

返回值必须同时返回分项，不只返回单个数字。

若结果 < 0，显示为“资金缺口”，而不是强行置零隐藏问题。

## 6. Debt Avalanche

算法：
1. 先确保所有债务最低还款；
2. 计算可用于额外还款的资金，不允许侵占 Liquidity Floor；
3. 按有效 APR 从高到低分配额外还款；
4. 每月递推本金和利息；
5. 清偿后把释放的月供滚入下一笔；
6. 输出总利息、还清日期、每月计划。

Snowball 改为按当前余额从小到大排序。

### 必测边界
- 0% 分期；
- 提前还款手续费；
- APR 相同；
- 最低还款高于剩余本金；
- 用户额外资金不足；
- 利率未知；
- 循环信用卡；
- 房贷固定月供。

## 7. Goal Projection

简单确定性 V1：

```text
required_monthly = (target - funded) / months_remaining
```

若目标资产有预期收益，不在 V1 默认使用“理想收益率”降低所需储蓄；只有用户显式开启规划假设时才进入 V3 的收益/风险模拟。

## 8. Scenario

任何 scenario 返回：

```json
{
  "scenario": "purchase",
  "assumptions": [],
  "before": {},
  "after": {},
  "deltas": {},
  "violations": [],
  "data_quality": {}
}
```

AI 只能解释该结果。
