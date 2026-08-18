import { describe, expect, it } from 'vitest'
import {
  filterBuyerMemberships,
  isBuyerMembership,
  membershipSelectOptions,
  selectDefaultOwnerCompany,
} from '~/utils/companyMembership'
import type { UserCompanyMembership } from '~/types/company'

function membership(
  overrides: Partial<UserCompanyMembership> & Pick<UserCompanyMembership, 'company_id' | 'legal_name'>,
): UserCompanyMembership {
  return {
    membership_id: 'm-1',
    company_type: 'SHIPPER',
    membership_status: 'ACTIVE',
    roles: [{ role_id: 'r-1', code: 'PROCUREMENT_MANAGER', name: 'Procurement' }],
    ...overrides,
  }
}

describe('companyMembership', () => {
  it('accepts active buyer roles only', () => {
    expect(isBuyerMembership(membership({ company_id: 'c-1', legal_name: 'Shipper A' }))).toBe(true)
    expect(
      isBuyerMembership(
        membership({
          company_id: 'c-2',
          legal_name: 'Carrier B',
          roles: [{ role_id: 'r-2', code: 'CARRIER_ADMIN', name: 'Carrier' }],
        }),
      ),
    ).toBe(false)
    expect(
      isBuyerMembership(
        membership({
          company_id: 'c-3',
          legal_name: 'Inactive',
          membership_status: 'SUSPENDED',
        }),
      ),
    ).toBe(false)
  })

  it('filters and selects default owner company', () => {
    const memberships = [
      membership({ company_id: 'c-1', legal_name: 'Buyer One' }),
      membership({
        company_id: 'c-2',
        legal_name: 'Carrier',
        roles: [{ role_id: 'r-2', code: 'CARRIER_ADMIN', name: 'Carrier' }],
      }),
    ]

    const buyers = filterBuyerMemberships(memberships)
    expect(buyers).toHaveLength(1)
    expect(selectDefaultOwnerCompany(buyers)).toBe('c-1')
    expect(membershipSelectOptions(buyers)[0]?.value).toBe('c-1')
  })
})
