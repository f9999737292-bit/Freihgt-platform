import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import type { UserCompanyMembership } from '../types/user'
import {
  filterBuyerOwnerMemberships,
  membershipsToOwnerSelectOptions,
} from '../composables/useRfxOwnerCompanies'
import { BUYER_OWNER_COMPANY_TYPES } from '../types/rfx'

function membership(overrides: Partial<UserCompanyMembership> & Pick<UserCompanyMembership, 'company_id' | 'legal_name' | 'company_type'>): UserCompanyMembership {
  return {
    membership_id: overrides.membership_id ?? 'mem-1',
    company_id: overrides.company_id,
    legal_name: overrides.legal_name,
    company_type: overrides.company_type,
    membership_status: overrides.membership_status ?? 'ACTIVE',
    roles: overrides.roles ?? [],
  }
}

describe('RFx owner company authorization semantics', () => {
  it('includes only active buyer company types', () => {
    const items = filterBuyerOwnerMemberships([
      membership({ company_id: 'shipper-1', legal_name: 'Wave2R8 Shipper', company_type: 'SHIPPER' }),
      membership({ company_id: 'carrier-1', legal_name: 'Wave2R8 Carrier', company_type: 'CARRIER' }),
      membership({ company_id: 'inactive-shipper', legal_name: 'Inactive', company_type: 'SHIPPER', membership_status: 'INACTIVE' }),
    ])

    expect(items.map((item) => item.company_id)).toEqual(['shipper-1'])
    expect(BUYER_OWNER_COMPANY_TYPES).toEqual(['SHIPPER', 'FORWARDER', 'LSP'])
  })

  it('builds select options from authorized memberships only', () => {
    const options = membershipsToOwnerSelectOptions([
      membership({ company_id: 'shipper-1', legal_name: 'Wave2R8 Shipper', company_type: 'SHIPPER' }),
      membership({ company_id: 'carrier-1', legal_name: 'Wave2R8 Carrier', company_type: 'CARRIER' }),
    ])

    expect(options).toEqual([{ label: 'Wave2R8 Shipper (SHIPPER)', value: 'shipper-1' }])
  })

  it('does not expose cross-company carrier ownership options', () => {
    const options = membershipsToOwnerSelectOptions([
      membership({ company_id: 'carrier-only', legal_name: 'Unauthorized Carrier', company_type: 'CARRIER' }),
    ])

    expect(options).toEqual([])
  })
})

describe('RfxCreateModal owner load UX (source contract)', () => {
  it('loads authorized owner companies via useRfxOwnerCompanies', () => {
    const modalSource = readFileSync(resolve(import.meta.dirname, '../components/rfx/RfxCreateModal.vue'), 'utf8')
    const composableSource = readFileSync(resolve(import.meta.dirname, '../composables/useRfxOwnerCompanies.ts'), 'utf8')

    expect(modalSource).toContain('useRfxOwnerCompanies')
    expect(modalSource).toContain('loadAuthorizedOwnerCompanies')
    expect(modalSource).not.toContain('listCompanies')
    expect(modalSource).toContain("ownerLoadState.value === 'error'")
    expect(modalSource).toContain("ownerLoadState.value === 'empty'")
    expect(composableSource).toContain('getUserCompanies')
  })

  it('surfaces explicit owner load failure instead of silent empty select', () => {
    const modalSource = readFileSync(resolve(import.meta.dirname, '../components/rfx/RfxCreateModal.vue'), 'utf8')

    expect(modalSource).toContain('rfx.ownerLoadFailed')
    expect(modalSource).not.toMatch(/companies\.value\s*=\s*\[\]/)
  })
})
