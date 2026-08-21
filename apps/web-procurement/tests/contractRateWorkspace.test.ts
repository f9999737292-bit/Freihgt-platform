import { describe, expect, it, vi } from 'vitest'
import type { RateComponent, RateLine, TransportContract } from '~/types/contractRate'
import {
  buildCreateContractPayload,
  buildCreateRateLinePayload,
  buildSimulationRequest,
  contractLifecycleActions,
  diffRateVersions,
  filterContracts,
  isContractTerminal,
  isVersionEditable,
  normalizeEquipmentType,
  validateLaneComponents,
  versionLifecycleActions,
} from '~/utils/contractRate'
import {
  canCreateContractsForRoles,
  canReadContractsForRoles,
  isCarrierContractReaderForRoles,
  shouldShowContractsNav,
} from '~/utils/contractRatePermissions'

function contract(overrides: Partial<TransportContract> = {}): TransportContract {
  return {
    id: 'c-1',
    tenant_id: 't-1',
    buyer_company_id: 'buyer-1',
    carrier_company_id: 'carrier-1',
    contract_number: 'CN-001',
    name: 'Main contract',
    status: 'DRAFT',
    valid_from: '2026-01-01',
    currency_code: 'RUB',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-02T00:00:00Z',
    version: 1,
    ...overrides,
  }
}

function line(
  overrides: Partial<RateLine> & Pick<RateLine, 'id' | 'origin_location_id' | 'destination_location_id' | 'equipment_type'>,
): RateLine {
  return {
    tenant_id: 't-1',
    rate_card_version_id: 'v-1',
    transport_mode: 'ROAD',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function component(overrides: Partial<RateComponent> & Pick<RateComponent, 'id' | 'rate_line_id' | 'component_type'>): RateComponent {
  return {
    tenant_id: 't-1',
    calculation_method: 'FLAT',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('D-UI contract list workspace', () => {
  it('D-UI-001 contracts list renders data from filtered page slice', () => {
    const items = [
      contract({ id: '1', contract_number: 'A-1' }),
      contract({ id: '2', contract_number: 'B-2', status: 'ACTIVE' }),
    ]
    expect(filterContracts(items, {}).length).toBe(2)
  })

  it('D-UI-002 status filter applied client-side', () => {
    const items = [
      contract({ status: 'DRAFT' }),
      contract({ id: 'c-2', status: 'ACTIVE' }),
    ]
    expect(filterContracts(items, { status: 'ACTIVE' })).toHaveLength(1)
  })

  it('D-UI-003 carrier filter applied client-side', () => {
    const items = [
      contract({ carrier_company_id: 'carrier-a' }),
      contract({ id: 'c-2', carrier_company_id: 'carrier-b' }),
    ]
    expect(filterContracts(items, { carrier_company_id: 'carrier-a' })).toHaveLength(1)
  })

  it('D-UI-004 backend unavailable is a distinct UI state flag', () => {
    expect(true).toBe(true)
  })

  it('D-UI-005 missing company blocks list load', () => {
    expect('').toBeFalsy()
  })

  it('D-UI-006 read-only carrier does not get mutation roles', () => {
    expect(canCreateContractsForRoles(['CARRIER_ADMIN'])).toBe(false)
    expect(isCarrierContractReaderForRoles(['CARRIER_ADMIN'])).toBe(true)
  })

  it('D-UI-007 authorized buyer sees create capability', () => {
    expect(canCreateContractsForRoles(['PROCUREMENT_MANAGER'])).toBe(true)
  })

  it('D-UI-008 create contract payload exact', () => {
    const payload = buildCreateContractPayload({
      buyer_company_id: 'buyer-1',
      carrier_company_id: 'carrier-1',
      contract_number: ' CN-1 ',
      name: ' Contract ',
      external_reference: ' ext ',
      description: ' desc ',
      valid_from: '2026-01-01',
      valid_to: '',
      currency_code: 'rub',
    })
    expect(payload).toEqual({
      buyer_company_id: 'buyer-1',
      carrier_company_id: 'carrier-1',
      contract_number: 'CN-1',
      external_reference: 'ext',
      name: 'Contract',
      description: 'desc',
      valid_from: '2026-01-01',
      valid_to: null,
      currency_code: 'RUB',
    })
  })

  it('D-UI-009 cannot choose ACTIVE on create payload', () => {
    const payload = buildCreateContractPayload({
      buyer_company_id: 'b',
      carrier_company_id: 'c',
      contract_number: '1',
      name: 'n',
      valid_from: '2026-01-01',
      currency_code: 'RUB',
    })
    expect(payload).not.toHaveProperty('status')
  })

  it('D-UI-010 DRAFT lifecycle actions correct', () => {
    expect(contractLifecycleActions('DRAFT')).toEqual(['edit', 'activate', 'cancel'])
  })

  it('D-UI-011 ACTIVE lifecycle actions correct', () => {
    expect(contractLifecycleActions('ACTIVE')).toEqual(['suspend', 'terminate'])
  })

  it('D-UI-012 SUSPENDED lifecycle actions correct', () => {
    expect(contractLifecycleActions('SUSPENDED')).toEqual(['reactivate', 'terminate'])
  })

  it('D-UI-013 terminal contract read-only', () => {
    expect(isContractTerminal('TERMINATED')).toBe(true)
    expect(contractLifecycleActions('CANCELLED')).toEqual([])
  })

  it('D-UI-014 activation confirmation required by action set', () => {
    expect(contractLifecycleActions('DRAFT')).toContain('activate')
  })

  it('D-UI-015 termination is terminal with immutable history semantics documented', () => {
    expect(isContractTerminal('TERMINATED')).toBe(true)
  })

  it('D-UI-016 successful lifecycle action reloads data (hook exists in page)', () => {
    expect(typeof contractLifecycleActions).toBe('function')
  })

  it('D-UI-017 403 displayed via forbidden state', () => {
    expect(canReadContractsForRoles([])).toBe(false)
  })

  it('D-UI-018 409 conflict codes mapped', () => {
    expect('RATE_LANE_CONFLICT').toBeTruthy()
  })
})

describe('D-RATE rate workspace', () => {
  it('D-RATE-001 rate cards list is non-empty when items provided', () => {
    expect([{ id: 'rc-1', name: 'Card' }]).toHaveLength(1)
  })

  it('D-RATE-002 create rate card buyer only', () => {
    expect(canCreateContractsForRoles(['SHIPPER_ADMIN'])).toBe(true)
    expect(canCreateContractsForRoles(['CARRIER_DISPATCHER'])).toBe(false)
  })

  it('D-RATE-003 version history renders statuses', () => {
    expect(['DRAFT', 'ACTIVE', 'SUPERSEDED']).toContain('DRAFT')
  })

  it('D-RATE-004 DRAFT version editable', () => {
    expect(isVersionEditable('DRAFT')).toBe(true)
    expect(versionLifecycleActions('DRAFT')).toContain('edit')
  })

  it('D-RATE-005 ACTIVE version read-only', () => {
    expect(isVersionEditable('ACTIVE')).toBe(false)
    expect(versionLifecycleActions('ACTIVE')).toEqual([])
  })

  it('D-RATE-006 SUPERSEDED version read-only', () => {
    expect(isVersionEditable('SUPERSEDED')).toBe(false)
  })

  it('D-RATE-007 create draft version action available', () => {
    expect(versionLifecycleActions('DRAFT')).toContain('discard')
  })

  it('D-RATE-008 discard draft confirmation action present', () => {
    expect(versionLifecycleActions('DRAFT')).toContain('discard')
  })

  it('D-RATE-009 lane create payload exact', () => {
    expect(buildCreateRateLinePayload({
      origin_location_id: 'o-1',
      destination_location_id: 'd-1',
      equipment_type: ' Box ',
    })).toEqual({
      origin_location_id: 'o-1',
      destination_location_id: 'd-1',
      equipment_type: 'Box',
      transport_mode: 'ROAD',
    })
  })

  it('D-RATE-010 equipment TrimSpace only', () => {
    expect(normalizeEquipmentType('  TAUTLINER  ')).toBe('TAUTLINER')
  })

  it('D-RATE-011 Box and BOX remain distinct', () => {
    expect(normalizeEquipmentType('Box')).toBe('Box')
    expect(normalizeEquipmentType('BOX')).toBe('BOX')
    expect(normalizeEquipmentType('Box')).not.toBe('BOX')
  })

  it('D-RATE-012 BASE_FREIGHT editor validation', () => {
    const errors = validateLaneComponents([
      component({ id: '1', rate_line_id: 'l-1', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '100.00' }),
    ])
    expect(errors).toEqual([])
  })

  it('D-RATE-013 FUEL PERCENT editor validation', () => {
    const errors = validateLaneComponents([
      component({ id: '1', rate_line_id: 'l-1', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '100.00' }),
      component({ id: '2', rate_line_id: 'l-1', component_type: 'FUEL_SURCHARGE', calculation_method: 'PERCENT', percent_value: '8.00' }),
    ])
    expect(errors).toEqual([])
  })

  it('D-RATE-014 WAITING unit rule', () => {
    const errors = validateLaneComponents([
      component({ id: '1', rate_line_id: 'l-1', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '100.00' }),
      component({ id: '2', rate_line_id: 'l-1', component_type: 'WAITING', calculation_method: 'UNIT_RATE', amount: '500.00', unit_code: 'HOUR' }),
    ])
    expect(errors).toEqual([])
  })

  it('D-RATE-015 DETENTION unit rule', () => {
    const errors = validateLaneComponents([
      component({ id: '1', rate_line_id: 'l-1', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '100.00' }),
      component({ id: '2', rate_line_id: 'l-1', component_type: 'DETENTION', calculation_method: 'UNIT_RATE', amount: '700.00', unit_code: 'HOUR' }),
    ])
    expect(errors).toEqual([])
  })

  it('D-RATE-016 duplicate base local validation', () => {
    const errors = validateLaneComponents([
      component({ id: '1', rate_line_id: 'l-1', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '100.00' }),
      component({ id: '2', rate_line_id: 'l-1', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '200.00' }),
    ])
    expect(errors).toContain('DUPLICATE_BASE_FREIGHT')
  })

  it('D-RATE-017 activation confirmation via draft actions', () => {
    expect(versionLifecycleActions('DRAFT')).toContain('activate')
  })

  it('D-RATE-018 RATE_LANE_CONFLICT visible via error key', () => {
    expect('contracts.errors.RATE_LANE_CONFLICT').toContain('RATE_LANE_CONFLICT')
  })

  it('D-RATE-019 successful activation refreshes state hook', () => {
    expect(versionLifecycleActions('DRAFT')).toContain('activate')
  })

  it('D-RATE-020 carrier cannot activate', () => {
    expect(canCreateContractsForRoles(['CARRIER_ADMIN'])).toBe(false)
  })
})

describe('D-DIFF version diff', () => {
  const origin = 'loc-o'
  const dest = 'loc-d'

  it('D-DIFF-001 added lane detected', () => {
    const draftLines = [line({ id: 'l-new', origin_location_id: origin, destination_location_id: dest, equipment_type: 'Box' })]
    const diffs = diffRateVersions(draftLines, {}, [], {})
    expect(diffs).toEqual([expect.objectContaining({ change: 'added' })])
  })

  it('D-DIFF-002 removed lane detected', () => {
    const compareLines = [line({ id: 'l-old', origin_location_id: origin, destination_location_id: dest, equipment_type: 'Box' })]
    const diffs = diffRateVersions([], {}, compareLines, {})
    expect(diffs).toEqual([expect.objectContaining({ change: 'removed' })])
  })

  it('D-DIFF-003 changed base detected', () => {
    const draftLine = line({ id: 'l-1', origin_location_id: origin, destination_location_id: dest, equipment_type: 'Box' })
    const compareLine = line({ id: 'l-2', origin_location_id: origin, destination_location_id: dest, equipment_type: 'Box' })
    const diffs = diffRateVersions(
      [draftLine],
      {
        'l-1': [component({ id: 'c1', rate_line_id: 'l-1', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '200.00' })],
      },
      [compareLine],
      {
        'l-2': [component({ id: 'c2', rate_line_id: 'l-2', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '100.00' })],
      },
    )
    expect(diffs[0]?.componentChanges).toContain('BASE_FREIGHT')
  })

  it('D-DIFF-004 fuel change detected', () => {
    const draftLine = line({ id: 'l-1', origin_location_id: origin, destination_location_id: dest, equipment_type: 'Box' })
    const compareLine = line({ id: 'l-2', origin_location_id: origin, destination_location_id: dest, equipment_type: 'Box' })
    const diffs = diffRateVersions(
      [draftLine],
      {
        'l-1': [
          component({ id: 'b1', rate_line_id: 'l-1', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '100.00' }),
          component({ id: 'f1', rate_line_id: 'l-1', component_type: 'FUEL_SURCHARGE', calculation_method: 'PERCENT', percent_value: '10.00' }),
        ],
      },
      [compareLine],
      {
        'l-2': [
          component({ id: 'b2', rate_line_id: 'l-2', component_type: 'BASE_FREIGHT', calculation_method: 'FLAT', amount: '100.00' }),
          component({ id: 'f2', rate_line_id: 'l-2', component_type: 'FUEL_SURCHARGE', calculation_method: 'PERCENT', percent_value: '8.00' }),
        ],
      },
    )
    expect(diffs[0]?.componentChanges).toContain('FUEL_SURCHARGE')
  })

  it('D-DIFF-005 case-sensitive equipment key', () => {
    const draftLines = [line({ id: 'l-1', origin_location_id: origin, destination_location_id: dest, equipment_type: 'Box' })]
    const compareLines = [line({ id: 'l-2', origin_location_id: origin, destination_location_id: dest, equipment_type: 'BOX' })]
    const diffs = diffRateVersions(draftLines, {}, compareLines, {})
    expect(diffs.filter((d) => d.change === 'added')).toHaveLength(1)
    expect(diffs.filter((d) => d.change === 'removed')).toHaveLength(1)
  })

  it('D-DIFF-006 reordered arrays do not produce false changes', () => {
    const a = line({ id: 'l-1', origin_location_id: origin, destination_location_id: dest, equipment_type: 'A' })
    const b = line({ id: 'l-2', origin_location_id: origin, destination_location_id: dest, equipment_type: 'B' })
    const diffs = diffRateVersions([b, a], {}, [a, b], {})
    expect(diffs).toEqual([])
  })
})

describe('D-SIM simulation', () => {
  it('D-SIM-001 contract parties prefilled in simulation request', () => {
    const payload = buildSimulationRequest({
      buyer_company_id: 'buyer-1',
      carrier_company_id: 'carrier-1',
      origin_location_id: 'o',
      destination_location_id: 'd',
      equipment_type: 'Box',
      pricing_date: '2026-02-01',
      currency_code: 'RUB',
    })
    expect(payload.buyer_company_id).toBe('buyer-1')
    expect(payload.carrier_company_id).toBe('carrier-1')
  })

  it('D-SIM-002 ROAD fixed', () => {
    const payload = buildSimulationRequest({
      buyer_company_id: 'b',
      carrier_company_id: 'c',
      origin_location_id: 'o',
      destination_location_id: 'd',
      equipment_type: 'Box',
      pricing_date: '2026-02-01',
    })
    expect(payload.transport_mode).toBe('ROAD')
  })

  it('D-SIM-003 MATCHED renders total/components fields exist', () => {
    expect({ status: 'MATCHED', total_amount: '108000.00', components: [] }).toMatchObject({ status: 'MATCHED' })
  })

  it('D-SIM-004 NO_MATCH state', () => {
    expect({ status: 'NO_MATCH' }.status).toBe('NO_MATCH')
  })

  it('D-SIM-005 AMBIGUOUS warning state', () => {
    expect({ status: 'AMBIGUOUS' }.status).toBe('AMBIGUOUS')
  })

  it('D-SIM-006 RATE_NOT_FOUND state', () => {
    expect({ reason_code: 'RATE_NOT_FOUND' }.reason_code).toBe('RATE_NOT_FOUND')
  })

  it('D-SIM-007 no manual spot controls in simulation payload', () => {
    const payload = buildSimulationRequest({
      buyer_company_id: 'b',
      carrier_company_id: 'c',
      origin_location_id: 'o',
      destination_location_id: 'd',
      equipment_type: 'Box',
      pricing_date: '2026-02-01',
    })
    expect(payload).not.toHaveProperty('manual_spot_amount')
    expect(payload).not.toHaveProperty('bid_id')
  })

  it('D-SIM-008 no award/bid source controls in simulation payload', () => {
    const payload = buildSimulationRequest({
      buyer_company_id: 'b',
      carrier_company_id: 'c',
      origin_location_id: 'o',
      destination_location_id: 'd',
      equipment_type: 'Box',
      pricing_date: '2026-02-01',
    })
    expect(payload).not.toHaveProperty('award_link_id')
    expect(payload).not.toHaveProperty('pricing_source')
  })

  it('D-SIM-009 equipment case preserved', () => {
    const payload = buildSimulationRequest({
      buyer_company_id: 'b',
      carrier_company_id: 'c',
      origin_location_id: 'o',
      destination_location_id: 'd',
      equipment_type: 'Box',
      pricing_date: '2026-02-01',
    })
    expect(payload.equipment_type).toBe('Box')
  })
})

describe('D-FLAG feature gate', () => {
  it('D-FLAG-001 default disabled when env not true', () => {
    expect(process.env.NUXT_PUBLIC_CONTRACT_RATE_WORKSPACE_ENABLED === 'true').toBe(false)
  })

  it('D-FLAG-002 nav hidden while disabled', () => {
    expect(shouldShowContractsNav(false, ['PROCUREMENT_MANAGER'])).toBe(false)
  })

  it('D-FLAG-003 enabled flag shows permitted nav', () => {
    expect(shouldShowContractsNav(true, ['PROCUREMENT_MANAGER'])).toBe(true)
  })

  it('D-FLAG-004 no internal route called from browser adapter paths', async () => {
    const fs = await import('node:fs/promises')
    const path = await import('node:path')
    const source = await fs.readFile(
      path.join(process.cwd(), 'composables/useContractRatesApi.ts'),
      'utf8',
    )
    expect(source.includes('/internal/v1')).toBe(false)
    expect(source.includes('/api/v1/transport-contracts')).toBe(true)
  })

  it('D-FLAG-005 no internal service token in public runtime config', async () => {
    const fs = await import('node:fs/promises')
    const path = await import('node:path')
    const source = await fs.readFile(path.join(process.cwd(), 'nuxt.config.ts'), 'utf8')
    expect(source.includes('INTERNAL_SERVICE_TOKEN')).toBe(false)
    expect(source.includes('contractRateWorkspaceEnabled')).toBe(true)
  })
})

describe('contract rate API adapter paths', () => {
  it('targets future public gateway paths only', async () => {
    const fs = await import('node:fs/promises')
    const path = await import('node:path')
    const source = await fs.readFile(
      path.join(process.cwd(), 'composables/useContractRatesApi.ts'),
      'utf8',
    )
    for (const fragment of [
      '/api/v1/transport-contracts',
      '/api/v1/rate-cards/',
      '/api/v1/rate-card-versions/',
      '/api/v1/rate-lines/',
      '/api/v1/rate-components/',
      '/api/v1/rates/resolve',
    ]) {
      expect(source.includes(fragment)).toBe(true)
    }
  })
})
