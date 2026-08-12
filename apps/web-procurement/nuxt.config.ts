export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',
  devtools: { enabled: true },
  devServer: {
    port: 3005,
  },
  typescript: {
    strict: true,
  },
  modules: ['@nuxtjs/i18n'],
  runtimeConfig: {
    public: {
      apiBaseUrl: 'http://localhost:8080',
      defaultTenantId: '',
      mockAuth: false,
    },
  },
  i18n: {
    restructureDir: false,
    locales: [
      { code: 'ru-RU', name: 'RU', iso: 'ru-RU', file: 'ru-RU.json' },
      { code: 'en-US', name: 'EN', iso: 'en-US', file: 'en-US.json' },
      { code: 'zh-CN', name: '中文', iso: 'zh-CN', file: 'zh-CN.json' },
    ],
    lazy: true,
    langDir: 'i18n',
    defaultLocale: 'ru-RU',
    strategy: 'no_prefix',
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'freight_procurement_locale',
      fallbackLocale: 'ru-RU',
    },
  },
  app: {
    head: {
      title: 'Freight Platform Procurement',
    },
  },
})
