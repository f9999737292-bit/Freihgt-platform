export default defineEventHandler(() => {
  return {
    status: 'ok',
    service: 'web-admin',
    timestamp: new Date().toISOString(),
  }
})
