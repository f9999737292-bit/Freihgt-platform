export interface OfferLine {
  id?: string
  rfx_lot_id?: string | null
  amount: number
  currency_code: string
  comment?: string | null
}

export interface EvaluationResponseItem {
  id: string
  rfx_event_id: string
  participant_company_id: string
  status: string
  submitted_at?: string | null
  total_amount?: number
  currency_code?: string
  commercial_score?: number
  manual_score?: number
  total_score?: number
  rank?: number
  comparable?: boolean
  shortlisted?: boolean
  awarded?: boolean
  offer_complete?: boolean
  participant_status?: string
  offer_lines?: OfferLine[]
}

export interface AuditEventItem {
  id: string
  action: string
  entity_type: string
  entity_id: string
  actor_user_id?: string
  actor_company_id?: string
  metadata?: Record<string, unknown>
  created_at: string
}

export interface RfxAwardResult {
  id: string
  rfx_event_id: string
  rfx_response_id: string
  carrier_company_id: string
  total_amount?: number
  currency_code?: string
  awarded_at: string
}

export function formatMoney(amount?: number, currency?: string): string {
  if (amount == null || Number.isNaN(amount)) return '—'
  const formatted = new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(amount)
  return currency ? `${formatted} ${currency}` : formatted
}

export function sortEvaluationItems(items: EvaluationResponseItem[]): EvaluationResponseItem[] {
  return [...items].sort((a, b) => {
    const rankA = a.rank ?? 9999
    const rankB = b.rank ?? 9999
    if (rankA !== rankB) return rankA - rankB
    return (a.total_amount ?? 0) - (b.total_amount ?? 0)
  })
}
