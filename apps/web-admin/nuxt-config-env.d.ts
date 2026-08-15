/** Minimal Node process typing for nuxt.config.ts without pulling full @types/node. */
declare const process: {
  env: Record<string, string | undefined>
}
