import type { AuthUser } from '~/types/api'
import type { PreviewRuleContext } from '~/types/lowCode'
import { mergePreviewRuleContext } from '~/types/lowCode'

type UserWithRbac = AuthUser & {
  roles?: string[]
}

export function useLowCodePreviewContext() {
  const authStore = useAuthStore()
  const { isPlatformAdmin } = usePermissions()

  function resolvePreviewRole(): string | undefined {
    if (isPlatformAdmin()) return 'PLATFORM_ADMIN'
    const user = authStore.user as UserWithRbac | null
    const roles = user?.roles ?? []
    return roles[0]?.trim() || undefined
  }

  function buildPreviewContext(
    entityStatus?: string | null,
    override?: PreviewRuleContext,
  ): PreviewRuleContext | undefined {
    return mergePreviewRuleContext(
      {
        entity_status: entityStatus?.trim() || undefined,
        role: resolvePreviewRole(),
      },
      override,
    )
  }

  return {
    buildPreviewContext,
    resolvePreviewRole,
  }
}
