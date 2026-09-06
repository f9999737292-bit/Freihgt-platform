import type {
  V3ResponseScoreView,
  V3ScoreExplanationResponse,
  V3ScoreLoadState,
} from '~/types/rfx-score'
import { ApiError } from '~/utils/apiClient'

export function useRfxScoreApi() {
  const { apiGet } = useApi()
  const authStore = useAuthStore()
  const tenantStore = useTenantStore()
  const scoreCache = reactive(new Map<string, { state: V3ScoreLoadState; score?: V3ResponseScoreView | null }>())

  function cacheKey(eventId: string, responseId: string) {
    return `${eventId}:${responseId}`
  }

  async function fetchResponseScore(eventId: string, responseId: string): Promise<V3ResponseScoreView> {
    const config = useRuntimeConfig()
    const base = config.public.apiBaseUrl.replace(/\/$/, '')
    const url = `${base}/api/v1/rfx-events/${encodeURIComponent(eventId)}/responses/${encodeURIComponent(responseId)}/score`
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${authStore.token}`,
        'X-Company-ID': tenantStore.currentCompanyId || '',
        'X-Tenant-ID': tenantStore.tenantId,
        'X-User-ID': authStore.user?.id || '',
        Accept: 'application/json',
      },
    })
    if (!response.ok) {
      let body: { error?: { code?: string; message?: string } } | null = null
      try {
        body = await response.json()
      } catch {
        /* ignore */
      }
      throw new ApiError(response.status, {
        code: body?.error?.code || 'INTERNAL_ERROR',
        message: body?.error?.message || response.statusText || 'Request failed',
        details: {},
      })
    }
    return response.json() as Promise<V3ResponseScoreView>
  }

  async function getResponseScore(eventId: string, responseId: string): Promise<V3ResponseScoreView | null> {
    const key = cacheKey(eventId, responseId)
    scoreCache.set(key, { state: 'LOADING' })
    try {
      const data = await fetchResponseScore(eventId, responseId)
      scoreCache.set(key, { state: resolveScoreState(data), score: data })
      return data
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        scoreCache.set(key, { state: 'NOT_AVAILABLE', score: null })
        return null
      }
      scoreCache.set(key, { state: 'FAILED', score: null })
      throw err
    }
  }

  async function getResponseScoreExplanation(
    eventId: string,
    responseId: string,
  ): Promise<V3ScoreExplanationResponse> {
    const config = useRuntimeConfig()
    const base = config.public.apiBaseUrl.replace(/\/$/, '')
    const url = `${base}/api/v1/rfx-events/${encodeURIComponent(eventId)}/responses/${encodeURIComponent(responseId)}/score/explanation`
    const response = await fetch(url, {
      method: 'GET',
      headers: {
        Authorization: `Bearer ${authStore.token}`,
        'X-Company-ID': tenantStore.currentCompanyId || '',
        'X-Tenant-ID': tenantStore.tenantId,
        'X-User-ID': authStore.user?.id || '',
        Accept: 'application/json',
      },
    })
    if (!response.ok) {
      throw new ApiError(response.status, {
        code: 'INTERNAL_ERROR',
        message: response.statusText || 'Request failed',
        details: {},
      })
    }
    return response.json() as Promise<V3ScoreExplanationResponse>
  }

  async function hasPublishedScoreModel(eventId: string): Promise<boolean> {
    try {
      const data = await apiGet<{ model?: { status?: string } }>(`/api/v1/rfx-events/${eventId}/score-model`)
      return data.model?.status === 'PUBLISHED'
    } catch (err) {
      if (err instanceof ApiError && (err.status === 404 || err.status === 403)) return false
      throw err
    }
  }

  function getCachedScore(eventId: string, responseId: string) {
    return scoreCache.get(cacheKey(eventId, responseId))
  }

  function invalidateScores(eventId: string) {
    for (const key of scoreCache.keys()) {
      if (key.startsWith(`${eventId}:`)) scoreCache.delete(key)
    }
  }

  return {
    scoreCache,
    getResponseScore,
    getResponseScoreExplanation,
    hasPublishedScoreModel,
    getCachedScore,
    invalidateScores,
  }
}

export function resolveScoreState(data: V3ResponseScoreView | null): V3ScoreLoadState {
  if (!data?.qualification) return 'NOT_AVAILABLE'
  const calc = data.qualification.calculation_status
  if (calc === 'FAILED') return 'FAILED'
  if (calc === 'PENDING') return 'PENDING'
  if (calc === 'CALCULATED') return 'AVAILABLE'
  return 'NOT_AVAILABLE'
}

export function formatV3Score(score: number | null | undefined): string {
  if (score === null || score === undefined) return '—'
  return String(score)
}
