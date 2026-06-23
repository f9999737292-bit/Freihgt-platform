export default defineNuxtRouteMiddleware(() => {
  const { isPlatformAdmin } = usePermissions()
  if (isPlatformAdmin()) return

  const { t } = useI18n()
  const toast = useToast()
  toast.pushToast('error', t('lowCode.adminAccessDenied'))
  return navigateTo('/low-code')
})
