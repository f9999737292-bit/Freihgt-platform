/** Resolve template metadata i18n map for current locale with fallbacks. */

const LOCALE_FALLBACK_ORDER = ['ru-RU', 'en-US', 'zh-CN'] as const

export function resolveI18nMapValue(
  map: Record<string, string> | null | undefined,
  locale: string,
): string {
  if (!map) return ''
  if (map[locale]?.trim()) return map[locale].trim()
  for (const fallback of LOCALE_FALLBACK_ORDER) {
    if (map[fallback]?.trim()) return map[fallback].trim()
  }
  const first = Object.values(map).find((v) => v?.trim())
  return first?.trim() ?? ''
}

export function emptyI18nMap(locale: string): Record<string, string> {
  return { [locale]: '' }
}
