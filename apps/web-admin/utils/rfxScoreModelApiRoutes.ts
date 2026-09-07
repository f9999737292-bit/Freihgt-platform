/** RFx v3.0D score-model routes — parity anchor vs OpenAPI. */

import type { ApiRouteSpec } from '~/utils/rfxQuestionnaireApiRoutes'

const EVENT = '{id}'

export const RFX_SCORE_MODEL_API_ROUTES = {
  getScoreModel: {
    method: 'GET',
    path: `/api/v1/rfx-events/${EVENT}/score-model`,
    caller: 'loadScoreModel',
  },
  putScoreModel: {
    method: 'PUT',
    path: `/api/v1/rfx-events/${EVENT}/score-model`,
    caller: 'saveDraft',
  },
  validateScoreModel: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/score-model/validate`,
    caller: 'validateReadiness',
  },
  publishScoreModel: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/score-model/publish`,
    caller: 'publish',
  },
} as const satisfies Record<string, ApiRouteSpec>

export const SCORE_MODEL_OPENAPI_PARITY: readonly ApiRouteSpec[] = Object.values(RFX_SCORE_MODEL_API_ROUTES)
