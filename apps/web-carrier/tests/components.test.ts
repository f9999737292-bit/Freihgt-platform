import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import CompanyGate from '../components/CompanyGate.vue'
import FleetView from '../components/FleetView.vue'
import PortalState from '../components/PortalState.vue'

const companies = [{ companyId: 'co-1', legalName: 'Carrier Co' }]

describe('customer state components', () => {
  it('renders loading, empty, forbidden, not found, and unavailable', () => {
    expect(mount(PortalState, { props: { kind: 'loading', title: 'Loading' } }).text()).toContain('Loading')
    expect(mount(PortalState, { props: { kind: 'empty', title: 'No tenders' } }).text()).toContain('No tenders')
    expect(mount(PortalState, { props: { kind: 'forbidden', title: 'Forbidden' } }).find('[role="alert"]').text()).toContain('Forbidden')
    expect(mount(PortalState, { props: { kind: 'not_found', title: 'Not found' } }).text()).toContain('Not found')
    expect(mount(PortalState, { props: { kind: 'unavailable', title: 'Unavailable' } }).text()).toContain('Unavailable')
  })

  it('shows the company gate and emits only a listed company', async () => {
    const gate = mount(CompanyGate, { props: { companies } })
    expect(gate.find('[data-testid="company-gate"]').exists()).toBe(true)
    await gate.find('form').trigger('submit')
    expect(gate.emitted('select')?.[0]).toEqual(['co-1'])

    const empty = mount(CompanyGate, { props: { companies: [] } })
    expect(empty.text()).toContain('carrier.companyGateEmpty')
    expect(empty.find('form').exists()).toBe(false)
  })

  it.each(['CARRIER_ADMIN', 'CARRIER_DISPATCHER'])('fleet view for %s has no create control', (role) => {
    const view = mount(FleetView, {
      props: {
        role,
        drivers: [{ id: 'd-1', full_name: 'Driver', status: 'ACTIVE' }],
        vehicles: [{ id: 'v-1', plate_number: 'A100', status: 'ACTIVE' }],
      },
    })
    expect(view.find('[data-testid="fleet-view"]').exists()).toBe(true)
    expect(view.find('[data-testid="fleet-create"]').exists()).toBe(false)
  })
})
