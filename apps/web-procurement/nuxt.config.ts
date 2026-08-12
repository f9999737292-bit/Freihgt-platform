const localeFiles = [
  'common.json',
  'login.json',
  'home.json',
  'nav.json',
  'bid.json',
  'freightRequests.list.json',
  'freightRequests.detail.json',
]

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
      {
        code: 'ru-RU',
        name: 'RU',
        iso: 'ru-RU',
        files: localeFiles.map((file) => `ru-RU/${file}`),
      },
      {
        code: 'en-US',
        name: 'EN',
        iso: 'en-US',
        files: localeFiles.map((file) => `en-US/${file}`),
      },
      {
        code: 'zh-CN',
        name: '中文',
        iso: 'zh-CN',
        files: localeFiles.map((file) => `zh-CN/${file}`),
      },
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
