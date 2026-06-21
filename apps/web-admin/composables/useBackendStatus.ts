export type BackendStatus = 'online' | 'offline' | 'checking'

const HEALTH_CHECK_TIMEOUT_MS = 3000

export function useBackendStatus() {
  const config = useRuntimeConfig()
  const uiStore = useUiStore()

  const backendOnline = useState<boolean>('backendOnline', () => false)
  const backendStatus = useState<BackendStatus>('backendStatus', () => 'checking')
  const lastCheckedAt = useState<Date | null>('backendLastCheckedAt', () => null)
  const errorMessage = useState<string | null>('backendErrorMessage', () => null)

  async function checkBackendStatus(): Promise<void> {
    backendStatus.value = 'checking'
    uiStore.setApiGatewayStatus('checking')
    errorMessage.value = null

    const baseUrl = config.public.apiBaseUrl.replace(/\/$/, '')
    const healthUrl = `${baseUrl}/health`
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), HEALTH_CHECK_TIMEOUT_MS)

    try {
      const response = await fetch(healthUrl, {
        method: 'GET',
        headers: { Accept: 'application/json' },
        signal: controller.signal,
      })

      if (response.ok) {
        backendOnline.value = true
        backendStatus.value = 'online'
        uiStore.setApiGatewayStatus('online')
      } else {
        backendOnline.value = false
        backendStatus.value = 'offline'
        uiStore.setApiGatewayStatus('offline')
        errorMessage.value = `Health check returned HTTP ${response.status}`
      }
    } catch (error) {
      backendOnline.value = false
      backendStatus.value = 'offline'
      uiStore.setApiGatewayStatus('offline')
      if (error instanceof Error) {
        errorMessage.value = error.name === 'AbortError' ? 'Health check timed out' : error.message
      } else {
        errorMessage.value = 'Connection failed'
      }
    } finally {
      clearTimeout(timeoutId)
      lastCheckedAt.value = new Date()
    }
  }

  return {
    backendOnline,
    backendStatus,
    lastCheckedAt,
    errorMessage,
    checkBackendStatus,
  }
}
