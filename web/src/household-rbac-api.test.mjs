import assert from 'node:assert/strict'
import { afterEach, it } from 'node:test'

import {
  createHouseholdMember,
  disableHouseholdMember,
  listHouseholdMembers,
  updateHouseholdMemberRole,
} from './api.ts'
import { bootstrapSession, clearAuthState } from './auth.ts'

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
  clearAuthState()
})

it('member management uses session csrf for mutations and never sends household id', async () => {
  const requests = []
  globalThis.fetch = async (url, init = {}) => {
    requests.push({ url, init })
    if (url === '/api/v1/auth/session') {
      return new Response(JSON.stringify({
        authenticated: true,
        username: 'owner',
        household_id: 7,
        role: 'owner',
        csrf_token: 'csrf',
      }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }
    if (url === '/api/v1/household/members' && (init.method ?? 'GET') === 'GET') {
      return new Response(JSON.stringify({ items: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
    }
    if (url === '/api/v1/household/members' && init.method === 'POST') {
      return new Response(JSON.stringify({ user_id: 12, username: 'partner', role: 'editor', disabled: false, totp_enrolled: false }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    if (url === '/api/v1/household/members/12' && init.method === 'PATCH') {
      return new Response(JSON.stringify({ user_id: 12, username: 'partner', role: 'viewer', disabled: false, totp_enrolled: false }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    if (url === '/api/v1/household/members/12' && init.method === 'DELETE') {
      return new Response(null, { status: 204 })
    }
    throw new Error(`unexpected request ${url}`)
  }

  await bootstrapSession()
  await listHouseholdMembers()
  await createHouseholdMember({ username: 'partner', password: 'correct horse battery staple', role: 'editor' })
  await updateHouseholdMemberRole(12, 'viewer')
  await disableHouseholdMember(12)

  for (const request of requests.slice(1)) {
    assert.ok(!String(request.url).includes('household_id'))
  }
  const create = requests.find((item) => item.init.method === 'POST' && item.url === '/api/v1/household/members')
  const update = requests.find((item) => item.init.method === 'PATCH')
  const remove = requests.find((item) => item.init.method === 'DELETE')
  for (const request of [create, update, remove]) {
    assert.equal(new Headers(request.init.headers).get('X-CSRF-Token'), 'csrf')
  }
  assert.deepEqual(JSON.parse(create.init.body), {
    username: 'partner',
    password: 'correct horse battery staple',
    role: 'editor',
  })
  assert.deepEqual(JSON.parse(update.init.body), { role: 'viewer' })
})
