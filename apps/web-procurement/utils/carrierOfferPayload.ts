export function parseOfferAmount(value: unknown): number | null {
  if (value == null || value === '') return null
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed) || parsed < 0) return null
  return parsed
}

export function buildLotOfferLines(
  lots: Array<{ id: string }>,
  amounts: Record<string, unknown>,
  currencyCode: string,
): { ok: true; lines: Array<{ rfx_lot_id: string; amount: number; currency_code: string }> } | { ok: false; missingLotIds: string[] } {
  const lines: Array<{ rfx_lot_id: string; amount: number; currency_code: string }> = []
  const missingLotIds: string[] = []
  for (const lot of lots) {
    const amount = parseOfferAmount(amounts[lot.id])
    if (amount == null) {
      missingLotIds.push(lot.id)
      continue
    }
    lines.push({
      rfx_lot_id: lot.id,
      amount,
      currency_code: currencyCode,
    })
  }
  if (missingLotIds.length > 0) {
    return { ok: false, missingLotIds }
  }
  return { ok: true, lines }
}
