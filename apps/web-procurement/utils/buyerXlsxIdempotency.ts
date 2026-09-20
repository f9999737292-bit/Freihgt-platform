const COMMIT_KEY_PREFIX = 'buyer-xlsx-commit:'

export function createBuyerXlsxIdempotencyStore() {
  const keys = new Map<string, string>()

  function keyForAnalysis(analysisId: string): string {
    const existing = keys.get(analysisId)
    if (existing) return existing
    const next = `${COMMIT_KEY_PREFIX}${analysisId}`
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
