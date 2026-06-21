export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',
  devtools: { enabled: true },
  devServer: {
    port: 3000,
    host: '127.0.0.1',
  },
  typescript: {
    strict: true,
  },
  modules: ['@pinia/nuxt', '@nuxtjs/i18n', '@nuxt/eslint'],
  css: ['~/assets/css/variables.css', '~/assets/css/main.css'],
  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.NUXT_PUBLIC_API_BASE_URL || 'http://localhost:8080',
      appName: process.env.NUXT_PUBLIC_APP_NAME || 'Freight Platform Admin',
      defaultLocale: process.env.NUXT_PUBLIC_DEFAULT_LOCALE || 'ru-RU',
      defaultTenantId: process.env.NUXT_PUBLIC_DEFAULT_TENANT_ID || '',
      mockAuth: process.env.NUXT_PUBLIC_MOCK_AUTH === 'true',
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
      cookieKey: 'freight_admin_locale',
      fallbackLocale: 'ru-RU',
    },
  },
  app: {
    head: {
      title: '7Rights Freight Platform Admin',
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
    },
  },
})
