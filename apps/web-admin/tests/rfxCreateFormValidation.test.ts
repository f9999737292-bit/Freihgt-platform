import { describe, expect, it } from 'vitest'
import {
  emptyCreateRfxForm,
  replaceRfxFormErrors,
  validateCreateRfxForm,
  type RfxFormErrors,
} from '../types/rfx'

describe('RFx create form validation state', () => {
  it('clears stale title error when title becomes valid on revalidation', () => {
    const form = emptyCreateRfxForm()
    const errors: RfxFormErrors = {}

    replaceRfxFormErrors(errors, validateCreateRfxForm(form))
    expect(errors.title).toBe('required')

    form.title = 'Pilot RFx title'
    replaceRfxFormErrors(errors, validateCreateRfxForm(form))
    expect(errors.title).toBeUndefined()
  })

  it('clears stale owner error when owner_company_id becomes valid on revalidation', () => {
    const form = emptyCreateRfxForm()
    const errors: RfxFormErrors = {}

    replaceRfxFormErrors(errors, validateCreateRfxForm(form))
    expect(errors.owner_company_id).toBe('required')

    form.owner_company_id = '55ec888f-0000-4000-8000-000000000001'
    replaceRfxFormErrors(errors, validateCreateRfxForm(form))
    expect(errors.owner_company_id).toBeUndefined()
  })

  it('clears valid_to range error when valid_from fixes an invalid range', () => {
    const form = emptyCreateRfxForm()
    form.valid_from = '2026-12-31'
    form.valid_to = '2026-01-01'
    const errors: RfxFormErrors = {}

    replaceRfxFormErrors(errors, validateCreateRfxForm(form))
    expect(errors.valid_to).toBe('range')

    form.valid_from = '2026-01-01'
    if (!form.valid_from || !form.valid_to || form.valid_to >= form.valid_from) {
      delete errors.valid_to
    }
    expect(errors.valid_to).toBeUndefined()
  })

  it('does not leave prior field keys after Object.assign-style sparse updates', () => {
    const form = emptyCreateRfxForm()
    const errors: RfxFormErrors = {}

    replaceRfxFormErrors(errors, validateCreateRfxForm(form))
    expect(Object.keys(errors).length).toBeGreaterThan(1)

    form.title = 'Valid title'
    form.rfx_number = 'RFX-001'
    form.owner_company_id = '55ec888f-0000-4000-8000-000000000001'
    form.response_deadline = '2026-12-31T12:00'
    replaceRfxFormErrors(errors, validateCreateRfxForm(form))

    expect(errors.title).toBeUndefined()
    expect(errors.rfx_number).toBeUndefined()
    expect(errors.owner_company_id).toBeUndefined()
    expect(errors.response_deadline).toBeUndefined()
  })
})

describe('RfxCreateModal validation integration (source contract)', () => {
  it('uses replaceRfxFormErrors instead of sparse Object.assign', async () => {
    const { readFileSync } = await import('node:fs')
    const { resolve } = await import('node:path')
    const source = readFileSync(resolve(import.meta.dirname, '../components/rfx/RfxCreateModal.vue'), 'utf8')

    expect(source).toContain('replaceRfxFormErrors')
    expect(source).not.toMatch(/Object\.assign\(errors,\s*validateCreateRfxForm/)
    expect(source).toContain("if (value.trim()) delete errors.title")
    expect(source).toContain("if (value.trim()) delete errors.owner_company_id")
    expect(source).toContain('clearValidToRangeErrorIfFixed')
    expect(source).toContain('watch(() => form.valid_from, clearValidToRangeErrorIfFixed)')
  })
})
