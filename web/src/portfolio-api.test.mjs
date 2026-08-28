import assert from 'node:assert/strict'
import { afterEach, describe, it } from 'node:test'

import { clearAuthState, bootstrapSession } from './auth.ts'
import { deletePortfolioAsset, listPortfolioAssets, upsertPortfolioAsset } from './api.ts'

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
  clearAuthState()
})

describe('portfolio API', () => {
  it('lists authenticated-household snapshots without client household id', async () => {
    let captured
    globalThis.fetch = async (url, init) => {
      captured = { url, init }
      return new Response(JSON.stringify({ items: [{ asset_ref: 'property:home', value_minor: '100000' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }

    const response = await listPortfolioAssets()
    assert.equal(captured.url, '/api/v1/portfolio/assets')
    assert.ok(!captured.url.includes('household_id'))
    assert.equal(captured.init.credentials, 'same-origin')
    assert.equal(captured.init.cache, 'no-store')
    assert.equal(response.items[0].asset_ref, 'property:home')
  })

  it('upserts with csrf and no browser household id', async () => {
    let captured
    globalThis.fetch = async (url, init = {}) => {
      if (url === '/api/v1/auth/session') {
        return new Response(JSON.stringify({ authenticated: true, username: 'owner', household_id: 7, role: 'owner', csrf_token: 'csrf' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      captured = { url, init }
      return new Response(JSON.stringify({ asset_ref: 'broker/ABC 1', value_minor: '12345' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }
    await bootstrapSession()

    const request = {
      name: 'Broker position',
      asset_class: 'equity',
      value_minor: '12345',
      currency: 'CNY',
      source_currency: 'CNY',
      valuation_as_of: '2026-08-19T06:00:00.000Z',
      source_kind: 'manual',
    }
    const response = await upsertPortfolioAsset('broker/ABC 1', request)

    assert.equal(captured.url, '/api/v1/portfolio/assets/broker%2FABC%201')
    assert.equal(captured.init.method, 'PUT')
    assert.deepEqual(JSON.parse(captured.init.body), request)
    const headers = new Headers(captured.init.headers)
    assert.equal(headers.get('Content-Type'), 'application/json')
    assert.equal(headers.get('X-CSRF-Token'), 'csrf')
    assert.equal(response.value_minor, '12345')
  })

  it('deletes successfully with csrf and without decoding a 204 body', async () => {
    let captured
    globalThis.fetch = async (url, init = {}) => {
      if (url === '/api/v1/auth/session') {
        return new Response(JSON.stringify({ authenticated: true, username: 'owner', household_id: 7, role: 'owner', csrf_token: 'csrf' }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
      captured = { url, init }
      return new Response(null, { status: 204 })
    }
    await bootstrapSession()

    await deletePortfolioAsset('property:home')
    assert.equal(captured.url, '/api/v1/portfolio/assets/property%3Ahome')
    assert.equal(captured.init.method, 'DELETE')
    assert.equal(new Headers(captured.init.headers).get('X-CSRF-Token'), 'csrf')
  })

  it('preserves the stable backend error code', async () => {
    globalThis.fetch = async () => new Response(JSON.stringify({ error: { code: 'invalid_request', message: 'ignored' } }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    })

    await assert.rejects(() => listPortfolioAssets(), /invalid_request/)
  })
})
