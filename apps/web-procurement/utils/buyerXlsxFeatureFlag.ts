export function isRfxExcelExchangeEnabled(value: unknown): boolean {
  if (value === true || value === 1) return true
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    return normalized === 'true' || normalized === '1' || normalized === 'yes' || normalized === 'on'
  }
  return false
}
