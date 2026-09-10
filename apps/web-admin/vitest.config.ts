import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '~': fileURLToPath(new URL('.', import.meta.url)),
    },
  },
  test: {
    environment: 'node',
    setupFiles: ['tests/setup/vue-auto-imports.ts'],
    environmentMatchGlobs: [
      ['tests/rfxE6ComponentAcceptance.test.ts', 'jsdom'],
    ],
    include: ['tests/**/*.test.ts'],
  },
})
