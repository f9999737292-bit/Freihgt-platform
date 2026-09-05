import type { UserCompanyMembership } from '~/types/user'
import { BUYER_OWNER_COMPANY_TYPES, isBuyerOwnerCompanyType } from '~/types/rfx'

export function filterBuyerOwnerMemberships(memberships: UserCompanyMembership[]) {
  return memberships.filter(
    (membership) =>
      membership.membership_status === 'ACTIVE' && isBuyerOwnerCompanyType(membership.company_type),
  )
}

export function membershipsToOwnerSelectOptions(memberships: UserCompanyMembership[]) {
  return filterBuyerOwnerMemberships(memberships).map((membership) => ({
    label: `${membership.legal_name} (${membership.company_type})`,
    value: membership.company_id,
  }))
}

export function useRfxOwnerCompanies() {
  const { user } = useAuth()
  const { getUserCompanies, isApiUnavailableError } = useUsersApi()

  async function loadAuthorizedOwnerCompanies() {
    const currentUser = user.value
    if (!currentUser?.id) {
      return { options: [], memberships: [], state: 'error' as const }
    }

    const memberships = await getUserCompanies(currentUser.id)
    const buyerMemberships = filterBuyerOwnerMemberships(memberships)
    return {
      options: membershipsToOwnerSelectOptions(memberships),
      memberships: buyerMemberships,
      state: buyerMemberships.length > 0 ? ('ready' as const) : ('empty' as const),
    }
  }

  return {
    buyerOwnerCompanyTypes: BUYER_OWNER_COMPANY_TYPES,
    filterBuyerOwnerMemberships,
    membershipsToOwnerSelectOptions,
    loadAuthorizedOwnerCompanies,
    isApiUnavailableError,
  }
}
