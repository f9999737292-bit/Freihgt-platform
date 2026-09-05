/** RFx v3.0B studio routes consumed by web-admin — parity anchor vs OpenAPI. */

export type HttpMethod = 'GET' | 'POST' | 'PATCH' | 'DELETE'

export interface ApiRouteSpec {
  method: HttpMethod
  /** OpenAPI path template with `{id}` / `{section_id}` placeholders. */
  path: string
  /** Composable function that must call this route. */
  caller: string
}

const EVENT = '{id}'

export const RFX_QUESTIONNAIRE_API_ROUTES = {
  getStudio: {
    method: 'GET',
    path: `/api/v1/rfx-events/${EVENT}/studio`,
    caller: 'getStudio',
  },
  getQuestionnaire: {
    method: 'GET',
    path: `/api/v1/rfx-events/${EVENT}/questionnaire`,
    caller: 'getQuestionnaire',
  },
  saveDraft: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/save-draft`,
    caller: 'saveDraft',
  },
  validatePublish: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/validate-publish`,
    caller: 'validatePublish',
  },
  createSection: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/sections`,
    caller: 'createSection',
  },
  updateSection: {
    method: 'PATCH',
    path: `/api/v1/rfx-events/${EVENT}/sections/{section_id}`,
    caller: 'updateSection',
  },
  deleteSection: {
    method: 'DELETE',
    path: `/api/v1/rfx-events/${EVENT}/sections/{section_id}`,
    caller: 'deleteSection',
  },
  reorderSections: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/sections/reorder`,
    caller: 'reorderSections',
  },
  createQuestion: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/questions`,
    caller: 'createQuestion',
  },
  updateQuestion: {
    method: 'PATCH',
    path: `/api/v1/rfx-events/${EVENT}/questions/{question_id}`,
    caller: 'updateQuestion',
  },
  deleteQuestion: {
    method: 'DELETE',
    path: `/api/v1/rfx-events/${EVENT}/questions/{question_id}`,
    caller: 'deleteQuestion',
  },
  duplicateQuestion: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/questions/{question_id}/duplicate`,
    caller: 'duplicateQuestion',
  },
  reorderQuestions: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/questions/reorder`,
    caller: 'reorderQuestions',
  },
  createOption: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/questions/{question_id}/options`,
    caller: 'createOption',
  },
  updateOption: {
    method: 'PATCH',
    path: `/api/v1/rfx-events/${EVENT}/questions/{question_id}/options/{option_id}`,
    caller: 'updateOption',
  },
  deleteOption: {
    method: 'DELETE',
    path: `/api/v1/rfx-events/${EVENT}/questions/{question_id}/options/{option_id}`,
    caller: 'deleteOption',
  },
  createRule: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/rules`,
    caller: 'createRule',
  },
  updateRule: {
    method: 'PATCH',
    path: `/api/v1/rfx-events/${EVENT}/rules/{rule_id}`,
    caller: 'updateRule',
  },
  deleteRule: {
    method: 'DELETE',
    path: `/api/v1/rfx-events/${EVENT}/rules/{rule_id}`,
    caller: 'deleteRule',
  },
} as const satisfies Record<string, ApiRouteSpec>

/** Matrix exported for vitest OpenAPI parity checks. */
export const FRONTEND_OPENAPI_PARITY: readonly ApiRouteSpec[] = Object.values(RFX_QUESTIONNAIRE_API_ROUTES)

/** v3.0B — option reorder is intentionally absent from OpenAPI and frontend. */
export const FRONTEND_OPENAPI_EXCLUDED_V3_0B = [
  `/api/v1/rfx-events/${EVENT}/questions/{question_id}/options/reorder`,
] as const

export function rfxEventApiPath(eventId: string, suffix: string): string {
  const normalized = suffix.startsWith('/') ? suffix : `/${suffix}`
  return `/api/v1/rfx-events/${eventId}${normalized}`
}
