/** RFx v3.0E4 template library routes — parity anchor vs OpenAPI. */

import type { ApiRouteSpec } from '~/utils/rfxQuestionnaireApiRoutes'

const TEMPLATE = '{id}'

export const RFX_TEMPLATE_API_ROUTES = {
  listTemplates: {
    method: 'GET',
    path: '/api/v1/rfx-templates',
    caller: 'listTemplates',
  },
  createTemplate: {
    method: 'POST',
    path: '/api/v1/rfx-templates',
    caller: 'createTemplate',
  },
  getTemplate: {
    method: 'GET',
    path: `/api/v1/rfx-templates/${TEMPLATE}`,
    caller: 'getTemplate',
  },
  updateTemplate: {
    method: 'PATCH',
    path: `/api/v1/rfx-templates/${TEMPLATE}`,
    caller: 'updateTemplate',
  },
  archiveTemplate: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/archive`,
    caller: 'archiveTemplate',
  },
  publishTemplateVersion: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/versions/publish`,
    caller: 'publishTemplateVersion',
  },
  forkTemplateDraft: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/versions/fork-draft`,
    caller: 'forkTemplateDraft',
  },
  getTemplateQuestionnaire: {
    method: 'GET',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questionnaire`,
    caller: 'getTemplateQuestionnaire',
  },
  createSection: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/sections`,
    caller: 'createSection',
  },
  updateSection: {
    method: 'PATCH',
    path: `/api/v1/rfx-templates/${TEMPLATE}/sections/{section_id}`,
    caller: 'updateSection',
  },
  deleteSection: {
    method: 'DELETE',
    path: `/api/v1/rfx-templates/${TEMPLATE}/sections/{section_id}`,
    caller: 'deleteSection',
  },
  reorderSections: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/sections/reorder`,
    caller: 'reorderSections',
  },
  createQuestion: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questions`,
    caller: 'createQuestion',
  },
  updateQuestion: {
    method: 'PATCH',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questions/{question_id}`,
    caller: 'updateQuestion',
  },
  deleteQuestion: {
    method: 'DELETE',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questions/{question_id}`,
    caller: 'deleteQuestion',
  },
  duplicateQuestion: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questions/{question_id}/duplicate`,
    caller: 'duplicateQuestion',
  },
  reorderQuestions: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questions/reorder`,
    caller: 'reorderQuestions',
  },
  createOption: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questions/{question_id}/options`,
    caller: 'createOption',
  },
  updateOption: {
    method: 'PATCH',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questions/{question_id}/options/{option_id}`,
    caller: 'updateOption',
  },
  deleteOption: {
    method: 'DELETE',
    path: `/api/v1/rfx-templates/${TEMPLATE}/questions/{question_id}/options/{option_id}`,
    caller: 'deleteOption',
  },
  createRule: {
    method: 'POST',
    path: `/api/v1/rfx-templates/${TEMPLATE}/rules`,
    caller: 'createRule',
  },
  updateRule: {
    method: 'PATCH',
    path: `/api/v1/rfx-templates/${TEMPLATE}/rules/{rule_id}`,
    caller: 'updateRule',
  },
  deleteRule: {
    method: 'DELETE',
    path: `/api/v1/rfx-templates/${TEMPLATE}/rules/{rule_id}`,
    caller: 'deleteRule',
  },
} as const satisfies Record<string, ApiRouteSpec>

export const RFX_TEMPLATE_OPENAPI_PARITY: readonly ApiRouteSpec[] = Object.values(RFX_TEMPLATE_API_ROUTES)

export function rfxTemplateApiPath(templateId: string, suffix: string): string {
  const normalized = suffix.startsWith('/') ? suffix : `/${suffix}`
  return `/api/v1/rfx-templates/${templateId}${normalized}`
}
