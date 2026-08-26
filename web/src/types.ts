export interface MoneyDTO {
  minor: string
  currency: string
}

export interface OverviewResponse {
  data_as_of: string
  quality: string
  net_worth: MoneyDTO
  income: MoneyDTO
  expense: MoneyDTO
  net_cashflow: MoneyDTO
  savings_rate?: string
  safe_to_spend: MoneyDTO
  emergency_months?: string
  total_debt: MoneyDTO
  goal_count: number
  warnings?: string[]
}

export interface CashflowResponse {
  data_as_of: string
  quality: string
  period: string
  income: MoneyDTO
  expense: MoneyDTO
  net_cashflow: MoneyDTO
  savings_rate?: string
  warnings?: string[]
}

export interface BudgetLineResponse {
  kind: string
  external_category_ref?: string
  semantic_group?: string
  planned: MoneyDTO
  actual: MoneyDTO
  remaining: MoneyDTO
  utilization?: string
}

export interface BudgetResponse {
  data_as_of: string
  quality: string
  period: string
  currency: string
  lines: BudgetLineResponse[]
  warnings?: string[]
}

export interface DebtResponse {
  id: number
  name: string
  type: string
  balance: MoneyDTO
  apr?: string
  repayment_type: string
  minimum_payment: MoneyDTO
  scheduled_payment: MoneyDTO
  term_remaining_months: number
  due_day: number
}

export interface DebtsResponse {
  data_as_of: string
  quality: string
  currency: string
  total: MoneyDTO
  items: DebtResponse[]
  warnings?: string[]
}

export interface GoalResponse {
  id: number
  name: string
  target: MoneyDTO
  funded: MoneyDTO
  target_date: string
  priority: number
  flexibility: string
  monthly_contribution: MoneyDTO
  required_monthly: MoneyDTO
  capacity_shortfall: MoneyDTO
  status: string
}

export interface GoalsResponse {
  data_as_of: string
  quality: string
  items: GoalResponse[]
  warnings?: string[]
}

export interface AdvisorResponse {
  text?: string
  reviewed: boolean
  review?: string
  blocked: boolean
  block_reason?: string
  warnings?: string[]
}

export type PortfolioAssetClass =
  | 'cash'
  | 'deposit'
  | 'fixed_income'
  | 'equity'
  | 'fund'
  | 'gold'
  | 'property'
  | 'other'

export type PortfolioSnapshotSourceKind = 'manual' | 'import'

export interface PortfolioAssetUpsertRequest {
  name: string
  asset_class: PortfolioAssetClass
  value_minor: string
  currency: string
  source_currency: string
  valuation_as_of: string
  fx_as_of?: string
  source_account_ref?: string
  source_kind: PortfolioSnapshotSourceKind
}

export interface PortfolioAssetResponse extends PortfolioAssetUpsertRequest {
  asset_ref: string
}

export interface PortfolioAssetsResponse {
  items: PortfolioAssetResponse[]
}

export interface DashboardData {
  overview: OverviewResponse
  cashflow: CashflowResponse
  budget: BudgetResponse
  debts: DebtsResponse
  goals: GoalsResponse
}
