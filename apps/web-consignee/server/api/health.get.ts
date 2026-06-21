export default defineEventHandler(() => {
  return {
    status: 'ok',
    service: 'web-consignee',
    timestamp: new Date().toISOString(),
  }
})
