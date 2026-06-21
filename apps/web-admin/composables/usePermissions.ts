import type { AuthUser } from '~/types/api'

// TODO: extend AuthUser with roles[] and permissions[] when /auth/me returns RBAC payload.

type UserWithRbac = AuthUser & {
  roles?: string[]
  permissions?: string[]
}

function currentUser(): UserWithRbac | null {
  const authStore = useAuthStore()
  return authStore.user as UserWithRbac | null
}

function userRoles(): string[] {
  const user = currentUser()
  return user?.roles ?? []
}

function userPermissions(): string[] {
  const user = currentUser()
  return user?.permissions ?? []
}

function isDevPlatformAdminFallback(): boolean {
  const config = useRuntimeConfig()
  const user = currentUser()
  if (!user?.email) return false
  return config.public.mockAuth === true && user.email.toLowerCase() === 'admin@7rights.local'
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

  return {
    hasRole,
    hasAnyRole,
    hasPermission,
    hasAnyPermission,
    isPlatformAdmin,
  }
}
