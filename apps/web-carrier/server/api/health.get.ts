export default defineEventHandler(() => {
  return {
    status: 'ok',
    service: 'web-carrier',
    timestamp: new Date().toISOString(),
  }
})
