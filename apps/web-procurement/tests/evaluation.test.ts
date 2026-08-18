import { describe, expect, it } from 'vitest'
import { formatMoney, sortEvaluationItems } from '~/types/evaluation'
import { canEditCommercial } from '~/types/carrierRfx'

describe('evaluation helpers', () => {
  it('formats money with currency', () => {
    expect(formatMoney(1000.5, 'RUB')).toContain('1')
    expect(formatMoney(1000.5, 'RUB')).toContain('RUB')
  })

  it('sorts by rank then amount', () => {
    const sorted = sortEvaluationItems([
      { id: 'b', rfx_event_id: 'e1', participant_company_id: 'b', status: 'SUBMITTED', rank: 2, total_amount: 100 },
      { id: 'a', rfx_event_id: 'e1', participant_company_id: 'a', status: 'SUBMITTED', rank: 1, total_amount: 200 },
    ])
    expect(sorted[0].id).toBe('a')
  })

  it('allows commercial edit only in draft', () => {
    expect(canEditCommercial('DRAFT')).toBe(true)
    expect(canEditCommercial('SUBMITTED')).toBe(false)
  })
})
