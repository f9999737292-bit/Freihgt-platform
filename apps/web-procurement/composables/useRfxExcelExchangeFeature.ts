import { isRfxExcelExchangeEnabled } from '~/utils/buyerXlsxFeatureFlag'

const EXCEL_UI_STORAGE_KEY = 'freight_procurement_rfx_excel_exchange'

export function useRfxExcelExchangeFeature() {
  const config = useRuntimeConfig()
  const enabled = computed(() => {
    if (isRfxExcelExchangeEnabled(config.public.rfxExcelExchangeEnabled)) return true
    if (isRfxExcelExchangeEnabled(config.public.excelUiEnabled)) return true
    if (isRfxExcelExchangeEnabled(import.meta.env.NUXT_PUBLIC_RFX_EXCEL_EXCHANGE_ENABLED)) return true
    if (import.meta.client && import.meta.dev) {
      return isRfxExcelExchangeEnabled(localStorage.getItem(EXCEL_UI_STORAGE_KEY))
    }
    return false
  })

  return { enabled }
}
