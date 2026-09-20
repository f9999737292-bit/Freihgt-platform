import { isRfxExcelExchangeEnabled } from '~/utils/buyerXlsxFeatureFlag'

export function useRfxExcelExchangeFeature() {
  const config = useRuntimeConfig()
  const enabled = computed(() =>
    isRfxExcelExchangeEnabled(config.public.rfxExcelExchangeEnabled)
    || isRfxExcelExchangeEnabled(import.meta.env.NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED),
  )

  return { enabled }
}
