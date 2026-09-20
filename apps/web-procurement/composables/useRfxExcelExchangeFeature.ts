export function useRfxExcelExchangeFeature() {
  const config = useRuntimeConfig()
  const enabled = computed(() => config.public.rfxExcelExchangeEnabled === true)

  return { enabled }
}
