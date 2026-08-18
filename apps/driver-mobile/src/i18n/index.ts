import { createI18n } from 'vue-i18n'
import enUS from './en-US.json'
import ruRU from './ru-RU.json'
import zhCN from './zh-CN.json'

export const i18n = createI18n({
  legacy: false,
  locale: 'ru-RU',
  fallbackLocale: 'en-US',
  messages: {
    'ru-RU': ruRU,
    'en-US': enUS,
    'zh-CN': zhCN,
  },
})
