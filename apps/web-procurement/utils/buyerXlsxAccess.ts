export const BUYER_XLSX_MANAGE_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'FORWARDER_MANAGER',
] as const

export function hasBuyerXlsxManageRole(roles: readonly string[]): boolean {
  return roles.some((role) => (BUYER_XLSX_MANAGE_ROLES as readonly string[]).includes(role))
}

export function canShowBuyerXlsxPanel(input: {
  excelExchangeEnabled: boolean
  roles: readonly string[]
  eventStatus: string
}): boolean {
  return input.excelExchangeEnabled
    && hasBuyerXlsxManageRole(input.roles)
    && input.eventStatus === 'DRAFT'
}
