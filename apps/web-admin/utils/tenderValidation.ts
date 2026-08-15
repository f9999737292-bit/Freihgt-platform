import {
  TENDER_SCORING_FACTORS,
  type ScoringFactorWeight,
  type TenderScoringFactor,
} from '~/types/tender'

const WEIGHT_SUM_TARGET = 100
const WEIGHT_SUM_TOLERANCE = 0.01

export interface ScoringValidationResult {
  valid: boolean
  errors: string[]
  totalWeight: number
}

export function validateScoringWeights(factors: ScoringFactorWeight[]): ScoringValidationResult {
  const errors: string[] = []
  const seen = new Set<string>()
  let totalWeight = 0

  if (factors.length === 0) {
    errors.push('at_least_one_factor')
  }

  for (const entry of factors) {
    const factor = String(entry.factor).trim().toUpperCase()
    if (!TENDER_SCORING_FACTORS.includes(factor as TenderScoringFactor)) {
      errors.push(`unsupported_factor:${factor}`)
    }
    if (seen.has(factor)) {
      errors.push(`duplicate_factor:${factor}`)
    }
    seen.add(factor)
    if (Number.isNaN(entry.weight) || entry.weight < 0) {
      errors.push(`negative_weight:${factor}`)
    }
    totalWeight += entry.weight
  }

  if (Math.abs(totalWeight - WEIGHT_SUM_TARGET) > WEIGHT_SUM_TOLERANCE) {
    errors.push('weight_sum_not_100')
  }

  return {
    valid: errors.length === 0,
    errors,
    totalWeight,
  }
}
