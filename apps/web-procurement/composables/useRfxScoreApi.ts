import type {
  V3ResponseScoreView,
  V3ScoreExplanationResponse,
  V3ScoreLoadState,
} from '~/types/rfx-score'
import { ApiError } from '~/utils/apiClient'

export function useRfxScoreApi() {
  const { apiGet } = useApi()
  const scoreCache = reactive(new Map<string, { state: V3ScoreLoadState; score?: V3ResponseScoreView | null }>())

  function cacheKey(eventId: string, responseId: string) {
    return `${eventId}:${responseId}`
  }

  async function getResponseScore(eventId: string, responseId: string): Promise<V3ResponseScoreView | null> {
    const key = cacheKey(eventId, responseId)
    scoreCache.set(key, { state: 'LOADING' })
    try {
      const data = await apiGet<V3ResponseScoreView>(
        `/api/v1/rfx-events/${eventId}/responses/${responseId}/score`,
      )
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
    return apiGet<V3ScoreExplanationResponse>(
      `/api/v1/rfx-events/${eventId}/responses/${responseId}/score/explanation`,
    )
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
