import type {
  BillingRegister,
  BillingRegisterDetail,
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

  function actorQuery(_actor: SettlementActor) {
    return settlementActorQuery(requireCompanyId(tenantStore.currentCompanyId))
  }

  async function listSettlements(
    actor: SettlementActor,
    options: { status?: string; limit?: number; offset?: number } = {},
  ) {
    const query: Record<string, string | number | undefined> = {
      ...actorQuery(actor),
      limit: options.limit ?? 50,
      offset: options.offset ?? 0,
    }
    if (options.status) query.status = options.status
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

  async function listBillingRegisters(actor: SettlementActor, options: { limit?: number; offset?: number } = {}) {
    const query: Record<string, string | number | undefined> = {
      ...actorQuery(actor),
      limit: options.limit ?? 50,
      offset: options.offset ?? 0,
    }
    const data = await apiGet<BillingRegisterListResponse>('/api/v1/billing-registers', { query })
    return {
      items: data.items ?? [],
      total: data.total ?? data.items?.length ?? 0,
    }
  }

  async function getBillingRegister(id: string, actor: SettlementActor) {
    return apiGet<BillingRegisterDetail>(`/api/v1/billing-registers/${encodeURIComponent(id)}`, {
      query: actorQuery(actor),
    })
  }

  async function createBillingRegister(
    actor: SettlementActor,
    body: {
      register_number: string
      contractor_company_id: string
      period_from: string
      period_to: string
      currency_code: string
      vat_rate?: number
    },
  ) {
    return apiPost<BillingRegister>('/api/v1/billing-registers', {
      customer_company_id: requireCompanyId(tenantStore.currentCompanyId),
      ...body,
    }, { query: actorQuery(actor) })
  }

  async function includeSettlementInRegister(registerId: string, actor: SettlementActor, settlementId: string) {
    return apiPost<BillingRegisterDetail>(
      `/api/v1/billing-registers/${encodeURIComponent(registerId)}/settlements`,
      { settlement_id: settlementId },
      { query: actorQuery(actor) },
    )
  }

  async function calculateBillingRegister(registerId: string, actor: SettlementActor) {
    return apiPost<BillingRegister>(
      `/api/v1/billing-registers/${encodeURIComponent(registerId)}/calculate`,
      {},
      { query: actorQuery(actor) },
    )
  }

  async function approveBillingRegister(registerId: string, actor: SettlementActor) {
    return apiPost<BillingRegister>(
      `/api/v1/billing-registers/${encodeURIComponent(registerId)}/approve`,
      {},
      { query: actorQuery(actor) },
    )
  }

  async function createClosingDocumentPackage(registerId: string, actor: SettlementActor, packageNumber: string) {
    return apiPost(
      `/api/v1/billing-registers/${encodeURIComponent(registerId)}/closing-document-package`,
      { package_number: packageNumber, package_type: 'ACT_PLUS_VAT_INVOICE' },
      { query: actorQuery(actor) },
    )
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
    getBillingRegister,
    createBillingRegister,
    includeSettlementInRegister,
    calculateBillingRegister,
    approveBillingRegister,
    createClosingDocumentPackage,
  }
}
