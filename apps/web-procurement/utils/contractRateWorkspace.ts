import type { ContractStatus, PatchRateComponentInput, PatchRateLineInput, TransportContract } from '~/types/contractRate'
import { buildCreateRateLinePayload } from '~/utils/contractRate'
import { ApiError } from '~/utils/apiClient'
import { isApiUnavailableError, shouldShowNotFound } from '~/utils/apiError'

export type ContractListViewState =
  | 'loading'
  | 'missing_company'
  | 'forbidden'
  | 'backend_unavailable'
  | 'empty'
  | 'ready'

export type ContractDetailViewState =
  | 'loading'
  | 'not_found'
  | 'forbidden'
  | 'backend_unavailable'
  | 'ready'

export type LifecycleAction = 'activate' | 'suspend' | 'reactivate' | 'terminate' | 'cancel'

export interface ContractLifecycleApi {
  activateTransportContract(id: string): Promise<TransportContract>
  suspendTransportContract(id: string): Promise<TransportContract>
  reactivateTransportContract(id: string): Promise<TransportContract>
  terminateTransportContract(id: string): Promise<TransportContract>
  cancelTransportContract(id: string): Promise<TransportContract>
}

export function resolveContractListViewState(input: {
  loading: boolean
  missingCompany: boolean
  forbidden: boolean
  apiUnavailable: boolean
  itemCount: number
}): ContractListViewState {
  if (input.loading) return 'loading'
  if (input.missingCompany) return 'missing_company'
  if (input.forbidden) return 'forbidden'
  if (input.apiUnavailable) return 'backend_unavailable'
  if (input.itemCount === 0) return 'empty'
  return 'ready'
}

export function resolveContractDetailViewState(input: {
  loading: boolean
  notFound: boolean
  forbidden: boolean
  apiUnavailable: boolean
  hasContract: boolean
}): ContractDetailViewState {
  if (input.loading) return 'loading'
  if (input.notFound) return 'not_found'
  if (input.forbidden) return 'forbidden'
  if (input.apiUnavailable) return 'backend_unavailable'
  if (input.hasContract) return 'ready'
  return 'loading'
}

export function resolveContractDetailError(error: unknown): 'forbidden' | 'not_found' | 'backend_unavailable' | 'error' {
  if (error instanceof ApiError && error.status === 403) return 'forbidden'
  if (shouldShowNotFound(error)) return 'not_found'
  if (isApiUnavailableError(error)) return 'backend_unavailable'
  return 'error'
}

export function mapContractRateErrorCode(error: unknown): string {
  if (error instanceof ApiError) {
    return String(error.details?.code ?? error.code ?? 'UNKNOWN')
  }
  return 'UNKNOWN'
}

export function requiresLifecycleConfirmation(action: string): boolean {
  return ['activate', 'suspend', 'reactivate', 'terminate', 'cancel'].includes(action)
}

export async function runContractLifecycleAction(
  api: ContractLifecycleApi,
  contractId: string,
  action: LifecycleAction,
): Promise<TransportContract> {
  switch (action) {
    case 'activate':
      return api.activateTransportContract(contractId)
    case 'suspend':
      return api.suspendTransportContract(contractId)
    case 'reactivate':
      return api.reactivateTransportContract(contractId)
    case 'terminate':
      return api.terminateTransportContract(contractId)
    case 'cancel':
      return api.cancelTransportContract(contractId)
  }
}

export function terminalHistoryNavVisible(status: ContractStatus): boolean {
  return true
}

export function lifecycleMutationVisible(status: ContractStatus): boolean {
  return status !== 'TERMINATED' && status !== 'EXPIRED' && status !== 'CANCELLED'
}

export async function patchDraftRateVersion(
  patchRateCardVersion: (versionId: string, input: { valid_from?: string | null; valid_to?: string | null }) => Promise<unknown>,
  versionId: string,
  input: { valid_from: string; valid_to?: string },
) {
  return patchRateCardVersion(versionId, {
    valid_from: input.valid_from,
    valid_to: input.valid_to?.trim() ? input.valid_to : null,
  })
}

export async function patchDraftRateLine(
  patchRateLine: (lineId: string, input: PatchRateLineInput) => Promise<unknown>,
  lineId: string,
  input: { origin_location_id: string; destination_location_id: string; equipment_type: string; transport_mode?: string },
) {
  return patchRateLine(lineId, buildCreateRateLinePayload(input))
}

export async function deleteDraftRateLine(
  deleteRateLine: (lineId: string) => Promise<unknown>,
  lineId: string,
) {
  return deleteRateLine(lineId)
}

export async function patchDraftRateComponent(
  patchRateComponent: (componentId: string, input: PatchRateComponentInput) => Promise<unknown>,
  componentId: string,
  input: PatchRateComponentInput,
) {
  return patchRateComponent(componentId, input)
}

export async function deleteDraftRateComponent(
  deleteRateComponent: (componentId: string) => Promise<unknown>,
  componentId: string,
) {
  return deleteRateComponent(componentId)
}
