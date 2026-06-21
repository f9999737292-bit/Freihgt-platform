export default defineNuxtPlugin(() => {
  const { checkGatewayHealth } = useApi()
  checkGatewayHealth()
  if (import.meta.client) {
    setInterval(() => checkGatewayHealth(), 60_000)
  }
})
