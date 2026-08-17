import type { MoneyDTO } from './types'

const currencyDigits: Record<string, number> = {
  CNY: 2,
  USD: 2,
  EUR: 2,
  GBP: 2,
  HKD: 2,
  JPY: 0,
  KRW: 0,
  KWD: 3,
  BHD: 3,
}

const currencyMarks: Record<string, string> = {
  CNY: '¥',
  USD: '$',
  EUR: '€',
  GBP: '£',
  HKD: 'HK$',
  JPY: '¥',
  KRW: '₩',
}

export function formatMoney(value: MoneyDTO): string {
  const digits = currencyDigits[value.currency] ?? 2
  const minor = BigInt(value.minor)
  const negative = minor < 0n
  const absolute = negative ? -minor : minor
  const scale = 10n ** BigInt(digits)
  const whole = absolute / scale
  const fraction = absolute % scale
  const wholeText = groupDigits(whole.toString())
  const fractionText = digits === 0 ? '' : `.${fraction.toString().padStart(digits, '0')}`
  const mark = currencyMarks[value.currency] ?? `${value.currency} `
  return `${negative ? '-' : ''}${mark}${wholeText}${fractionText}`
}

export function formatPercent(ratio?: string, fractionDigits = 1): string {
  if (!ratio) return '—'
  if (!Number.isInteger(fractionDigits) || fractionDigits < 0 || fractionDigits > 6) return '—'

  const normalized = ratio.trim()
  if (!/^-?\d+(?:\.\d+)?$/.test(normalized)) return '—'

  const negative = normalized.startsWith('-')
  const unsigned = negative ? normalized.slice(1) : normalized
  const [whole, fraction = ''] = unsigned.split('.')
  const numerator = BigInt(`${whole}${fraction}`)
  const shift = 2 + fractionDigits - fraction.length

  let scaled: bigint
  if (shift >= 0) {
    scaled = numerator * 10n ** BigInt(shift)
  } else {
    const divisor = 10n ** BigInt(-shift)
    const quotient = numerator / divisor
    const remainder = numerator % divisor
    scaled = remainder * 2n >= divisor ? quotient + 1n : quotient
  }

  const digits = scaled.toString().padStart(fractionDigits + 1, '0')
  const sign = negative && scaled !== 0n ? '-' : ''
  if (fractionDigits === 0) return `${sign}${digits}%`

  const integerPart = digits.slice(0, -fractionDigits)
  const decimalPart = digits.slice(-fractionDigits)
  return `${sign}${integerPart}.${decimalPart}%`
}

export function chartValue(value: MoneyDTO): number {
  const digits = currencyDigits[value.currency] ?? 2
  return Number(BigInt(value.minor)) / 10 ** digits
}

function groupDigits(value: string): string {
  return value.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
}
