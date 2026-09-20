import { isRfxExcelExchangeEnabled } from '~/utils/buyerXlsxFeatureFlag'

export function useRfxExcelExchangeFeature() {
  const config = useRuntimeConfig()
  const enabled = computed(() => isRfxExcelExchangeEnabled(config.public.rfxExcelExchangeEnabled))

  return { enabled }
}
