/**
 * Low-code permission matrix (UI v0.1).
 * See docs/LOW_CODE_PERMISSIONS_MATRIX_V0.1.md
 *
 * Backend admin enforcement: LOW_CODE_ADMIN_AUTH_ENABLED + PLATFORM_ADMIN only.
 * Runtime GET/PUT: tenant-scoped; no service-level RBAC when auth-off.
 */

const LOW_CODE_ADMIN_ROLES = ['PLATFORM_ADMIN'] as const

/** Pilot target for runtime custom-field PUT (UI only in v0.1). */
const LOW_CODE_RUNTIME_WRITE_ROLES = [
  'PLATFORM_ADMIN',
  'SHIPPER_ADMIN',
  'SHIPPER_LOGIST',
  'CARRIER_ADMIN',
  'CARRIER_DISPATCHER',
  'PROCUREMENT_MANAGER',
  'FINANCE_MANAGER',
  'CONSIGNEE_OPERATOR',
] as const

/** Pilot target for audit UI (v0.1 UI remains open; see matrix doc). */
const LOW_CODE_AUDIT_VIEW_ROLES = ['PLATFORM_ADMIN', 'SHIPPER_ADMIN', 'CARRIER_ADMIN', 'FINANCE_MANAGER'] as const

export function useLowCodePermissions() {
  const { isPlatformAdmin, hasAnyRole } = usePermissions()
  const { hasTenant } = useTenantContext()

  function canAccessLowCodeAdmin(): boolean {
    return isPlatformAdmin() || hasAnyRole([...LOW_CODE_ADMIN_ROLES])
  }

  function canRunMigrationPreview(): boolean {
    return canAccessLowCodeAdmin()
  }

  function canRunMigrationExecute(): boolean {
    return canAccessLowCodeAdmin()
  }

  function canRunBatchMigrationPreview(): boolean {
    return canAccessLowCodeAdmin()
  }

  function canRunBatchMigrationExecute(): boolean {
    return canAccessLowCodeAdmin()
  }

  /** Runtime custom-field edit on entity detail / values page (UI gate; API tenant-scoped). */
  function canEditCustomFieldsRuntime(): boolean {
    if (!hasTenant.value) return false
    if (isPlatformAdmin()) return true
    return hasAnyRole([...LOW_CODE_RUNTIME_WRITE_ROLES])
  }

  /**
   * Audit page read (v0.1: any authenticated tenant user in UI).
   * Pilot recommendation: restrict to admin/finance roles — use canViewLowCodeAuditStrict().
   */
  function canViewLowCodeAudit(): boolean {
    return hasTenant.value
  }

  function canViewLowCodeAuditStrict(): boolean {
    if (!hasTenant.value) return false
    if (isPlatformAdmin()) return true
    return hasAnyRole([...LOW_CODE_AUDIT_VIEW_ROLES])
  }

  return {
    canAccessLowCodeAdmin,
    canRunMigrationPreview,
    canRunMigrationExecute,
    canRunBatchMigrationPreview,
    canRunBatchMigrationExecute,
    canEditCustomFieldsRuntime,
    canViewLowCodeAudit,
    canViewLowCodeAuditStrict,
    LOW_CODE_ADMIN_ROLES,
    LOW_CODE_RUNTIME_WRITE_ROLES,
    LOW_CODE_AUDIT_VIEW_ROLES,
  }
}
