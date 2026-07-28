import type { AuthUser } from '~/types/api'

export type ProductRole =
  | 'admin'
  | 'shipper'
  | 'carrier'
  | 'forwarder'
  | 'consignee'
  | 'finance'
  | 'procurement'

const IDENTITY_TO_PRODUCT: Record<string, ProductRole> = {
  PLATFORM_ADMIN: 'admin',
  SHIPPER_ADMIN: 'shipper',
  SHIPPER_LOGIST: 'shipper',
  CARRIER_ADMIN: 'carrier',
  CARRIER_DISPATCHER: 'carrier',
  FORWARDER_MANAGER: 'forwarder',
  CONSIGNEE_OPERATOR: 'consignee',
  CONSIGNEE_VIEWER: 'consignee',
  FINANCE_MANAGER: 'finance',
  PROCUREMENT_MANAGER: 'procurement',
}

const ALL_NAV_ROUTES = [
  '/dashboard',
  '/control-tower',
  '/companies',
  '/users',
  '/transport-orders',
  '/freight-requests',
  '/rfx',
  '/shipments',
  '/documents',
  '/billing-registers',
  '/low-code',
  '/health',
  '/settings',
] as const

const ROLE_ROUTES: Record<ProductRole, readonly string[]> = {
  admin: ALL_NAV_ROUTES,
  shipper: [
    '/dashboard',
    '/control-tower',
    '/transport-orders',
    '/freight-requests',
    '/rfx',
    '/shipments',
    '/documents',
    '/billing-registers',
    '/companies',
    '/users',
    '/settings',
  ],
  carrier: [
    '/dashboard',
    '/shipments',
    '/transport-orders',
    '/freight-requests',
    '/rfx',
    '/documents',
    '/billing-registers',
    '/companies',
    '/users',
    '/settings',
  ],
  forwarder: [
    '/dashboard',
    '/control-tower',
    '/transport-orders',
    '/freight-requests',
    '/rfx',
    '/shipments',
    '/documents',
    '/billing-registers',
    '/companies',
    '/users',
    '/settings',
  ],
  consignee: ['/dashboard', '/shipments', '/documents', '/companies', '/settings'],
  finance: [
    '/dashboard',
    '/control-tower',
    '/documents',
    '/billing-registers',
    '/transport-orders',
    '/shipments',
    '/companies',
    '/users',
    '/settings',
    '/health',
  ],
  procurement: [
    '/dashboard',
    '/control-tower',
    '/freight-requests',
    '/rfx',
    '/transport-orders',
    '/shipments',
    '/documents',
    '/billing-registers',
    '/companies',
    '/users',
    '/settings',
  ],
}

const LANDING_ROUTES: Record<ProductRole, string> = {
  admin: '/dashboard',
  shipper: '/dashboard',
  carrier: '/shipments',
  forwarder: '/freight-requests',
  consignee: '/shipments',
  finance: '/billing-registers',
  procurement: '/freight-requests',
}

const LANDING_ROLE_PRIORITY: ProductRole[] = [
  'admin',
  'finance',
  'procurement',
  'forwarder',
  'carrier',
  'shipper',
  'consignee',
]

function currentUser(): AuthUser | null {
  const authStore = useAuthStore()
  return authStore.user
}

function userRoles(): string[] {
  const user = currentUser()
  return user?.roles ?? []
}

function userPermissions(): string[] {
  const user = currentUser() as (AuthUser & { permissions?: string[] }) | null
  return user?.permissions ?? []
}

function isDevPlatformAdminFallback(): boolean {
  const config = useRuntimeConfig()
  const user = currentUser()
  if (!user?.email) return false
  return config.public.mockAuth === true && user.email.toLowerCase() === 'admin@7rights.local'
}

function hasAdminAccess(): boolean {
  return userRoles().includes('PLATFORM_ADMIN') || isDevPlatformAdminFallback()
}

function normalizeRoute(route: string): string {
  const path = route.split('?')[0]?.split('#')[0]?.replace(/\/$/, '') || '/'
  return path === '' ? '/' : path
}

function getAllowedRoutesForRoles(roles: ProductRole[]): Set<string> {
  const allowed = new Set<string>()
  for (const role of roles) {
    for (const route of ROLE_ROUTES[role]) {
      allowed.add(route)
    }
  }
  return allowed
}

export function usePermissions() {
  function hasRole(role: string): boolean {
    if (isDevPlatformAdminFallback()) return true
    return userRoles().includes(role)
  }

  function hasAnyRole(roles: string[]): boolean {
    if (isDevPlatformAdminFallback()) return true
    return roles.some((role) => userRoles().includes(role))
  }

  function hasPermission(permission: string): boolean {
    if (isDevPlatformAdminFallback()) return true
    return userPermissions().includes(permission)
  }

  function hasAnyPermission(permissions: string[]): boolean {
    if (isDevPlatformAdminFallback()) return true
    return permissions.some((permission) => userPermissions().includes(permission))
  }

  function isPlatformAdmin(): boolean {
    return hasRole('PLATFORM_ADMIN') || isDevPlatformAdminFallback()
  }

  function getProductRoles(): ProductRole[] {
    if (isDevPlatformAdminFallback() || hasAdminAccess()) {
      return ['admin']
    }

    const roles = new Set<ProductRole>()
    for (const identityRole of userRoles()) {
      const productRole = IDENTITY_TO_PRODUCT[identityRole]
      if (productRole) {
        roles.add(productRole)
      }
    }

    return [...roles]
  }

  function hasProductRole(role: ProductRole): boolean {
    if (role === 'admin' && hasAdminAccess()) {
      return true
    }
    return getProductRoles().includes(role)
  }

  function canAccessRoute(route: string): boolean {
    if (hasAdminAccess() || isDevPlatformAdminFallback()) {
      return true
    }

    const normalized = normalizeRoute(route)
    const allowedRoutes = getAllowedRoutesForRoles(getProductRoles())

    for (const allowedRoute of allowedRoutes) {
      if (normalized === allowedRoute || normalized.startsWith(`${allowedRoute}/`)) {
        return true
      }
    }

    return false
  }

  function canSeeNavItem(route: string): boolean {
    return canAccessRoute(route)
  }

  function getLandingRoute(): string {
    if (hasAdminAccess() || isDevPlatformAdminFallback()) {
      return LANDING_ROUTES.admin
    }

    const productRoles = getProductRoles()
    if (productRoles.length === 0) {
      return LANDING_ROUTES.admin
    }

    for (const role of LANDING_ROLE_PRIORITY) {
      if (productRoles.includes(role)) {
        return LANDING_ROUTES[role]
      }
    }

    return LANDING_ROUTES.admin
  }

  return {
    hasRole,
    hasAnyRole,
    hasPermission,
    hasAnyPermission,
    isPlatformAdmin,
    getProductRoles,
    hasProductRole,
    canAccessRoute,
    canSeeNavItem,
    getLandingRoute,
  }
}
