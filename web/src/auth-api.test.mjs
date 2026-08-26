import assert from 'node:assert/strict'
import { afterEach, it } from 'node:test'

import {
  beginLogin,
  bootstrapSession,
  clearAuthState,
  currentAuthState,
  logout,
} from './auth.ts'
import { askAdvisor, loadDashboard } from './api.ts'

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
  clearAuthState()
})

it('dashboard contains no household id and session bootstrap selects authenticated state', async () => {
  const requests = []
  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url, init })
    if (url === '/api/v1/auth/session') {
      return new Response(JSON.stringify({ authenticated: true, username: 'owner', household_id: 7, csrf_token: 'csrf' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    return new Response(JSON.stringify({ cashflow: { period: '2026-08' } }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  await bootstrapSession()
  const dashboard = await loadDashboard('2026-08')

  assert.equal(currentAuthState().phase, 'authenticated')
  assert.equal(requests[1].url, '/api/v1/dashboard?period=2026-08')
  assert.ok(!String(requests[1].url).includes('household_id'))
  assert.equal(dashboard.cashflow.period, '2026-08')
})

it('unsafe finance request adds csrf after authentication and 401 clears auth state', async () => {
  const requests = []
  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url, init })
    if (url === '/api/v1/auth/session') {
      return new Response(JSON.stringify({ authenticated: true, username: 'owner', household_id: 7, csrf_token: 'csrf' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    if (url === '/api/v1/advisor') {
      return new Response(JSON.stringify({ blocked: false, reviewed: false }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    return new Response(JSON.stringify({ error: { code: 'unauthenticated' } }), {
      status: 401,
      headers: { 'Content-Type': 'application/json' },
    })
  }

  await bootstrapSession()
  await askAdvisor('test', true)
  assert.equal(new Headers(requests[1].init.headers).get('X-CSRF-Token'), 'csrf')

  await assert.rejects(() => loadDashboard('2026-08'), /unauthenticated/)
  assert.equal(currentAuthState().phase, 'login')
  assert.equal(currentAuthState().csrfToken, '')
})

it('login selects totp state and logout clears csrf state', async () => {
  const requests = []
  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url, init })
    if (url === '/api/v1/auth/login') {
      return new Response(JSON.stringify({ challenge: 'challenge', step: 'verify_totp' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    if (url === '/api/v1/auth/session') {
      return new Response(JSON.stringify({ authenticated: true, username: 'owner', household_id: 7, csrf_token: 'csrf' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    if (url === '/api/v1/auth/logout') return new Response(null, { status: 204 })
    throw new Error(`unexpected request ${url}`)
  }

  const step = await beginLogin('owner', 'password')
  assert.equal(step.phase, 'verify_totp')
  assert.equal(step.challenge, 'challenge')

  await bootstrapSession()
  await logout()
  const logoutRequest = requests.find((item) => item.url === '/api/v1/auth/logout')
  assert.equal(new Headers(logoutRequest.init.headers).get('X-CSRF-Token'), 'csrf')
  assert.equal(currentAuthState().phase, 'login')
  assert.equal(currentAuthState().csrfToken, '')
})

it('unauthenticated bootstrap selects login state', async () => {
  globalThis.fetch = async () => new Response(JSON.stringify({ authenticated: false }), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  })
  const result = await bootstrapSession()
  assert.equal(result.phase, 'login')
  assert.equal(result.authenticated, false)
})
