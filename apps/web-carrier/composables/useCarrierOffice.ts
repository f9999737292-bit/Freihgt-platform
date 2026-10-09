import type { CarrierCompanyMembership, CarrierTabSession, ServerUserSnapshot } from '@freight-platform/shared-ts/types'
import {
  PortalClient,
  canEnterCarrierPortal,
  carrierCompanies,
  clearTabSession,
  readTabSession,
  selectCarrierCompany,
  userFromLogin,
  writeTabSession,
} from '@freight-platform/portal-client'

export function useCarrierOffice() {
  const session = useState<CarrierTabSession | null>('carrier-tab-session', () => null)
  const notice = useState<string | null>('carrier-notice', () => null)
  const config = useRuntimeConfig()

  function persist(next: CarrierTabSession | null) {
    session.value = next
    if (!import.meta.client) return
    if (next) writeTabSession(window.sessionStorage, next)
    else clearTabSession(window.sessionStorage)
  }

  function hydrate() {
    if (!import.meta.client) return
    session.value = readTabSession(window.sessionStorage)
  }

  const client = new PortalClient({
    baseUrl: String(config.public.apiBaseUrl || 'http://localhost:8080'),
    getAccessToken: () => session.value?.accessToken ?? null,
    getAllowedCompanyIds: () => carrierCompanies(session.value?.memberships ?? []).map((item) => item.companyId),
    getLocale: () => readPortalLocale(),
    onUnauthorized: () => {
      persist(null)
      if (import.meta.client) void navigateTo('/login')
    },
  })

  async function login(input: { tenantId: string; email: string; password: string }) {
    const payload = await client.login(input)
    const user = userFromLogin(payload)
    const draft: CarrierTabSession = {
      accessToken: payload.access_token,
      user,
      tenantId: user.tenantId,
      selectedCompanyId: null,
      memberships: [],
    }
    session.value = draft
    const memberships = await client.listMemberships(user)
    if (!canEnterCarrierPortal(user.roles, memberships)) {
      persist(null)
      return { ok: false as const, reason: 'role' as const }
    }
    persist({ ...draft, memberships })
    return { ok: true as const }
  }

  function chooseCompany(companyId: string) {
    if (!session.value) return
    const selected = selectCarrierCompany(session.value.memberships, companyId)
    persist({ ...session.value, selectedCompanyId: selected })
  }

  function signOut() {
    notice.value = null
    persist(null)
    return navigateTo('/login')
  }

  const companies = computed(() => carrierCompanies(session.value?.memberships ?? []))
  const selectedCompanyId = computed(() => session.value?.selectedCompanyId ?? null)

  function requireCompany(): string {
    const companyId = selectedCompanyId.value
    if (!companyId) {
      throw new Error('company gate')
    }
    return selectCarrierCompany(session.value?.memberships ?? [], companyId)
  }

  return {
    session,
    notice,
    client,
    companies,
    selectedCompanyId,
    hydrate,
    login,
    chooseCompany,
    signOut,
    requireCompany,
  }
}

export type { CarrierCompanyMembership, ServerUserSnapshot }

function readPortalLocale(): string {
  if (import.meta.client) {
    const match = document.cookie.match(/(?:^|; )freight_locale=([^;]+)/)
    if (match?.[1]) return decodeURIComponent(match[1])
  }
  return 'ru-RU'
}
