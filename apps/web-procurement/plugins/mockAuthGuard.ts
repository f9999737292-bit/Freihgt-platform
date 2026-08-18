export default defineNuxtPlugin(() => {
  const config = useRuntimeConfig()
  const isProd = process.env.NODE_ENV === 'production'
  if (isProd && config.public.mockAuth) {
    throw new Error('NUXT_PUBLIC_MOCK_AUTH must not be enabled in production builds')
  }
})
