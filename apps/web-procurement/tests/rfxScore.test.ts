import { describe, expect, it } from 'vitest'
import { formatV3Score, resolveScoreState } from '../composables/useRfxScoreApi'

describe('rfx score evaluation helpers', () => {
  it('does not convert missing score to zero', () => {
    expect(formatV3Score(null)).toBe('—')
    expect(formatV3Score(undefined)).toBe('—')
  })

  it('does not convert FAILED calculation to rejected qualification display state', () => {
    expect(
      resolveScoreState({
        qualification: { status: 'REJECTED', calculation_status: 'FAILED', total_score: null },
        answer_scores: [],
      }),
    ).toBe('FAILED')
  })

  it('maps CALCULATED to AVAILABLE', () => {
    expect(
      resolveScoreState({
        qualification: { status: 'QUALIFIED', calculation_status: 'CALCULATED', total_score: 70 },
        answer_scores: [],
      }),
    ).toBe('AVAILABLE')
  })
})

describe('evaluation page v3 integration contract', () => {
  it('keeps commercial score separate from v3 column', () => {
    const { readFileSync } = require('node:fs')
    const { resolve } = require('node:path')
    const source = readFileSync(resolve(__dirname, '../pages/tenders/[id]/evaluation.vue'), 'utf8')
    expect(source).toContain('legacy-commercial-score')
    expect(source).toContain('TendersV3ScoreCell')
    expect(source).toContain('commercialScore')
  })
})
