import { CARRIER_XLSX_IDEMPOTENCY_KEY_MAX_LENGTH } from '~/utils/carrierXlsxApiRoutes'

export const CARRIER_XLSX_COMMIT_KEY_PREFIX = 'carrier-xlsx-commit:'

export function createCarrierXlsxIdempotencyStore() {
  const keys = new Map<string, string>()

  function keyForAnalysis(analysisId: string): string {
    const existing = keys.get(analysisId)
    if (existing) return existing
    const next = `${CARRIER_XLSX_COMMIT_KEY_PREFIX}${analysisId}`
    if (next.length > CARRIER_XLSX_IDEMPOTENCY_KEY_MAX_LENGTH) {
      throw new Error('Carrier XLSX idempotency key exceeds 128 characters')
    }
    keys.set(analysisId, next)
    return next
  }

  function rememberPreview(analysisId: string | undefined): string | null {
    if (!analysisId) return null
    if (keys.has(analysisId)) return keys.get(analysisId) ?? null
    return keyForAnalysis(analysisId)
  }

  function reset(): void {
    keys.clear()
  }

  return {
    keyForAnalysis,
    rememberPreview,
    reset,
  }
}
