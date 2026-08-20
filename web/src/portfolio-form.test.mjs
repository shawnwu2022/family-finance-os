import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  buildPortfolioAssetRequest,
  formFromPortfolioAsset,
  isoToLocalDateTimeInput,
  localDateTimeInputToISO,
} from './portfolio-form.ts'

describe('portfolio form contract', () => {
  it('builds a canonical base-currency request without optional empty fields', () => {
    const result = buildPortfolioAssetRequest({
      assetRef: '  property:home  ',
      name: '  Home  ',
      assetClass: 'property',
      amount: '123.45',
      currency: ' cny ',
      sourceCurrency: 'CNY',
      valuationAsOf: '2026-08-19T14:30',
      fxAsOf: '',
      sourceAccountRef: '  ',
      sourceKind: 'manual',
    })

    assert.equal(result.ok, true)
    assert.equal(result.assetRef, 'property:home')
    assert.deepEqual(result.request, {
      name: 'Home',
      asset_class: 'property',
      value_minor: '12345',
      currency: 'CNY',
      source_currency: 'CNY',
      valuation_as_of: localDateTimeInputToISO('2026-08-19T14:30'),
      source_kind: 'manual',
    })
  })

  it('requires an explicit FX timestamp when source currency differs', () => {
    const missingFX = buildPortfolioAssetRequest({
      assetRef: 'gold:usd',
      name: 'Gold',
      assetClass: 'gold',
      amount: '10.00',
      currency: 'CNY',
      sourceCurrency: 'USD',
      valuationAsOf: '2026-08-19T14:30',
      fxAsOf: '',
      sourceAccountRef: ' broker-1 ',
      sourceKind: 'import',
    })
    assert.deepEqual(missingFX, { ok: false, error: 'fx_as_of_required' })

    const withFX = buildPortfolioAssetRequest({
      assetRef: 'gold:usd',
      name: 'Gold',
      assetClass: 'gold',
      amount: '10.00',
      currency: 'CNY',
      sourceCurrency: 'USD',
      valuationAsOf: '2026-08-19T14:30',
      fxAsOf: '2026-08-19T14:20',
      sourceAccountRef: ' broker-1 ',
      sourceKind: 'import',
    })
    assert.equal(withFX.ok, true)
    assert.equal(withFX.request.fx_as_of, localDateTimeInputToISO('2026-08-19T14:20'))
    assert.equal(withFX.request.source_account_ref, 'broker-1')
  })

  it('rejects malformed identity, currency, amount, time, and int64 overflow', () => {
    const base = {
      assetRef: 'asset:1',
      name: 'Asset',
      assetClass: 'equity',
      amount: '1.00',
      currency: 'CNY',
      sourceCurrency: 'CNY',
      valuationAsOf: '2026-08-19T14:30',
      fxAsOf: '',
      sourceAccountRef: '',
      sourceKind: 'manual',
    }

    const cases = [
      [{ ...base, assetRef: '   ' }, 'asset_ref_required'],
      [{ ...base, name: '' }, 'name_required'],
      [{ ...base, currency: 'CN' }, 'invalid_currency'],
      [{ ...base, sourceCurrency: 'usd1' }, 'invalid_source_currency'],
      [{ ...base, amount: '-1' }, 'invalid_amount'],
      [{ ...base, amount: '92233720368547758.08' }, 'amount_out_of_range'],
      [{ ...base, valuationAsOf: 'not-a-time' }, 'invalid_valuation_as_of'],
    ]

    for (const [form, error] of cases) {
      assert.deepEqual(buildPortfolioAssetRequest(form), { ok: false, error })
    }
  })

  it('round-trips browser-local datetime input and maps stored assets into edit form state', () => {
    const local = '2026-08-19T14:30'
    const iso = localDateTimeInputToISO(local)
    assert.ok(iso)
    assert.equal(isoToLocalDateTimeInput(iso), local)

    const form = formFromPortfolioAsset({
      asset_ref: 'broker:fund',
      name: 'Fund',
      asset_class: 'fund',
      value_minor: '12340',
      currency: 'CNY',
      source_currency: 'CNY',
      valuation_as_of: iso,
      source_account_ref: 'broker-1',
      source_kind: 'manual',
    })
    assert.equal(form.assetRef, 'broker:fund')
    assert.equal(form.amount, '123.40')
    assert.equal(form.valuationAsOf, local)
    assert.equal(form.fxAsOf, '')
    assert.equal(form.sourceAccountRef, 'broker-1')
  })
})
