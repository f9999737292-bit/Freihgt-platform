const localeFiles = [
  'common.json',
  'login.json',
  'nav.json',
  'tenders.json',
  'carrierTenders.json',
  'orderExecution.json',
  'settlements.json',
  'payments.json',
  'contracts.json',
  'freightCosts.json',
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
  modules: ['@pinia/nuxt', '@nuxtjs/i18n'],
  css: ['~/assets/css/variables.css', '~/assets/css/main.css'],
  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.NUXT_PUBLIC_API_BASE_URL || 'http://localhost:8080',
      defaultTenantId: process.env.NUXT_PUBLIC_DEFAULT_TENANT_ID || '',
      mockAuth: process.env.NUXT_PUBLIC_MOCK_AUTH === 'true',
      contractRateWorkspaceEnabled: process.env.NUXT_PUBLIC_CONTRACT_RATE_WORKSPACE_ENABLED === 'true',
      freightCostWorkspaceEnabled: process.env.NUXT_PUBLIC_FREIGHT_COST_WORKSPACE_ENABLED === 'true',
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
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
    },
  },
})
