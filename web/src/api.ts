import type {
  AdvisorResponse,
  BudgetResponse,
  CashflowResponse,
  DashboardData,
  DebtsResponse,
  GoalsResponse,
  OverviewResponse,
} from './types'

interface APIErrorEnvelope {
  error?: {
    code?: string
    message?: string
  }
}

export async function loadDashboard(householdId: number, period: string): Promise<DashboardData> {
  const household = encodeURIComponent(String(householdId))
  const month = encodeURIComponent(period)
  const [overview, cashflow, budget, debts, goals] = await Promise.all([
    requestJSON<OverviewResponse>(`/api/v1/overview?household_id=${household}`),
    requestJSON<CashflowResponse>(`/api/v1/cashflow?household_id=${household}&period=${month}`),
    requestJSON<BudgetResponse>(`/api/v1/budget?household_id=${household}&period=${month}`),
    requestJSON<DebtsResponse>(`/api/v1/debts?household_id=${household}`),
    requestJSON<GoalsResponse>(`/api/v1/goals?household_id=${household}`),
  ])
  return { overview, cashflow, budget, debts, goals }
}

export async function askAdvisor(householdId: number, question: string, requireReview: boolean): Promise<AdvisorResponse> {
  return requestJSON<AdvisorResponse>('/api/v1/advisor', {
    method: 'POST',
    body: JSON.stringify({
      household_id: householdId,
      question,
      require_tool: true,
      require_review: requireReview,
    }),
  })
}

async function requestJSON<T>(url: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body) headers.set('Content-Type', 'application/json')
  headers.set('Accept', 'application/json')

  const response = await fetch(url, {
    ...init,
    headers,
    credentials: 'same-origin',
    cache: 'no-store',
  })
  if (!response.ok) {
    let code = 'request_failed'
    try {
      const envelope = (await response.json()) as APIErrorEnvelope
      if (envelope.error?.code) code = envelope.error.code
    } catch {
      // Keep the stable generic code; never surface an untrusted response body.
    }
    throw new Error(code)
  }
  return (await response.json()) as T
}
