import { describe, expect, it } from 'vitest'
import {
  assertSavedOnlyAfterServerAck,
  autosaveStatusFromCarrierHttpError,
  canShowCarrierSaved,
  classifyCarrierAutosaveHttpError,
  isCarrierLeaveWarningState,
  markCarrierDirtyTransition,
  markCarrierSavedTransition,
} from '~/utils/carrierResponseAutosave'

describe('carrierResponseAutosave FSM', () => {
  it('classifies 422 as validation not conflict', () => {
    expect(classifyCarrierAutosaveHttpError(422)).toBe('validation')
    expect(classifyCarrierAutosaveHttpError(409)).toBe('conflict')
    expect(autosaveStatusFromCarrierHttpError(422)).toBe('invalid')
    expect(autosaveStatusFromCarrierHttpError(409)).toBe('conflict')
  })

  it('never shows saved while invalid or conflict', () => {
    expect(canShowCarrierSaved('invalid')).toBe(false)
    expect(canShowCarrierSaved('conflict')).toBe(false)
    expect(canShowCarrierSaved('save_failed')).toBe(false)
    expect(canShowCarrierSaved('saved')).toBe(true)
  })

  it('markDirty does not clobber saving state', () => {
    expect(markCarrierDirtyTransition('saving')).toBe('saving')
    expect(markCarrierDirtyTransition('saved')).toBe('dirty')
  })

  it('markSaved blocked when invalid', () => {
    const next = markCarrierSavedTransition('invalid', '2026-01-01T00:00:00Z')
    expect(next.status).toBe('invalid')
  })

  it('leave warning covers unsaved states', () => {
    expect(isCarrierLeaveWarningState('dirty')).toBe(true)
    expect(isCarrierLeaveWarningState('invalid')).toBe(true)
    expect(isCarrierLeaveWarningState('saved')).toBe(false)
  })

  it('saved only after server ack', () => {
    expect(assertSavedOnlyAfterServerAck('saved', true)).toBe(true)
    expect(assertSavedOnlyAfterServerAck('saved', false)).toBe(false)
    expect(assertSavedOnlyAfterServerAck('dirty', false)).toBe(true)
  })
})
