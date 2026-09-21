import { canCancelStatus, canPublishStatus, isEditableStatus } from '~/types/rfx'

const TENDER_MANAGE_ROLES = [
  'PLATFORM_ADMIN',
  'PROCUREMENT_MANAGER',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'FORWARDER_MANAGER',
] as const

export function hasTenderManageRole(roles: readonly string[]): boolean {
  return roles.includes('PLATFORM_ADMIN') || TENDER_MANAGE_ROLES.some((role) => roles.includes(role))
}

export function tenderWorkspaceActionVisibility(input: {
  status: string
  roles: readonly string[]
}) {
  const canManage = hasTenderManageRole(input.roles)
  return {
    back: true,
    edit: isEditableStatus(input.status) && canManage,
    evaluation: canManage,
    publish: canPublishStatus(input.status) && canManage,
    cancel: canCancelStatus(input.status) && canManage,
  }
}
