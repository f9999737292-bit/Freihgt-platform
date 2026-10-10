import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import CompanyGate from '../components/CompanyGate.vue'
import PortalState from '../components/PortalState.vue'
import ShipmentFacts from '../components/ShipmentFacts.vue'

describe('shipper office components', () => {
  it('renders loading, empty, forbidden, not found, and unavailable', () => {
    expect(mount(PortalState, { props: { kind: 'loading', title: 'Loading' } }).text()).toContain('Loading')
    expect(mount(PortalState, { props: { kind: 'empty', title: 'No shipments' } }).text()).toContain('No shipments')
    expect(mount(PortalState, { props: { kind: 'forbidden', title: 'Forbidden' } }).find('[role="alert"]').text()).toContain('Forbidden')
    expect(mount(PortalState, { props: { kind: 'not_found', title: 'Shipment not found' } }).text()).toContain('Shipment not found')
    expect(mount(PortalState, { props: { kind: 'unavailable', title: 'Unavailable' } }).text()).toContain('Unavailable')
  })

  it('shows the company gate and emits only a listed company', async () => {
    const gate = mount(CompanyGate, { props: { companies: [{ companyId: 'co-shipper', legalName: 'Shipper Co' }] } })
    expect(gate.find('[data-testid="company-gate"]').exists()).toBe(true)
    await gate.find('form').trigger('submit')
    expect(gate.emitted('select')?.[0]).toEqual(['co-shipper'])

    const empty = mount(CompanyGate, { props: { companies: [] } })
    expect(empty.text()).toContain('shipper.companyGateEmpty')
    expect(empty.find('form').exists()).toBe(false)
  })

  it('shows server shipment facts and no tracking or document controls', () => {
    const view = mount(ShipmentFacts, {
      props: {
        shipment: {
          id: 'shp-1',
          shipment_number: 'SHP-1',
          status: 'BOOKED',
          carrier_company_id: 'co-carrier',
          consignee_company_id: 'co-consignee',
          transport_mode: 'ROAD',
          planned_pickup_at: '2026-12-01T08:00:00Z',
          planned_delivery_at: '2026-12-02T18:00:00Z',
          actual_pickup_at: '2026-12-01T09:00:00Z',
          actual_delivery_at: null,
        },
      },
    })
    expect(view.get('[data-testid="shipment-number"]').text()).toBe('SHP-1')
    expect(view.get('[data-testid="shipment-status"]').text()).toBe('BOOKED')
    expect(view.get('[data-testid="actual-delivery"]').text()).toBe('—')
    expect(view.text().toLowerCase()).not.toContain('eta')
    expect(view.find('[data-testid="tracking-map"]').exists()).toBe(false)
    expect(view.find('[data-testid="documents"]').exists()).toBe(false)
  })
})

describe('shipper api routes', () => {
  it('calls only the customer shipment reads', () => {
    const source = readFileSync(join(process.cwd(), 'composables/useShipperApi.ts'), 'utf8')
    expect(source).toContain('/api/v1/shipper/shipments')
    expect(source).toContain('shipper_company_id')
    expect(source.includes('/api/v1/shipments')).toBe(false)
  })
})
