import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { chartValue, formatMoney, formatPercent } from './money.ts'

describe('formatMoney', () => {
  it('formats exact minor units without floating-point conversion', () => {
    assert.equal(formatMoney({ minor: '123456789012345', currency: 'CNY' }), '¥1,234,567,890,123.45')
    assert.equal(formatMoney({ minor: '-42', currency: 'KWD' }), '-KWD 0.042')
    assert.equal(formatMoney({ minor: '1234', currency: 'JPY' }), '¥1,234')
  })
})

describe('formatPercent', () => {
  it('moves the decimal point by exactly two places for ratios below one', () => {
    assert.equal(formatPercent('0.4'), '40.0%')
    assert.equal(formatPercent('0.1234'), '12.3%')
    assert.equal(formatPercent('-0.005'), '-0.5%')
  })

  it('handles whole ratios and invalid values deterministically', () => {
    assert.equal(formatPercent('1'), '100.0%')
    assert.equal(formatPercent('1.25', 2), '125.00%')
    assert.equal(formatPercent('not-a-number'), '—')
    assert.equal(formatPercent(), '—')
  })
})

describe('chartValue', () => {
  it('uses the configured currency scale for chart-only numeric conversion', () => {
    assert.equal(chartValue({ minor: '12345', currency: 'CNY' }), 123.45)
    assert.equal(chartValue({ minor: '1234', currency: 'JPY' }), 1234)
  })
})
