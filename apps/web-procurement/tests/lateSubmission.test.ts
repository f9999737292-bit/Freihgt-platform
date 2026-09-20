import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { ApiError } from '~/utils/apiClient'
import {
  canCreateLateSubmissionRequest,
  canDecideLateSubmission,
  canEnableQuestionnaireSubmitButton,
  canLateSubmitQuestionnaire,
  canShowBuyerLateQueue,
  canShowCarrierLateRequestPanel,
  hasLateSubmissionBuyerManageRole,
  hasLateSubmissionBuyerReadRole,
  hasLateSubmissionCarrierRole,
  isApprovedWindowActive,
  lateSubmissionHumanJwtOperations,
  lateSubmitBlockReason,
} from '~/utils/lateSubmissionAccess'
import { classifyLateSubmissionHttpError } from '~/utils/lateSubmissionErrors'
import { isRfxLateSubmissionEnabled } from '~/utils/lateSubmissionFeatureFlag'
import {
  createLateSubmissionIdempotencyStore,
  lateCreateAttemptPhase,
  LATE_SUBMISSION_IDEMPOTENCY_KEY_MAX_LENGTH,
} from '~/utils/lateSubmissionIdempotency'
import type { LateSubmissionRequest } from '~/types/lateSubmission'

const eventId = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa'
const requestId = 'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb'

function request(overrides: Partial<LateSubmissionRequest> = {}): LateSubmissionRequest {
  return {
    id: requestId,
    rfx_event_id: eventId,
    carrier_company_id: 'cccccccc-cccc-cccc-cccc-cccccccccccc',
    reason_code: 'TECHNICAL_FAILURE',
    reason_text: 'outage',
    requested_until: '2026-09-21T12:00:00Z',
    status: 'REQUESTED',
    version: 1,
    ...overrides,
  }
}

function flatten(value: Record<string, unknown>, prefix = ''): Record<string, unknown> {
  return Object.entries(value).reduce<Record<string, unknown>>((acc, [key, nested]) => {
    const next = prefix ? `${prefix}.${key}` : key
    if (nested && typeof nested === 'object' && !Array.isArray(nested)) {
      Object.assign(acc, flatten(nested as Record<string, unknown>, next))
    } else {
      acc[next] = nested
    }
    return acc
  }, {})
}

function i18nKeys(locale: string): string[] {
  const raw = JSON.parse(
    readFileSync(resolve(__dirname, `../i18n/${locale}/lateSubmission.json`), 'utf8'),
  ) as { lateSubmission: Record<string, unknown> }
  return Object.keys(flatten(raw.lateSubmission))
}

describe('late submission F3 routes and access', () => {
  it('uses the accepted human JWT paths and never opens commercial submit', () => {
    const ops = lateSubmissionHumanJwtOperations(eventId, requestId)
    expect(ops.create).toBe(`/api/v1/rfx-events/${eventId}/late-submission-requests`)
    expect(ops.mine).toBe(`/api/v1/rfx-events/${eventId}/late-submission-requests/mine`)
    expect(ops.buyerQueue).toBe(`/api/v1/rfx-events/${eventId}/late-submission-requests`)
    expect(ops.approve).toBe(`/api/v1/rfx-events/${eventId}/late-submission-requests/${requestId}/approve`)
    expect(ops.reject).toBe(`/api/v1/rfx-events/${eventId}/late-submission-requests/${requestId}/reject`)
    expect(ops.questionnaireSubmit).toBe(`/api/v1/rfx-events/${eventId}/carrier-response/submit`)
    expect(Object.values(ops).every((path) => !path.includes('/rfx-responses/'))).toBe(true)
  })

  it('keeps carrier and buyer RBAC separate from SHIPPER_LOGIST manage', () => {
    expect(hasLateSubmissionCarrierRole(['CARRIER_DISPATCHER'])).toBe(true)
    expect(hasLateSubmissionCarrierRole(['SHIPPER_LOGIST'])).toBe(false)
    expect(hasLateSubmissionBuyerReadRole(['SHIPPER_LOGIST'])).toBe(true)
    expect(hasLateSubmissionBuyerManageRole(['SHIPPER_LOGIST'])).toBe(false)
    expect(hasLateSubmissionBuyerManageRole(['PROCUREMENT_MANAGER'])).toBe(true)
    expect(canShowBuyerLateQueue({ lateSubmissionEnabled: true, roles: ['SHIPPER_LOGIST'] })).toBe(true)
    expect(canDecideLateSubmission({
      lateSubmissionEnabled: true,
      roles: ['SHIPPER_LOGIST'],
      request: request(),
    })).toBe(false)
    expect(canDecideLateSubmission({
      lateSubmissionEnabled: true,
      roles: ['PROCUREMENT_MANAGER'],
      request: request(),
    })).toBe(true)
  })

  it('hides the carrier panel before deadline or when the flag is off', () => {
    expect(isRfxLateSubmissionEnabled('true')).toBe(true)
    expect(isRfxLateSubmissionEnabled('1')).toBe(true)
    expect(isRfxLateSubmissionEnabled(false)).toBe(false)
    expect(canShowCarrierLateRequestPanel({
      lateSubmissionEnabled: false,
      roles: ['CARRIER_DISPATCHER'],
      deadline: '2020-01-01T00:00:00Z',
    })).toBe(false)
    expect(canShowCarrierLateRequestPanel({
      lateSubmissionEnabled: true,
      roles: ['CARRIER_DISPATCHER'],
      deadline: '2099-01-01T00:00:00Z',
    })).toBe(false)
    expect(canShowCarrierLateRequestPanel({
      lateSubmissionEnabled: true,
      roles: ['CARRIER_DISPATCHER'],
      deadline: '2020-01-01T00:00:00Z',
    })).toBe(true)
  })

  it('allows create only when no REQUESTED or APPROVED request exists', () => {
    const base = {
      lateSubmissionEnabled: true,
      roles: ['CARRIER_ADMIN'],
      deadline: '2020-01-01T00:00:00Z',
    }
    expect(canCreateLateSubmissionRequest({ ...base, request: null })).toBe(true)
    expect(canCreateLateSubmissionRequest({ ...base, request: request({ status: 'REQUESTED' }) })).toBe(false)
    expect(canCreateLateSubmissionRequest({ ...base, request: request({ status: 'APPROVED' }) })).toBe(false)
    expect(canCreateLateSubmissionRequest({ ...base, request: request({ status: 'REJECTED' }) })).toBe(true)
  })

  it('allows late questionnaire submit only for DRAFT inside an active APPROVED window', () => {
    const now = Date.parse('2026-09-20T12:00:00Z')
    const approved = request({
      status: 'APPROVED',
      approved_valid_from: '2026-09-20T11:00:00Z',
      approved_valid_until: '2026-09-20T13:00:00Z',
    })
    expect(isApprovedWindowActive(approved.approved_valid_from, approved.approved_valid_until, now)).toBe(true)
    expect(canLateSubmitQuestionnaire({
      lateSubmissionEnabled: true,
      roles: ['CARRIER_DISPATCHER'],
      deadline: '2026-09-19T00:00:00Z',
      responseStatus: 'DRAFT',
      request: approved,
      now,
    })).toBe(true)
    expect(canLateSubmitQuestionnaire({
      lateSubmissionEnabled: true,
      roles: ['CARRIER_DISPATCHER'],
      deadline: '2026-09-19T00:00:00Z',
      responseStatus: 'SUBMITTED',
      request: approved,
      now,
    })).toBe(false)
    expect(lateSubmitBlockReason({
      deadline: '2026-09-19T00:00:00Z',
      responseStatus: 'DRAFT',
      request: request({
        status: 'APPROVED',
        approved_valid_from: '2026-09-20T12:30:00Z',
        approved_valid_until: '2026-09-20T14:00:00Z',
      }),
      now,
    })).toBe('window_not_started')
    expect(lateSubmitBlockReason({
      deadline: '2026-09-19T00:00:00Z',
      responseStatus: 'DRAFT',
      request: request({
        status: 'APPROVED',
        approved_valid_from: '2026-09-20T10:00:00Z',
        approved_valid_until: '2026-09-20T12:00:00Z',
      }),
      now,
    })).toBe('window_expired')
    expect(lateSubmitBlockReason({
      deadline: '2026-09-19T00:00:00Z',
      responseStatus: 'DRAFT',
      request: request({
        status: 'EXPIRED',
        approved_valid_from: '2026-09-20T10:00:00Z',
        approved_valid_until: '2026-09-20T12:00:00Z',
      }),
      now,
    })).toBe('window_expired')
  })

  it('keeps pre-deadline submit clickable and only locks the button outside an approved late window', () => {
    expect(canEnableQuestionnaireSubmitButton({
      deadlineExpired: false,
      lateSubmitAllowed: false,
    })).toBe(true)
    expect(canEnableQuestionnaireSubmitButton({
      deadlineExpired: true,
      lateSubmitAllowed: true,
    })).toBe(true)
    expect(canEnableQuestionnaireSubmitButton({
      deadlineExpired: true,
      lateSubmitAllowed: false,
    })).toBe(false)
  })

  it('reuses one Idempotency-Key per logical create attempt and rotates after terminal status', () => {
    const store = createLateSubmissionIdempotencyStore()
    const body = {
      reason_code: 'TECHNICAL_FAILURE' as const,
      reason_text: 'vpn outage',
      requested_until: '2026-09-21T12:00:00Z',
    }
    const first = store.bindCreateAttempt(eventId, 'carrier-1', null, body)
    expect(store.bindCreateAttempt(eventId, 'carrier-1', null, {
      ...body,
      reason_text: 'changed after the first click',
    })).toEqual(first)
    expect(first.key).toBe(store.keyForCreate(eventId, 'carrier-1'))
    expect(first.key.startsWith('late-create:')).toBe(true)
    expect(first.key.length).toBeLessThanOrEqual(LATE_SUBMISSION_IDEMPOTENCY_KEY_MAX_LENGTH)
    expect(first.body.reason_text).toBe('vpn outage')

    for (const status of ['REJECTED', 'EXPIRED', 'CONSUMED'] as const) {
      const terminal = request({ id: `${status.toLowerCase()}-1`, status })
      expect(lateCreateAttemptPhase(terminal)).toBe(`after:${terminal.id}`)
      const next = store.bindCreateAttempt(eventId, 'carrier-1', terminal, {
        ...body,
        reason_text: `retry after ${status}`,
      })
      expect(next.key).not.toBe(first.key)
      expect(next.key.startsWith('late-create:')).toBe(true)
      expect(next.key.length).toBeLessThanOrEqual(LATE_SUBMISSION_IDEMPOTENCY_KEY_MAX_LENGTH)
      expect(store.bindCreateAttempt(eventId, 'carrier-1', terminal, {
        ...body,
        reason_text: 'must keep the first body of this attempt',
      })).toEqual(next)
    }

    expect(store.keyForApprove(requestId, 1).startsWith('late-approve:')).toBe(true)
    expect(store.keyForReject(requestId, 1).startsWith('late-reject:')).toBe(true)
    expect(store.keyForSubmit('response-1', 2).startsWith('late-submit:')).toBe(true)
    expect(store.keyForApprove(requestId, 1).length).toBeLessThanOrEqual(LATE_SUBMISSION_IDEMPOTENCY_KEY_MAX_LENGTH)
  })

  it('classifies window and permission HTTP errors', () => {
    expect(classifyLateSubmissionHttpError(new ApiError(422, {
      code: 'UNPROCESSABLE_ENTITY',
      message: 'late submission window has not started yet',
      details: { field: 'approved_valid_from' },
    }))).toBe('windowNotStarted')
    expect(classifyLateSubmissionHttpError(new ApiError(422, {
      code: 'UNPROCESSABLE_ENTITY',
      message: 'late submission window has expired',
      details: { field: 'approved_valid_until' },
    }))).toBe('windowExpired')
    expect(classifyLateSubmissionHttpError(new ApiError(400, {
      code: 'VALIDATION_ERROR',
      message: 'Idempotency-Key header is required',
      details: { field: 'Idempotency-Key' },
    }))).toBe('idempotencyRequired')
  })

  it('keeps RU/EN/ZH copy keys aligned', () => {
    const en = i18nKeys('en-US')
    expect(i18nKeys('ru-RU')).toEqual(en)
    expect(i18nKeys('zh-CN')).toEqual(en)
    expect(en).toContain('submit.blockedWindowNotStarted')
    expect(en).toContain('submit.blockedWindowExpired')
    expect(en).toContain('carrier.commercialLocked')
  })
})
