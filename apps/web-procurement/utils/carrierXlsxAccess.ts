export const CARRIER_XLSX_READ_ROLES = [
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
] as const

export const CARRIER_XLSX_RESPOND_ROLES = CARRIER_XLSX_READ_ROLES

export function hasCarrierXlsxReadRole(roles: readonly string[]): boolean {
  return roles.some((role) => (CARRIER_XLSX_READ_ROLES as readonly string[]).includes(role))
}

export function hasCarrierXlsxRespondRole(roles: readonly string[]): boolean {
  return roles.some((role) => (CARRIER_XLSX_RESPOND_ROLES as readonly string[]).includes(role))
}

export function canShowCarrierXlsxPanel(input: {
  excelExchangeEnabled: boolean
  roles: readonly string[]
  responseId?: string | null
}): boolean {
  return input.excelExchangeEnabled
    && hasCarrierXlsxReadRole(input.roles)
    && Boolean(String(input.responseId || '').trim())
}

export function canImportCarrierXlsx(input: {
  responseStatus?: string | null
  responseNotEditable?: boolean
}): boolean {
  if (input.responseNotEditable) return false
  return String(input.responseStatus || '').toUpperCase() === 'DRAFT'
}
