/** RFx v3.0E1–E5 version lifecycle routes — parity anchor vs OpenAPI. */

import type { ApiRouteSpec } from '~/utils/rfxQuestionnaireApiRoutes'

const EVENT = '{id}'

export const RFX_VERSION_API_ROUTES = {
  listVersions: {
    method: 'GET',
    path: `/api/v1/rfx-events/${EVENT}/versions`,
    caller: 'listVersions',
  },
  getVersionDetail: {
    method: 'GET',
    path: `/api/v1/rfx-events/${EVENT}/versions/{version_id}`,
    caller: 'getVersionDetail',
  },
  publishQuestionnaire: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/questionnaire/publish`,
    caller: 'publishQuestionnaire',
  },
  forkDraft: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/versions/fork-draft`,
    caller: 'forkDraft',
  },
  compareVersions: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/versions/compare`,
    caller: 'compareVersions',
  },
  restoreDraft: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/versions/{version_id}/restore-draft`,
    caller: 'restoreDraft',
  },
  previewChangeImpact: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/change-impact/preview`,
    caller: 'previewChangeImpact',
  },
  cloneFromTemplate: {
    method: 'POST',
    path: '/api/v1/rfx-events/from-template',
    caller: 'cloneFromTemplate',
  },
  validatePublish: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/validate-publish`,
    caller: 'validatePublish',
  },
} as const satisfies Record<string, ApiRouteSpec>

export const RFX_VERSION_OPENAPI_PARITY: readonly ApiRouteSpec[] = Object.values(RFX_VERSION_API_ROUTES)

export function rfxEventVersionApiPath(eventId: string, suffix: string): string {
  const normalized = suffix.startsWith('/') ? suffix : `/${suffix}`
  return `/api/v1/rfx-events/${eventId}${normalized}`
}
