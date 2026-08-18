import { isLongFormRfxType } from '~/types/rfx'

export interface PublishReadinessInput {
  title: string
  lotCount: number
  responseDeadline?: string | null
  participantCount: number
  rfxType: string
  requiresDeadline?: boolean
}

export interface PublishReadinessResult {
  ready: boolean
  errors: string[]
  warnings: string[]
}

export function checkPublishReadiness(input: PublishReadinessInput): PublishReadinessResult {
  const errors: string[] = []
  const warnings: string[] = []

  if (!input.title?.trim()) {
    errors.push('title')
  }

  if (isLongFormRfxType(input.rfxType) && input.lotCount < 1) {
    errors.push('lots')
  } else if (input.lotCount < 1) {
    errors.push('lots')
  }

  const deadlineRequired = input.requiresDeadline !== false
  if (deadlineRequired && !input.responseDeadline?.trim()) {
    errors.push('deadline')
  }

  if (input.participantCount === 0) {
    warnings.push('participants')
  }

  return {
    ready: errors.length === 0,
    errors,
    warnings,
  }
}
