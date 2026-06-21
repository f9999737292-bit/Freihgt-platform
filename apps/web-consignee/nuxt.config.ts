import { createFreightI18nOptions } from '@freight-platform/i18n/config'

export default defineNuxtConfig({
  compatibilityDate: '2024-11-01',
  devtools: { enabled: true },
  devServer: {
    port: 3003,
  },
  typescript: {
    strict: true,
  },
  modules: ['@nuxtjs/i18n'],
  i18n: createFreightI18nOptions(),
  app: {
    head: {
      title: 'Freight Platform Consignee',
    },
  },
})
