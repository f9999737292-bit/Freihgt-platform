import type { DecimalString } from '~/types/freightCost'

const UNAVAILABLE_DISPLAY = '—'

function integerGroupingSeparator(locale: string): string {
  if (locale.startsWith('ru') || locale.startsWith('zh')) return '\u00a0'
  return ','
}

function decimalSeparator(locale: string): string {
  if (locale.startsWith('ru') || locale.startsWith('zh')) return ','
  return '.'
}

function normalizeUnsignedDecimal(value: string): { sign: '' | '-'; intDigits: string; fracDigits: string } | null {
  const trimmed = value.trim()
  if (!trimmed) return null
  if (!/^-?\d+(\.\d+)?$/.test(trimmed)) return null

  const sign = trimmed.startsWith('-') ? '-' : ''
  const unsigned = sign ? trimmed.slice(1) : trimmed
  const [intPart, fracPart = ''] = unsigned.split('.')
  const intDigits = intPart.replace(/^0+(?=\d)/, '') || '0'
  const fracDigits = fracPart.padEnd(2, '0').slice(0, 2)
  return { sign, intDigits, fracDigits }
}

function groupIntegerDigits(digits: string, locale: string): string {
  if (digits.length <= 3) return digits
  const separator = integerGroupingSeparator(locale)
  const parts: string[] = []
  let remaining = digits
  while (remaining.length > 3) {
    parts.unshift(remaining.slice(-3))
    remaining = remaining.slice(0, -3)
  }
  parts.unshift(remaining)
  return parts.join(separator)
}

export function isNullMoney(amount: DecimalString | null | undefined): boolean {
  return amount == null || amount.trim() === ''
}

export function isExplicitZeroMoney(amount: DecimalString | null | undefined): boolean {
  if (isNullMoney(amount)) return false
  const parsed = normalizeUnsignedDecimal(amount!)
  if (!parsed) return false
  return parsed.intDigits === '0' && /^0+$/.test(parsed.fracDigits)
}

export function formatDecimalMoney(
  amount: DecimalString | null | undefined,
  currency: string,
  locale: string,
): string {
  if (isNullMoney(amount)) return UNAVAILABLE_DISPLAY

  const parsed = normalizeUnsignedDecimal(amount!)
  if (!parsed) return UNAVAILABLE_DISPLAY

  const grouped = groupIntegerDigits(parsed.intDigits, locale)
  const decSep = decimalSeparator(locale)
  const formatted = `${parsed.sign}${grouped}${decSep}${parsed.fracDigits}`
  return `${formatted} ${currency.trim()}`
}

export function formatDecimalPercent(
  percent: DecimalString | null | undefined,
  locale: string,
): string {
  if (isNullMoney(percent)) return UNAVAILABLE_DISPLAY

  const parsed = normalizeUnsignedDecimal(percent!)
  if (!parsed) return UNAVAILABLE_DISPLAY

  const grouped = groupIntegerDigits(parsed.intDigits, locale)
  const decSep = decimalSeparator(locale)
  return `${parsed.sign}${grouped}${decSep}${parsed.fracDigits}%`
}

export function moneyAriaLabel(
  amount: DecimalString | null | undefined,
  currency: string,
  unavailableLabel: string,
): string {
  if (isNullMoney(amount)) return unavailableLabel
  return `${amount} ${currency}`
}

export function unavailableMoneyDisplay(): string {
  return UNAVAILABLE_DISPLAY
}
