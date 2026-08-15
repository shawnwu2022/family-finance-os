# 财务领域模型

## 1. 聚合边界

### Household
- `Household`
- `Member`
- `HouseholdPreference`
- `LiquidityPolicy`

### IncomeProfile
- `IncomeSource`
- `IncomeStability`
- `ExpectedIncomeSchedule`

### Budget
- `BudgetPlan`
- `BudgetLine`
- `BudgetPeriod`

### Debt
- `DebtContract`
- `DebtPaymentRule`
- `DebtScenario`

### Goal
- `FinancialGoal`
- `GoalContributionPlan`

### Portfolio
- `AssetSnapshot`
- `AssetClassTarget`

### Advisor/Audit
- `AdviceSession`
- `ToolExecution`
- `AdviceRecord`
- `DataQualitySnapshot`

## 2. Money

权威金额类型不使用 `float`。

建议内部契约：

```python
class Money:
    amount_minor: int
    currency: str
```

CNY `100.23` 元表示为 `10023` 分。跨币种换算必须同时记录汇率和价格日期。

## 3. DebtContract 最小字段

```text
id
household_id
name
debt_type
principal_minor
currency
apr
minimum_payment_minor
scheduled_payment_minor
term_remaining_months
next_due_date
rate_type
prepayment_fee_rule
revolving
source_account_ref
active
```

APR 缺失时 Debt Engine 不允许输出“最优利息策略”的确定结论，只能输出缺失数据提醒。

## 4. FinancialGoal 最小字段

```text
id
household_id
name
target_minor
funded_minor
target_date
priority
hard_or_flexible
monthly_contribution_minor
currency
```

## 5. BudgetLine

```text
period
category_ref / semantic_group
planned_minor
kind: essential | discretionary | financial_goal
rollover_policy
```

V1 `rollover_policy=none` 为默认；后续根据真实需求加 rollover。

## 6. 外部账本引用

Finance Core 只保存 `external_account_ref`、`external_category_ref` 等引用，不复制可编辑交易明细。若为性能建立缓存，必须可完全重建并有 `synced_at`。
