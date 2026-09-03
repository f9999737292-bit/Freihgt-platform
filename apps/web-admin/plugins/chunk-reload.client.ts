export default defineNuxtPlugin((nuxtApp) => {
  if (!import.meta.client) return

  nuxtApp.hook('app:chunkError', ({ error }) => {
    console.error('[bintrans] Nuxt chunk load failed; reloading for fresh assets', error)
    window.location.reload()
  })

  window.addEventListener('unhandledrejection', (event) => {
    const reason = String(event.reason?.message || event.reason || '')
    if (
      /Failed to fetch dynamically imported module|Importing a module script failed|ChunkLoadError|Loading chunk .* failed/i.test(
        reason,
      )
    ) {
      console.error('[bintrans] dynamic import failed; reloading', reason)
      event.preventDefault()
      window.location.reload()
    }
  })
})
