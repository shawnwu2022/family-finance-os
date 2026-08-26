import assert from 'node:assert/strict'
import { afterEach, it } from 'node:test'

import { loadDashboard } from './api.ts'

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
})

it('loads the dashboard through one bounded backend request', async () => {
  const requests = []
  globalThis.fetch = async (url, init) => {
    requests.push({ url, init })
    return new Response(JSON.stringify({
      overview: {},
      cashflow: { period: '2026-08' },
      budget: { period: '2026-08' },
      debts: { items: [] },
      goals: { items: [] },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } })
  }

  const dashboard = await loadDashboard(42, '2026-08')

  assert.equal(requests.length, 1)
  assert.equal(requests[0].url, '/api/v1/dashboard?household_id=42&period=2026-08')
  assert.equal(requests[0].init.cache, 'no-store')
  assert.equal(dashboard.cashflow.period, '2026-08')
})
