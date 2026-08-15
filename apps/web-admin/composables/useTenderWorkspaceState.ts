import type { InjectionKey, Ref } from 'vue'
import type {
  AllocationConfig,
  AllocationOutcome,
  AwardConversionResult,
  AwardProposalStatus,
  QualificationRules,
  QuotaBalancePolicy,
  QuotaPosition,
  QuotaTarget,
  RunEvaluationResponse,
  ScoringFactorWeight,
} from '~/types/tender'
import { DEFAULT_SCORING_TEMPLATE } from '~/types/tender'

export interface TenderWorkspaceState {
  templateVersionId: string | null
  scoringFactors: ScoringFactorWeight[]
  qualificationRules: QualificationRules
  requiredVolume: number
  evaluationId: string | null
  evaluation: RunEvaluationResponse | null
  scenarioId: string | null
  allocationOutcome: AllocationOutcome | null
  quotaPositions: QuotaPosition[]
  quotaTargets: QuotaTarget[]
  quotaPolicy: QuotaBalancePolicy
  actualShares: Record<string, number>
  allocationConfig: AllocationConfig
  proposalId: string | null
  proposalStatus: AwardProposalStatus | null
  awardId: string | null
  conversion: AwardConversionResult | null
}

export function createDefaultTenderWorkspaceState(): TenderWorkspaceState {
  return {
    templateVersionId: null,
    scoringFactors: DEFAULT_SCORING_TEMPLATE.map((f) => ({ ...f })),
    qualificationRules: {
      minimum_sla_score: 75,
      minimum_capacity: 100,
      require_carrier_active: true,
    },
    requiredVolume: 500,
    evaluationId: null,
    evaluation: null,
    scenarioId: null,
    allocationOutcome: null,
    quotaPositions: [],
    quotaTargets: [],
    quotaPolicy: {
      tolerance_pct: 2,
      carry_balance: true,
      max_correction_pct: 5,
      period_type: 'MONTHLY',
    },
    actualShares: {},
    allocationConfig: {
      strategy: 'SCORE_WEIGHTED',
      constraints: {
        min_suppliers: 1,
        max_suppliers: 4,
        total_volume: 500,
        max_carrier_share_pct: 70,
      },
    },
    proposalId: null,
    proposalStatus: null,
    awardId: null,
    conversion: null,
  }
}

export const tenderWorkspaceKey: InjectionKey<Ref<TenderWorkspaceState>> = Symbol('tenderWorkspace')

export function provideTenderWorkspace(state: Ref<TenderWorkspaceState>) {
  provide(tenderWorkspaceKey, state)
}

export function useTenderWorkspaceState(): Ref<TenderWorkspaceState> {
  const state = inject(tenderWorkspaceKey)
  if (!state) {
    throw new Error('useTenderWorkspaceState must be used within RfxTenderWorkspace')
  }
  return state
}
