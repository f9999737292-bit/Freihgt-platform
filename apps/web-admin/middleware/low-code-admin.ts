export default defineNuxtRouteMiddleware(() => {
  const { canAccessLowCodeAdmin } = useLowCodePermissions()
  if (canAccessLowCodeAdmin()) return
  const { t } = useI18n()
  const toast = useToast()
  toast.pushToast('error', t('lowCode.adminAccessDenied'))
  return navigateTo('/low-code')
})
