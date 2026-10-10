import { vi } from 'vitest'

vi.stubGlobal('useI18n', () => ({
  t: (key: string) => key,
}))
