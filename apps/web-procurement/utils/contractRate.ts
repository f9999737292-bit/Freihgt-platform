import type {
  ContractStatus,
  CreateTransportContractInput,
  RateCardVersion,
  RateCardVersionStatus,
  RateComponent,
  RateComponentType,
  RateLine,
  TransportContract,
} from '~/types/contractRate'

/** TrimSpace only — never change case. */
export function normalizeEquipmentType(value: string): string {
  return value.trim()
}

export function laneMatchKey(line: Pick<RateLine, 'origin_location_id' | 'destination_location_id' | 'equipment_type' | 'transport_mode'>): string {
  return [
    line.origin_location_id,
    line.destination_location_id,
    line.equipment_type,
    line.transport_mode || 'ROAD',
  ].join('|')
}

export function filterContracts(
  items: TransportContract[],
  filters: { q?: string; status?: ContractStatus | ''; carrier_company_id?: string },
): TransportContract[] {
  let result = items
  if (filters.status) {
    result = result.filter((item) => item.status === filters.status)
  }
  if (filters.carrier_company_id) {
    result = result.filter((item) => item.carrier_company_id === filters.carrier_company_id)
  }
  if (filters.q?.trim()) {
    const q = filters.q.trim().toLowerCase()
    result = result.filter((item) =>
      item.contract_number.toLowerCase().includes(q)
      || item.name.toLowerCase().includes(q)
      || (item.external_reference?.toLowerCase().includes(q) ?? false),
    )
  }
  return result
}

export function paginateItems<T>(items: T[], limit: number, offset: number): { items: T[]; total: number } {
  const total = items.length
  return { items: items.slice(offset, offset + limit), total }
}

export function contractLifecycleActions(status: ContractStatus): string[] {
  switch (status) {
    case 'DRAFT':
      return ['edit', 'activate', 'cancel']
    case 'ACTIVE':
      return ['suspend', 'terminate']
    case 'SUSPENDED':
      return ['reactivate', 'terminate']
    default:
      return []
  }
}

export function isContractTerminal(status: ContractStatus): boolean {
  return status === 'TERMINATED' || status === 'EXPIRED' || status === 'CANCELLED'
}

export function isContractDraftEditable(status: ContractStatus): boolean {
  return status === 'DRAFT'
}

export function isContractMetadataEditable(status: ContractStatus): boolean {
  return status === 'ACTIVE' || status === 'SUSPENDED'
}

export function canEditContractField(status: ContractStatus, field: keyof CreateTransportContractInput): boolean {
  if (isContractDraftEditable(status)) return true
  if (isContractMetadataEditable(status)) {
    return field === 'description' || field === 'external_reference'
  }
  return false
}

export function versionLifecycleActions(status: RateCardVersionStatus): string[] {
  if (status === 'DRAFT') return ['edit', 'discard', 'activate']
  return []
}

export function isVersionEditable(status: RateCardVersionStatus): boolean {
  return status === 'DRAFT'
}

export function buildCreateContractPayload(input: CreateTransportContractInput): CreateTransportContractInput {
  return {
    buyer_company_id: input.buyer_company_id,
    carrier_company_id: input.carrier_company_id,
    contract_number: input.contract_number.trim(),
    external_reference: input.external_reference?.trim() || null,
    name: input.name.trim(),
    description: input.description?.trim() || null,
    valid_from: input.valid_from,
    valid_to: input.valid_to || null,
    currency_code: input.currency_code.trim().toUpperCase(),
  }
}

export function buildCreateRateLinePayload(input: {
  origin_location_id: string
  destination_location_id: string
  equipment_type: string
  transport_mode?: string
}) {
  return {
    origin_location_id: input.origin_location_id,
    destination_location_id: input.destination_location_id,
    equipment_type: normalizeEquipmentType(input.equipment_type),
    transport_mode: input.transport_mode?.trim() || 'ROAD',
  }
}

const COMPONENT_METHOD: Record<RateComponentType, string> = {
  BASE_FREIGHT: 'FLAT',
  FUEL_SURCHARGE: 'PERCENT',
  WAITING: 'UNIT_RATE',
  DETENTION: 'UNIT_RATE',
}

export function expectedCalculationMethod(componentType: RateComponentType): string {
  return COMPONENT_METHOD[componentType]
}

export function validateLaneComponents(components: RateComponent[]): string[] {
  const errors: string[] = []
  const counts: Record<string, number> = {}
  for (const component of components) {
    counts[component.component_type] = (counts[component.component_type] ?? 0) + 1
    if (component.component_type === 'BASE_FREIGHT' && component.calculation_method !== 'FLAT') {
      errors.push('INVALID_BASE_FREIGHT_METHOD')
    }
    if (component.component_type === 'FUEL_SURCHARGE' && component.calculation_method !== 'PERCENT') {
      errors.push('INVALID_FUEL_METHOD')
    }
    if ((component.component_type === 'WAITING' || component.component_type === 'DETENTION')
      && component.calculation_method !== 'UNIT_RATE') {
      errors.push('INVALID_ACCESSORIAL_METHOD')
    }
  }
  if ((counts.BASE_FREIGHT ?? 0) !== 1) errors.push('DUPLICATE_BASE_FREIGHT')
  if ((counts.FUEL_SURCHARGE ?? 0) > 1) errors.push('DUPLICATE_FUEL_SURCHARGE')
  if ((counts.WAITING ?? 0) > 1) errors.push('DUPLICATE_WAITING')
  if ((counts.DETENTION ?? 0) > 1) errors.push('DUPLICATE_DETENTION')
  return errors
}

export interface LaneDiffEntry {
  key: string
  change: 'added' | 'removed' | 'changed'
  draftLine?: RateLine
  compareLine?: RateLine
  componentChanges?: string[]
}

export function diffRateVersions(
  draftLines: RateLine[],
  draftComponentsByLine: Record<string, RateComponent[]>,
  compareLines: RateLine[],
  compareComponentsByLine: Record<string, RateComponent[]>,
): LaneDiffEntry[] {
  const draftMap = new Map(draftLines.map((line) => [laneMatchKey(line), line]))
  const compareMap = new Map(compareLines.map((line) => [laneMatchKey(line), line]))
  const keys = new Set([...draftMap.keys(), ...compareMap.keys()])
  const diffs: LaneDiffEntry[] = []

  for (const key of keys) {
    const draftLine = draftMap.get(key)
    const compareLine = compareMap.get(key)
    if (draftLine && !compareLine) {
      diffs.push({ key, change: 'added', draftLine })
      continue
    }
    if (!draftLine && compareLine) {
      diffs.push({ key, change: 'removed', compareLine })
      continue
    }
    if (!draftLine || !compareLine) continue

    const draftComponents = draftComponentsByLine[draftLine.id] ?? []
    const compareComponents = compareComponentsByLine[compareLine.id] ?? []
    const componentChanges = diffComponents(draftComponents, compareComponents)
    if (componentChanges.length > 0) {
      diffs.push({ key, change: 'changed', draftLine, compareLine, componentChanges })
    }
  }

  return diffs
}

function componentSignature(component: RateComponent): string {
  return [
    component.component_type,
    component.calculation_method,
    component.amount ?? '',
    component.percent_value ?? '',
    component.unit_code ?? '',
  ].join(':')
}

function diffComponents(draft: RateComponent[], compare: RateComponent[]): string[] {
  const draftMap = new Map(draft.map((c) => [c.component_type, componentSignature(c)]))
  const compareMap = new Map(compare.map((c) => [c.component_type, componentSignature(c)]))
  const types = new Set([...draftMap.keys(), ...compareMap.keys()])
  const changes: string[] = []
  for (const type of types) {
    if (draftMap.get(type) !== compareMap.get(type)) {
      changes.push(type)
    }
  }
  return changes
}

export function findActiveVersion(versions: RateCardVersion[]): RateCardVersion | undefined {
  return versions.find((v) => v.status === 'ACTIVE')
}

export function findSupersededPredecessor(
  draftVersion: RateCardVersion,
  versions: RateCardVersion[],
): RateCardVersion | undefined {
  if (draftVersion.supersedes_version_id) {
    return versions.find((v) => v.id === draftVersion.supersedes_version_id)
  }
  return findActiveVersion(versions)
}

export function buildSimulationRequest(input: {
  buyer_company_id: string
  carrier_company_id: string
  origin_location_id: string
  destination_location_id: string
  equipment_type: string
  pricing_date: string
  currency_code?: string | null
}) {
  return {
    buyer_company_id: input.buyer_company_id,
    carrier_company_id: input.carrier_company_id,
    origin_location_id: input.origin_location_id,
    destination_location_id: input.destination_location_id,
    equipment_type: normalizeEquipmentType(input.equipment_type),
    transport_mode: 'ROAD',
    pricing_date: input.pricing_date,
    currency_code: input.currency_code ?? null,
  }
}

export function contractRateErrorMessage(code: string, t: (key: string) => string): string {
  const key = `contracts.errors.${code}`
  const translated = t(key)
  return translated === key ? code : translated
}
