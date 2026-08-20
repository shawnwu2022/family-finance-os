import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { chartValue, formatMoney, formatPercent, moneyInputFromMinor, parseMoneyInput } from './money.ts'

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

describe('portfolio money input conversion', () => {
  it('converts human decimal input to exact non-negative minor units', () => {
    assert.equal(parseMoneyInput('123.45', 'CNY'), '12345')
    assert.equal(parseMoneyInput('123.4', 'CNY'), '12340')
    assert.equal(parseMoneyInput(' 001.05 ', 'USD'), '105')
    assert.equal(parseMoneyInput('1234', 'JPY'), '1234')
    assert.equal(parseMoneyInput('0.042', 'KWD'), '42')
    assert.equal(parseMoneyInput('999999999999999999.99', 'CNY'), '99999999999999999999')
  })

  it('rejects ambiguous, negative, scientific, and over-precision input', () => {
    for (const value of ['', '-1', '+1', '1e3', '1,000', '1.001', '.5', '1.']) {
      assert.equal(parseMoneyInput(value, 'CNY'), null, value)
    }
    assert.equal(parseMoneyInput('1.1', 'JPY'), null)
    assert.equal(parseMoneyInput('1.0001', 'KWD'), null)
  })

  it('round-trips stored minor units into an editable decimal string', () => {
    assert.equal(moneyInputFromMinor('12345', 'CNY'), '123.45')
    assert.equal(moneyInputFromMinor('12340', 'CNY'), '123.40')
    assert.equal(moneyInputFromMinor('1234', 'JPY'), '1234')
    assert.equal(moneyInputFromMinor('42', 'KWD'), '0.042')
  })
})
