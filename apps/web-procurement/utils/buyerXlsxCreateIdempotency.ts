export const BUYER_XLSX_CREATE_COMMIT_KEY_PREFIX = 'buyer-xlsx-create-commit:'

export function createBuyerXlsxCreateIdempotencyStore() {
  const keys = new Map<string, string>()

  function keyForAnalysis(analysisId: string): string {
    const existing = keys.get(analysisId)
    if (existing) return existing
    const next = `${BUYER_XLSX_CREATE_COMMIT_KEY_PREFIX}${analysisId}`
    keys.set(analysisId, next)
    return next
  }

  function rememberPreview(analysisId: string | undefined): string | null {
    if (!analysisId) return null
    if (keys.has(analysisId)) return keys.get(analysisId) ?? null
    return keyForAnalysis(analysisId)
  }

  function currentKey(analysisId: string | undefined): string | null {
    if (!analysisId) return null
    return keys.get(analysisId) ?? null
  }

  function reset(): void {
    keys.clear()
  }

  return {
    keyForAnalysis,
    rememberPreview,
    currentKey,
    reset,
  }
}
