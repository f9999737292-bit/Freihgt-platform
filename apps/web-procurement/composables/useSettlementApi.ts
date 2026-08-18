import type {
  BillingRegisterListResponse,
  CreateSettlementRequest,
  FreightSettlement,
  FreightSettlementDetail,
  IncludeRegisterRequest,
  ProposeAccessorialRequest,
  RaiseDisputeRequest,
  ResolveDisputeRequest,
  SettlementActor,
  SettlementListResponse,
} from '~/types/settlement'
import { settlementActorQuery } from '~/utils/settlement'

function requireCompanyId(companyId: string | null | undefined): string {
  const value = companyId?.trim()
  if (!value) {
    throw new Error('Company ID is required')
  }
  return value
}

export function useSettlementApi() {
  const { apiGet, apiPost } = useApi()
  const tenantStore = useTenantStore()

  function actorQuery(actor: SettlementActor) {
    return settlementActorQuery(actor, requireCompanyId(tenantStore.currentCompanyId))
  }

  async function listSettlements(
    actor: SettlementActor,
    options: { status?: string; limit?: number; offset?: number } = {},
  ) {
    const companyId = requireCompanyId(tenantStore.currentCompanyId)
    const query: Record<string, string | number | undefined> = {
      limit: options.limit ?? 50,
      offset: options.offset ?? 0,
    }
    if (options.status) query.status = options.status
    if (actor === 'BUYER') {
      query.buyer_company_id = companyId
    } else {
      query.carrier_company_id = companyId
    }
    const data = await apiGet<SettlementListResponse>('/api/v1/freight-settlements', { query })
    return {
      items: data.items ?? [],
      total: data.total ?? data.items?.length ?? 0,
    }
  }

  async function getSettlement(id: string, actor: SettlementActor) {
    return apiGet<FreightSettlementDetail>(`/api/v1/freight-settlements/${encodeURIComponent(id)}`, {
      query: actorQuery(actor),
    })
  }

  async function createSettlement(actor: SettlementActor, body: CreateSettlementRequest) {
    return apiPost<FreightSettlement>('/api/v1/freight-settlements', body, {
      query: actorQuery(actor),
    })
  }

  async function proposeAccessorial(
    settlementId: string,
    actor: SettlementActor,
    body: ProposeAccessorialRequest,
  ) {
    return apiPost(`/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/accessorials`, body, {
      query: actorQuery(actor),
    })
  }

  async function approveAccessorial(settlementId: string, accessorialId: string, actor: SettlementActor) {
    return apiPost<FreightSettlementDetail>(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/accessorials/${encodeURIComponent(accessorialId)}/approve`,
      {},
      { query: actorQuery(actor) },
    )
  }

  async function rejectAccessorial(settlementId: string, accessorialId: string, actor: SettlementActor) {
    return apiPost<FreightSettlementDetail>(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/accessorials/${encodeURIComponent(accessorialId)}/reject`,
      {},
      { query: actorQuery(actor) },
    )
  }

  async function raiseDispute(settlementId: string, actor: SettlementActor, body: RaiseDisputeRequest) {
    return apiPost(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/disputes`,
      body,
      { query: actorQuery(actor) },
    )
  }

  async function resolveDispute(
    settlementId: string,
    disputeId: string,
    actor: SettlementActor,
    body: ResolveDisputeRequest,
  ) {
    return apiPost<FreightSettlementDetail>(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/disputes/${encodeURIComponent(disputeId)}/resolve`,
      body,
      { query: actorQuery(actor) },
    )
  }

  async function submitForReview(settlementId: string, actor: SettlementActor) {
    return apiPost<FreightSettlement>(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/submit-for-review`,
      {},
      { query: actorQuery(actor) },
    )
  }

  async function approveSettlement(settlementId: string, actor: SettlementActor) {
    return apiPost<FreightSettlement>(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/approve`,
      {},
      { query: actorQuery(actor) },
    )
  }

  async function markDocumentsReady(settlementId: string, actor: SettlementActor) {
    return apiPost<FreightSettlement>(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/mark-documents-ready`,
      {},
      { query: actorQuery(actor) },
    )
  }

  async function markReadyForPayment(settlementId: string, actor: SettlementActor) {
    return apiPost<FreightSettlement>(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/mark-ready-for-payment`,
      {},
      { query: actorQuery(actor) },
    )
  }

  async function includeInRegister(
    settlementId: string,
    actor: SettlementActor,
    body: IncludeRegisterRequest,
  ) {
    return apiPost<FreightSettlement>(
      `/api/v1/freight-settlements/${encodeURIComponent(settlementId)}/include-in-register`,
      body,
      { query: actorQuery(actor) },
    )
  }

  async function listBillingRegisters(options: {
    customerCompanyId?: string
    contractorCompanyId?: string
    limit?: number
    offset?: number
  } = {}) {
    const query: Record<string, string | number | undefined> = {
      tenant_id: tenantStore.tenantId,
      limit: options.limit ?? 50,
      offset: options.offset ?? 0,
    }
    if (options.customerCompanyId) query.customer_company_id = options.customerCompanyId
    if (options.contractorCompanyId) query.contractor_company_id = options.contractorCompanyId
    const data = await apiGet<BillingRegisterListResponse>('/api/v1/billing-registers', { query })
    return {
      items: data.items ?? [],
      total: data.total ?? data.items?.length ?? 0,
    }
  }

  return {
    listSettlements,
    getSettlement,
    createSettlement,
    proposeAccessorial,
    approveAccessorial,
    rejectAccessorial,
    raiseDispute,
    resolveDispute,
    submitForReview,
    approveSettlement,
    markDocumentsReady,
    markReadyForPayment,
    includeInRegister,
    listBillingRegisters,
  }
}
