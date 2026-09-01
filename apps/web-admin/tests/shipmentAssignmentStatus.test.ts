import { describe, expect, it } from 'vitest'
import {
  canAssignDriver,
  canAssignVehicle,
  getNextShipmentStatus,
} from '../types/shipment'

describe('shipment assignment status UX contract', () => {
  it('hides generic Next status for ACCEPTED_BY_CARRIER', () => {
    expect(getNextShipmentStatus('ACCEPTED_BY_CARRIER')).toBeNull()
    expect(canAssignVehicle('ACCEPTED_BY_CARRIER')).toBe(true)
    expect(canAssignDriver('ACCEPTED_BY_CARRIER')).toBe(true)
  })

  it('hides generic Next status for VEHICLE_ASSIGNED', () => {
    expect(getNextShipmentStatus('VEHICLE_ASSIGNED')).toBeNull()
    expect(canAssignDriver('VEHICLE_ASSIGNED')).toBe(true)
  })

  it('shows operational Next status only after DRIVER_ASSIGNED', () => {
    expect(getNextShipmentStatus('DRIVER_ASSIGNED')).toBe('PICKUP_SLOT_BOOKED')
  })

  it('preserves operational progression after assignment complete', () => {
    expect(getNextShipmentStatus('PICKUP_SLOT_BOOKED')).toBe('IN_PICKUP')
    expect(getNextShipmentStatus('IN_PICKUP')).toBe('LOADED')
    expect(getNextShipmentStatus('LOADED')).toBe('IN_TRANSIT')
  })
})
