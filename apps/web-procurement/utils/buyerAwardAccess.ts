/**
 * Mirrors api-gateway PolicyBuyerManage (rfxrbac.buyerManageRoles).
 * UI-only; server authorization remains authoritative.
 */
export const BUYER_AWARD_MANAGE_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'FORWARDER_MANAGER',
] as const

export function hasBuyerAwardManageRole(roles: readonly string[]): boolean {
  return roles.some((role) => (BUYER_AWARD_MANAGE_ROLES as readonly string[]).includes(role))
}
