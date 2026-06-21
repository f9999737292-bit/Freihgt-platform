export default defineEventHandler(() => {
  return {
    status: 'ok',
    service: 'web-finance',
    timestamp: new Date().toISOString(),
  }
})
