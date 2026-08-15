export const TENDER_SCORING_FACTORS = [
  'PRICE',
  'SLA',
  'CARRIER_KPI',
  'CAPACITY',
  'RELIABILITY',
  'TRANSIT_TIME',
] as const

export type TenderScoringFactor = (typeof TENDER_SCORING_FACTORS)[number]

export interface ScoringFactorWeight {
  factor: TenderScoringFactor | string
  weight: number
}

export const DEFAULT_SCORING_TEMPLATE: ScoringFactorWeight[] = [
  { factor: 'PRICE', weight: 35 },
  { factor: 'SLA', weight: 20 },
  { factor: 'CARRIER_KPI', weight: 15 },
  { factor: 'CAPACITY', weight: 10 },
  { factor: 'RELIABILITY', weight: 10 },
  { factor: 'TRANSIT_TIME', weight: 10 },
]
