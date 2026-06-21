export default defineEventHandler(() => {
  return {
    status: 'ok',
    service: 'web-procurement',
    timestamp: new Date().toISOString(),
  }
})
