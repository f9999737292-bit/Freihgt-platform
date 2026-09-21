export function applyInputModel(raw: string, numberModifier: boolean): string | number | null {
  if (!numberModifier) return raw
  const trimmed = raw.trim()
  if (trimmed === '') return null
  const parsed = Number(trimmed)
  return Number.isFinite(parsed) ? parsed : null
}
