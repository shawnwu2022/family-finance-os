import assert from 'node:assert/strict'
import { afterEach, describe, it } from 'node:test'

import { deletePortfolioAsset, listPortfolioAssets, upsertPortfolioAsset } from './api.ts'

const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
})

describe('portfolio API', () => {
  it('lists household-scoped snapshots without cache', async () => {
    let captured
    globalThis.fetch = async (url, init) => {
      captured = { url, init }
      return new Response(JSON.stringify({ items: [{ asset_ref: 'property:home', value_minor: '100000' }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }

    const response = await listPortfolioAssets(42)
    assert.equal(captured.url, '/api/v1/portfolio/assets?household_id=42')
    assert.equal(captured.init.credentials, 'same-origin')
    assert.equal(captured.init.cache, 'no-store')
    assert.equal(response.items[0].asset_ref, 'property:home')
  })

  it('upserts using the encoded path asset ref and string minor units', async () => {
    let captured
    globalThis.fetch = async (url, init) => {
      captured = { url, init }
      return new Response(JSON.stringify({ asset_ref: 'broker/ABC 1', value_minor: '12345' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    }

    const request = {
      name: 'Broker position',
      asset_class: 'equity',
      value_minor: '12345',
      currency: 'CNY',
      source_currency: 'CNY',
      valuation_as_of: '2026-08-19T06:00:00.000Z',
      source_kind: 'manual',
    }
    const response = await upsertPortfolioAsset(42, 'broker/ABC 1', request)

    assert.equal(captured.url, '/api/v1/portfolio/assets/broker%2FABC%201?household_id=42')
    assert.equal(captured.init.method, 'PUT')
    assert.deepEqual(JSON.parse(captured.init.body), request)
    assert.equal(new Headers(captured.init.headers).get('Content-Type'), 'application/json')
    assert.equal(response.value_minor, '12345')
  })

  it('deletes successfully without attempting to decode a 204 body', async () => {
    let captured
    globalThis.fetch = async (url, init) => {
      captured = { url, init }
      return new Response(null, { status: 204 })
    }

    await deletePortfolioAsset(42, 'property:home')
    assert.equal(captured.url, '/api/v1/portfolio/assets/property%3Ahome?household_id=42')
    assert.equal(captured.init.method, 'DELETE')
  })

  it('preserves the stable backend error code', async () => {
    globalThis.fetch = async () => new Response(JSON.stringify({ error: { code: 'invalid_request', message: 'ignored' } }), {
      status: 400,
      headers: { 'Content-Type': 'application/json' },
    })

    await assert.rejects(() => listPortfolioAssets(42), /invalid_request/)
  })
})
