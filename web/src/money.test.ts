import { describe, expect, it } from 'vitest'

import { chartValue, formatMoney, formatPercent } from './money'

describe('formatMoney', () => {
  it('formats exact minor units without floating-point conversion', () => {
    expect(formatMoney({ minor: '123456789012345', currency: 'CNY' })).toBe('¥1,234,567,890,123.45')
    expect(formatMoney({ minor: '-42', currency: 'KWD' })).toBe('-KWD 0.042')
    expect(formatMoney({ minor: '1234', currency: 'JPY' })).toBe('¥1,234')
  })
})

describe('formatPercent', () => {
  it('moves the decimal point by exactly two places for ratios below one', () => {
    expect(formatPercent('0.4')).toBe('40.0%')
    expect(formatPercent('0.1234')).toBe('12.3%')
    expect(formatPercent('-0.005')).toBe('-0.5%')
  })

  it('handles whole ratios and invalid values deterministically', () => {
    expect(formatPercent('1')).toBe('100.0%')
    expect(formatPercent('1.25', 2)).toBe('125.00%')
    expect(formatPercent('not-a-number')).toBe('—')
    expect(formatPercent()).toBe('—')
  })
})

describe('chartValue', () => {
  it('uses the configured currency scale for chart-only numeric conversion', () => {
    expect(chartValue({ minor: '12345', currency: 'CNY' })).toBe(123.45)
    expect(chartValue({ minor: '1234', currency: 'JPY' })).toBe(1234)
  })
})
