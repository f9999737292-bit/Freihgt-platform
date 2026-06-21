export const supportedLocales = [
  { code: 'ru-RU', name: 'Русский', iso: 'ru-RU', file: 'ru-RU.json' },
  { code: 'en-US', name: 'English', iso: 'en-US', file: 'en-US.json' },
  { code: 'zh-CN', name: '中文', iso: 'zh-CN', file: 'zh-CN.json' },
] as const

export type SupportedLocaleCode = (typeof supportedLocales)[number]['code']

export const defaultLocale: SupportedLocaleCode = 'ru-RU'

/** Shared @nuxtjs/i18n options for freight-platform frontend apps. */
export function createFreightI18nOptions(langDir = '../../packages/i18n/src/locales') {
  return {
    locales: supportedLocales.map(({ code, name, iso, file }) => ({
      code,
      name,
      iso,
      file,
    })),
    lazy: true,
    langDir,
    defaultLocale,
    strategy: 'prefix_except_default' as const,
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'freight_locale',
      redirectOn: 'root' as const,
    },
  }
}
