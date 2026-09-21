import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '~/utils/apiClient'
import { CarrierValidationError } from '~/utils/carrierResponseErrors'
import {
  isCarrierQuestionnaireBindingRequired,
  resolveCarrierQuestionnaireWorkspace,
  shouldStartCarrierResponseOnLoadError,
} from '~/utils/carrierQuestionnaireBinding'

type BindingWorkspace = {
  id: string
  product_status: string
  rfx_version_id?: string
}

function apiError(status: number, code: string, details: Record<string, unknown> = {}) {
  return new ApiError(status, { code, message: 'ignored', details })
}

describe('carrier questionnaire binding', () => {
  it('treats structured 422 rfx_version_id as binding-required', () => {
    expect(isCarrierQuestionnaireBindingRequired(apiError(422, 'UNPROCESSABLE_ENTITY', { field: 'rfx_version_id' }))).toBe(true)
    expect(isCarrierQuestionnaireBindingRequired(apiError(422, 'VALIDATION_ERROR', { field: 'rfx_version_id' }))).toBe(true)
  })

  it('does not classify other 422s or message text as binding-required', () => {
    expect(isCarrierQuestionnaireBindingRequired(apiError(422, 'UNPROCESSABLE_ENTITY', { field: 'questionnaire_enabled' }))).toBe(false)
    expect(isCarrierQuestionnaireBindingRequired(apiError(400, 'VALIDATION_ERROR', { field: 'rfx_version_id' }))).toBe(false)
    expect(isCarrierQuestionnaireBindingRequired(new CarrierValidationError([]))).toBe(false)
    expect(isCarrierQuestionnaireBindingRequired(new Error('carrier response is not bound to a published questionnaire version'))).toBe(false)
  })

  it('starts on GET 404 when startIfMissing and deadline is open', () => {
    expect(shouldStartCarrierResponseOnLoadError(apiError(404, 'NOT_FOUND'), { startIfMissing: true, deadlineExpired: false })).toBe(true)
    expect(shouldStartCarrierResponseOnLoadError(apiError(404, 'NOT_FOUND'), { startIfMissing: false, deadlineExpired: false })).toBe(false)
  })

  it('starts on binding-required 422 when startIfMissing and deadline is open', () => {
    const error = apiError(422, 'UNPROCESSABLE_ENTITY', { field: 'rfx_version_id' })
    expect(shouldStartCarrierResponseOnLoadError(error, { startIfMissing: true, deadlineExpired: false })).toBe(true)
    expect(shouldStartCarrierResponseOnLoadError(error, { startIfMissing: true, deadlineExpired: true })).toBe(false)
    expect(shouldStartCarrierResponseOnLoadError(error, { startIfMissing: false, deadlineExpired: false })).toBe(false)
  })

  it('does not start on another 422', () => {
    const error = apiError(422, 'UNPROCESSABLE_ENTITY', { field: 'questionnaire_enabled' })
    expect(shouldStartCarrierResponseOnLoadError(error, { startIfMissing: true, deadlineExpired: false })).toBe(false)
  })

  it('GET 404 then start applies the started workspace and keeps the response id', async () => {
    const started: BindingWorkspace = { id: 'resp-1', product_status: 'DRAFT' }
    const result = await resolveCarrierQuestionnaireWorkspace<BindingWorkspace>({
      get: vi.fn().mockRejectedValue(apiError(404, 'NOT_FOUND')),
      start: vi.fn().mockResolvedValue(started),
      startIfMissing: true,
      deadlineExpired: false,
      isNotStarted: (ws) => ws.product_status === 'NOT_STARTED',
    })
    expect(result.started).toBe(true)
    expect(result.workspace.id).toBe('resp-1')
  })

  it('GET binding-required 422 then start keeps the commercial response id', async () => {
    const start = vi.fn().mockResolvedValue({ id: 'commercial-1', product_status: 'DRAFT', rfx_version_id: 'ver-1' })
    const result = await resolveCarrierQuestionnaireWorkspace<BindingWorkspace>({
      get: vi.fn().mockRejectedValue(apiError(422, 'UNPROCESSABLE_ENTITY', { field: 'rfx_version_id' })),
      start,
      startIfMissing: true,
      deadlineExpired: false,
      isNotStarted: (ws) => ws.product_status === 'NOT_STARTED',
    })
    expect(start).toHaveBeenCalledTimes(1)
    expect(result.workspace.id).toBe('commercial-1')
    expect(result.workspace.rfx_version_id).toBe('ver-1')
  })

  it('other 422 is rethrown without start', async () => {
    const start = vi.fn()
    await expect(resolveCarrierQuestionnaireWorkspace({
      get: vi.fn().mockRejectedValue(apiError(422, 'UNPROCESSABLE_ENTITY', { field: 'questionnaire_enabled' })),
      start,
      startIfMissing: true,
      deadlineExpired: false,
      isNotStarted: () => false,
    })).rejects.toMatchObject({ status: 422, details: { field: 'questionnaire_enabled' } })
    expect(start).not.toHaveBeenCalled()
  })

  it('binding-required after deadline does not start', async () => {
    const start = vi.fn()
    await expect(resolveCarrierQuestionnaireWorkspace({
      get: vi.fn().mockRejectedValue(apiError(422, 'UNPROCESSABLE_ENTITY', { field: 'rfx_version_id' })),
      start,
      startIfMissing: true,
      deadlineExpired: true,
      isNotStarted: () => false,
    })).rejects.toMatchObject({ details: { field: 'rfx_version_id' } })
    expect(start).not.toHaveBeenCalled()
  })

  it('loadWorkspace uses the structured binding helper', () => {
    const source = readFileSync(resolve(process.cwd(), 'composables/useCarrierResponseWorkspace.ts'), 'utf8')
    expect(source).toContain('resolveCarrierQuestionnaireWorkspace')
    expect(source).toContain('isNotStarted: (workspace) => workspace.product_status === \'NOT_STARTED\'')
    expect(source).not.toMatch(/err\.status === 404 && options\.startIfMissing/)
  })

  it('reload after pin uses only GET', async () => {
    const get = vi.fn().mockResolvedValue({ id: 'commercial-1', product_status: 'DRAFT', rfx_version_id: 'ver-1' })
    const start = vi.fn()
    const result = await resolveCarrierQuestionnaireWorkspace<BindingWorkspace>({
      get,
      start,
      startIfMissing: true,
      deadlineExpired: false,
      isNotStarted: (ws) => ws.product_status === 'NOT_STARTED',
    })
    expect(result.started).toBe(false)
    expect(result.workspace.id).toBe('commercial-1')
    expect(start).not.toHaveBeenCalled()
  })
})
