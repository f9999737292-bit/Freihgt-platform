export default defineEventHandler(() => {
  return {
    status: 'ok',
    service: 'web-shipper',
    timestamp: new Date().toISOString(),
  }
})
