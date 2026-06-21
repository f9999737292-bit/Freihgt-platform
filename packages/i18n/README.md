# @freight-platform/i18n

Shared locale files and Nuxt i18n configuration.

## Supported locales

| Code | Language |
|------|----------|
| `ru-RU` | Russian (default) |
| `en-US` | English |
| `zh-CN` | Chinese |

## Usage in Nuxt apps

```ts
import { createFreightI18nOptions } from '@freight-platform/i18n/config'

export default defineNuxtConfig({
  modules: ['@nuxtjs/i18n'],
  i18n: createFreightI18nOptions(),
})
```

Add app-specific keys under `apps/<app>/locales/` when needed.
