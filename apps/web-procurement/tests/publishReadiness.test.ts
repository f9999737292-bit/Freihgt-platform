import { describe, expect, it } from 'vitest'
import { checkPublishReadiness } from '~/utils/publishReadiness'

describe('publishReadiness', () => {
  it('requires title, lot and deadline by default', () => {
    const result = checkPublishReadiness({
      title: '',
      lotCount: 0,
      responseDeadline: '',
      participantCount: 0,
      rfxType: 'LANE_TENDER',
    })

    expect(result.ready).toBe(false)
    expect(result.errors).toEqual(['title', 'lots', 'deadline'])
  })

  it('passes when required fields are present', () => {
    const result = checkPublishReadiness({
      title: 'Lane tender Q1',
      lotCount: 2,
      responseDeadline: '2026-09-01T18:00',
      participantCount: 3,
      rfxType: 'LANE_TENDER',
    })

    expect(result.ready).toBe(true)
    expect(result.errors).toEqual([])
  })

  it('warns when participants are missing', () => {
    const result = checkPublishReadiness({
      title: 'Spot RFQ',
      lotCount: 1,
      responseDeadline: '2026-09-01T18:00',
      participantCount: 0,
      rfxType: 'SPOT_RFQ',
    })

    expect(result.ready).toBe(true)
    expect(result.warnings).toContain('participants')
  })
})
