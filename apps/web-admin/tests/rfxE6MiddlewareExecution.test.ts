/**
 * E6 middleware execution — invoke rfx-buyer-manage with mocked composables.
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'

const navigateTo = vi.fn()
const enabled = ref(true)
const canManageRfxTemplates = vi.fn(() => true)
const isCarrierWithoutBuyerAccess = vi.fn(() => false)

vi.stubGlobal('navigateTo', navigateTo)
vi.stubGlobal('defineNuxtRouteMiddleware', (handler: () => unknown) => handler)
vi.stubGlobal('useRfxVersioningFeature', () => ({ enabled }))
vi.stubGlobal('useRfxBuyerPermissions', () => ({
  canManageRfxTemplates,
  isCarrierWithoutBuyerAccess,
}))

async function runBuyerManageMiddleware() {
  vi.resetModules()
  navigateTo.mockClear()
  const mod = await import('../middleware/rfx-buyer-manage')
  return mod.default()
}

describe('E6 middleware execution', () => {
  beforeEach(() => {
    enabled.value = true
    canManageRfxTemplates.mockReturnValue(true)
    isCarrierWithoutBuyerAccess.mockReturnValue(false)
  })

  it('E6-MW-01 redirects when versioning feature flag is disabled', async () => {
    enabled.value = false
    await runBuyerManageMiddleware()
    expect(navigateTo).toHaveBeenCalledTimes(1)
    expect(navigateTo).toHaveBeenCalledWith('/rfx')
  })

  it('E6-MW-02 redirects carrier without buyer manage access', async () => {
    isCarrierWithoutBuyerAccess.mockReturnValue(true)
    await runBuyerManageMiddleware()
    expect(navigateTo).toHaveBeenCalledTimes(1)
    expect(navigateTo).toHaveBeenCalledWith('/rfx')
  })

  it('E6-MW-03 redirects when buyer lacks template manage permission', async () => {
    canManageRfxTemplates.mockReturnValue(false)
    await runBuyerManageMiddleware()
    expect(navigateTo).toHaveBeenCalledTimes(1)
    expect(navigateTo).toHaveBeenCalledWith('/rfx')
  })

  it('E6-MW-04 allows buyer with manage permission and enabled feature', async () => {
    await runBuyerManageMiddleware()
    expect(navigateTo).not.toHaveBeenCalled()
  })
})
