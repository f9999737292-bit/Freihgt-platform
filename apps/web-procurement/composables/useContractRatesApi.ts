import type {
  ContractListResponse,
  CreateRateCardInput,
  CreateRateComponentInput,
  CreateRateLineInput,
  CreateRateVersionInput,
  CreateTransportContractInput,
  PatchRateComponentInput,
  PatchRateLineInput,
  PatchRateVersionInput,
  PatchTransportContractInput,
  RateCard,
  RateCardListResponse,
  RateCardVersion,
  RateComponent,
  RateComponentListResponse,
  RateLine,
  RateLineListResponse,
  RateResolutionRequest,
  RateResolutionResult,
  RateVersionListResponse,
  TransportContract,
} from '~/types/contractRate'

/**
 * Typed adapter for the future public contract-rate gateway contract (/api/v1/*).
 * v2.0D: routes are not live in api-gateway; tests mock this layer.
 */
export function useContractRatesApi() {
  const { apiGet, apiPost, apiPatch, apiDelete } = useApi()

  async function listTransportContracts() {
    const data = await apiGet<ContractListResponse>('/api/v1/transport-contracts')
    return data.items ?? []
  }

  async function getTransportContract(id: string) {
    return apiGet<TransportContract>(`/api/v1/transport-contracts/${encodeURIComponent(id)}`)
  }

  async function createTransportContract(input: CreateTransportContractInput) {
    return apiPost<TransportContract>('/api/v1/transport-contracts', input)
  }

  async function patchTransportContract(id: string, input: PatchTransportContractInput) {
    return apiPatch<TransportContract>(`/api/v1/transport-contracts/${encodeURIComponent(id)}`, input)
  }

  async function activateTransportContract(id: string) {
    return apiPost<TransportContract>(`/api/v1/transport-contracts/${encodeURIComponent(id)}/activate`)
  }

  async function suspendTransportContract(id: string) {
    return apiPost<TransportContract>(`/api/v1/transport-contracts/${encodeURIComponent(id)}/suspend`)
  }

  async function reactivateTransportContract(id: string) {
    return apiPost<TransportContract>(`/api/v1/transport-contracts/${encodeURIComponent(id)}/reactivate`)
  }

  async function terminateTransportContract(id: string, termination_reason?: string | null) {
    return apiPost<TransportContract>(
      `/api/v1/transport-contracts/${encodeURIComponent(id)}/terminate`,
      termination_reason ? { termination_reason } : undefined,
    )
  }

  async function cancelTransportContract(id: string) {
    return apiPost<TransportContract>(`/api/v1/transport-contracts/${encodeURIComponent(id)}/cancel`)
  }

  async function listRateCards(contractId: string) {
    const data = await apiGet<RateCardListResponse>(
      `/api/v1/transport-contracts/${encodeURIComponent(contractId)}/rate-cards`,
    )
    return data.items ?? []
  }

  async function createRateCard(contractId: string, input: CreateRateCardInput) {
    return apiPost<RateCard>(
      `/api/v1/transport-contracts/${encodeURIComponent(contractId)}/rate-cards`,
      input,
    )
  }

  async function getRateCard(id: string) {
    return apiGet<RateCard>(`/api/v1/rate-cards/${encodeURIComponent(id)}`)
  }

  async function listRateCardVersions(rateCardId: string) {
    const data = await apiGet<RateVersionListResponse>(
      `/api/v1/rate-cards/${encodeURIComponent(rateCardId)}/versions`,
    )
    return data.items ?? []
  }

  async function createRateCardVersion(rateCardId: string, input: CreateRateVersionInput) {
    return apiPost<RateCardVersion>(
      `/api/v1/rate-cards/${encodeURIComponent(rateCardId)}/versions`,
      input,
    )
  }

  async function getRateCardVersion(versionId: string) {
    return apiGet<RateCardVersion>(`/api/v1/rate-card-versions/${encodeURIComponent(versionId)}`)
  }

  async function patchRateCardVersion(versionId: string, input: PatchRateVersionInput) {
    return apiPatch<RateCardVersion>(
      `/api/v1/rate-card-versions/${encodeURIComponent(versionId)}`,
      input,
    )
  }

  async function discardRateCardVersion(versionId: string) {
    return apiDelete(`/api/v1/rate-card-versions/${encodeURIComponent(versionId)}`)
  }

  async function activateRateCardVersion(versionId: string) {
    return apiPost<RateCardVersion>(
      `/api/v1/rate-card-versions/${encodeURIComponent(versionId)}/activate`,
    )
  }

  async function listRateLines(versionId: string) {
    const data = await apiGet<RateLineListResponse>(
      `/api/v1/rate-card-versions/${encodeURIComponent(versionId)}/rate-lines`,
    )
    return data.items ?? []
  }

  async function createRateLine(versionId: string, input: CreateRateLineInput) {
    return apiPost<RateLine>(
      `/api/v1/rate-card-versions/${encodeURIComponent(versionId)}/rate-lines`,
      input,
    )
  }

  async function getRateLine(id: string) {
    return apiGet<RateLine>(`/api/v1/rate-lines/${encodeURIComponent(id)}`)
  }

  async function patchRateLine(id: string, input: PatchRateLineInput) {
    return apiPatch<RateLine>(`/api/v1/rate-lines/${encodeURIComponent(id)}`, input)
  }

  async function deleteRateLine(id: string) {
    return apiDelete(`/api/v1/rate-lines/${encodeURIComponent(id)}`)
  }

  async function listRateComponents(lineId: string) {
    const data = await apiGet<RateComponentListResponse>(
      `/api/v1/rate-lines/${encodeURIComponent(lineId)}/components`,
    )
    return data.items ?? []
  }

  async function createRateComponent(lineId: string, input: CreateRateComponentInput) {
    return apiPost<RateComponent>(
      `/api/v1/rate-lines/${encodeURIComponent(lineId)}/components`,
      input,
    )
  }

  async function patchRateComponent(id: string, input: PatchRateComponentInput) {
    return apiPatch<RateComponent>(`/api/v1/rate-components/${encodeURIComponent(id)}`, input)
  }

  async function deleteRateComponent(id: string) {
    return apiDelete(`/api/v1/rate-components/${encodeURIComponent(id)}`)
  }

  async function resolveRate(input: RateResolutionRequest) {
    return apiPost<RateResolutionResult>('/api/v1/rates/resolve', input)
  }

  return {
    listTransportContracts,
    getTransportContract,
    createTransportContract,
    patchTransportContract,
    activateTransportContract,
    suspendTransportContract,
    reactivateTransportContract,
    terminateTransportContract,
    cancelTransportContract,
    listRateCards,
    createRateCard,
    getRateCard,
    listRateCardVersions,
    createRateCardVersion,
    getRateCardVersion,
    patchRateCardVersion,
    discardRateCardVersion,
    activateRateCardVersion,
    listRateLines,
    createRateLine,
    getRateLine,
    patchRateLine,
    deleteRateLine,
    listRateComponents,
    createRateComponent,
    patchRateComponent,
    deleteRateComponent,
    resolveRate,
  }
}
