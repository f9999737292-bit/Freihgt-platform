export const SHIPMENT_STATUSES = [
  'CARRIER_ASSIGNED',
  'ACCEPTED_BY_CARRIER',
  'VEHICLE_ASSIGNED',
  'DRIVER_ASSIGNED',
  'PICKUP_SLOT_BOOKED',
  'DELIVERY_SLOT_BOOKED',
  'IN_PICKUP',
  'LOADED',
  'IN_TRANSIT',
  'ARRIVED_AT_CONSIGNEE',
  'UNLOADING',
  'DELIVERED',
  'DELIVERY_CONFIRMED',
  'DOCUMENTS_COMPLETED',
  'READY_FOR_BILLING',
  'INCLUDED_IN_BILLING_REGISTER',
  'FINANCIALLY_CLOSED',
  'CANCELLED',
] as const

export const VEHICLE_TYPES = ['TRUCK', 'TRAILER', 'VAN'] as const

export const EQUIPMENT_TYPES = ['TENT_20T', 'REF_20T', 'FLAT_20T', 'TILT_20T'] as const

export const DRIVER_STATUSES = ['ACTIVE', 'INACTIVE'] as const

export const VEHICLE_STATUSES = ['ACTIVE', 'INACTIVE'] as const

const NEXT_STATUS: Record<string, string> = {
  ACCEPTED_BY_CARRIER: 'VEHICLE_ASSIGNED',
  VEHICLE_ASSIGNED: 'DRIVER_ASSIGNED',
  DRIVER_ASSIGNED: 'PICKUP_SLOT_BOOKED',
  PICKUP_SLOT_BOOKED: 'IN_PICKUP',
  IN_PICKUP: 'LOADED',
  LOADED: 'IN_TRANSIT',
  IN_TRANSIT: 'ARRIVED_AT_CONSIGNEE',
  ARRIVED_AT_CONSIGNEE: 'UNLOADING',
  UNLOADING: 'DELIVERED',
  DELIVERED: 'DELIVERY_CONFIRMED',
  DELIVERY_CONFIRMED: 'DOCUMENTS_COMPLETED',
  READY_FOR_BILLING: 'INCLUDED_IN_BILLING_REGISTER',
  INCLUDED_IN_BILLING_REGISTER: 'FINANCIALLY_CLOSED',
}

const CANCEL_FORBIDDEN = new Set([
  'DELIVERED',
  'DELIVERY_CONFIRMED',
  'DOCUMENTS_COMPLETED',
  'READY_FOR_BILLING',
  'INCLUDED_IN_BILLING_REGISTER',
  'FINANCIALLY_CLOSED',
  'CANCELLED',
])

export interface Shipment {
  id: string
  tenant_id: string
  shipment_number: string
  status: string
  transport_order_id?: string | null
  shipper_company_id?: string | null
  consignee_company_id?: string | null
  carrier_company_id?: string | null
  forwarder_company_id?: string | null
  driver_id?: string | null
  vehicle_id?: string | null
  origin_location_id?: string | null
  destination_location_id?: string | null
  cargo_id?: string | null
  transport_mode?: string | null
  planned_pickup_at?: string | null
  planned_delivery_at?: string | null
  actual_pickup_at?: string | null
  actual_delivery_at?: string | null
  created_at?: string
  updated_at?: string
  version?: number
}

export interface Driver {
  id: string
  tenant_id: string
  carrier_company_id: string
  user_id?: string | null
  full_name: string
  phone?: string | null
  license_number?: string | null
  license_country?: string | null
  preferred_locale?: string
  status: string
}

export interface Vehicle {
  id: string
  tenant_id: string
  carrier_company_id: string
  plate_number: string
  vehicle_type: string
  equipment_type?: string | null
  capacity_weight?: number | null
  capacity_volume?: number | null
  registration_country?: string
  status: string
}

export interface LocationSummary {
  id: string
  name?: string | null
  city?: string | null
  country_code?: string | null
  address_line?: string | null
}

export interface CargoSummary {
  id: string
  description?: string | null
  weight_kg?: number | null
  volume_m3?: number | null
}

export interface ListShipmentsFilters {
  status?: string
  shipper_company_id?: string
  consignee_company_id?: string
  carrier_company_id?: string
  search?: string
  limit?: number
  offset?: number
}

export interface CreateShipmentFromBidPayload {
  tenant_id: string
  shipment_number: string
  bid_id: string
  transport_order_id: string
  planned_pickup_at?: string
  planned_delivery_at?: string
}

export interface CreateShipmentFromOrderPayload {
  tenant_id: string
  shipment_number: string
  transport_order_id: string
  carrier_company_id: string
  forwarder_company_id?: string | null
  planned_pickup_at?: string
  planned_delivery_at?: string
}

export interface AssignDriverPayload {
  tenant_id: string
  driver_id: string
}

export interface AssignVehiclePayload {
  tenant_id: string
  vehicle_id: string
}

export interface AcceptShipmentPayload {
  tenant_id: string
}

export interface UpdateShipmentStatusPayload {
  tenant_id: string
  status: string
  actual_time?: string
}

export interface CancelShipmentPayload {
  tenant_id: string
  reason: string
}

export interface CreateDriverPayload {
  tenant_id: string
  carrier_company_id: string
  user_id?: string | null
  full_name: string
  phone?: string
  license_number?: string
  license_country: string
  preferred_locale: string
}

export interface CreateVehiclePayload {
  tenant_id: string
  carrier_company_id: string
  plate_number: string
  vehicle_type: string
  equipment_type?: string
  capacity_weight?: number
  capacity_volume?: number
  registration_country: string
}

export interface ListDriversFilters {
  carrier_company_id?: string
  status?: string
  limit?: number
  offset?: number
}

export interface ListVehiclesFilters {
  carrier_company_id?: string
  status?: string
  limit?: number
  offset?: number
}

export interface ShipmentFromBidForm {
  bid_id: string
  transport_order_id: string
  shipment_number: string
  planned_pickup_at: string
  planned_delivery_at: string
}

export interface ShipmentFromOrderForm {
  transport_order_id: string
  carrier_company_id: string
  forwarder_company_id: string
  shipment_number: string
  planned_pickup_at: string
  planned_delivery_at: string
}

export interface DriverCreateForm {
  carrier_company_id: string
  full_name: string
  phone: string
  license_number: string
  license_country: string
  preferred_locale: string
}

export interface VehicleCreateForm {
  carrier_company_id: string
  plate_number: string
  vehicle_type: string
  equipment_type: string
  capacity_weight: string
  capacity_volume: string
  registration_country: string
}

export interface ShipmentFormErrors {
  bid_id?: string
  transport_order_id?: string
  shipment_number?: string
  carrier_company_id?: string
  planned_pickup_at?: string
  planned_delivery_at?: string
}

export interface DriverFormErrors {
  full_name?: string
  carrier_company_id?: string
  license_country?: string
  preferred_locale?: string
}

export interface VehicleFormErrors {
  plate_number?: string
  carrier_company_id?: string
  vehicle_type?: string
  equipment_type?: string
  registration_country?: string
}

function defaultPickupLocal(): string {
  const d = new Date()
  d.setDate(d.getDate() + 7)
  d.setHours(9, 0, 0, 0)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function defaultDeliveryLocal(): string {
  const d = new Date(defaultPickupLocal())
  d.setDate(d.getDate() + 2)
  d.setHours(18, 0, 0, 0)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export function emptyShipmentFromBidForm(bidId = '', transportOrderId = ''): ShipmentFromBidForm {
  return {
    bid_id: bidId,
    transport_order_id: transportOrderId,
    shipment_number: `SH-${Date.now().toString().slice(-6)}`,
    planned_pickup_at: defaultPickupLocal(),
    planned_delivery_at: defaultDeliveryLocal(),
  }
}

export function emptyShipmentFromOrderForm(transportOrderId = ''): ShipmentFromOrderForm {
  return {
    transport_order_id: transportOrderId,
    carrier_company_id: '',
    forwarder_company_id: '',
    shipment_number: `SH-${Date.now().toString().slice(-6)}`,
    planned_pickup_at: defaultPickupLocal(),
    planned_delivery_at: defaultDeliveryLocal(),
  }
}

export function emptyDriverCreateForm(carrierCompanyId = ''): DriverCreateForm {
  return {
    carrier_company_id: carrierCompanyId,
    full_name: '',
    phone: '+79990000000',
    license_number: '77AA123456',
    license_country: 'RU',
    preferred_locale: 'ru-RU',
  }
}

export function emptyVehicleCreateForm(carrierCompanyId = ''): VehicleCreateForm {
  return {
    carrier_company_id: carrierCompanyId,
    plate_number: 'А123ВС777',
    vehicle_type: 'TRUCK',
    equipment_type: 'TENT_20T',
    capacity_weight: '20000',
    capacity_volume: '82',
    registration_country: 'RU',
  }
}

export function toRFC3339(value: string): string {
  if (!value.trim()) return ''
  if (value.includes('T') && !value.endsWith('Z') && !value.includes('+')) {
    return new Date(value).toISOString()
  }
  return value
}

export function validateShipmentFromBidForm(form: ShipmentFromBidForm): ShipmentFormErrors {
  const errors: ShipmentFormErrors = {}
  if (!form.bid_id.trim()) errors.bid_id = 'required'
  if (!form.transport_order_id.trim()) errors.transport_order_id = 'required'
  if (!form.shipment_number.trim()) errors.shipment_number = 'required'
  if (!form.planned_pickup_at.trim()) errors.planned_pickup_at = 'required'
  if (!form.planned_delivery_at.trim()) errors.planned_delivery_at = 'required'
  if (
    form.planned_pickup_at &&
    form.planned_delivery_at &&
    form.planned_delivery_at < form.planned_pickup_at
  ) {
    errors.planned_delivery_at = 'range'
  }
  return errors
}

export function validateShipmentFromOrderForm(form: ShipmentFromOrderForm): ShipmentFormErrors {
  const errors: ShipmentFormErrors = {}
  if (!form.transport_order_id.trim()) errors.transport_order_id = 'required'
  if (!form.carrier_company_id.trim()) errors.carrier_company_id = 'required'
  if (!form.shipment_number.trim()) errors.shipment_number = 'required'
  if (!form.planned_pickup_at.trim()) errors.planned_pickup_at = 'required'
  if (!form.planned_delivery_at.trim()) errors.planned_delivery_at = 'required'
  if (
    form.planned_pickup_at &&
    form.planned_delivery_at &&
    form.planned_delivery_at < form.planned_pickup_at
  ) {
    errors.planned_delivery_at = 'range'
  }
  return errors
}

export function validateDriverCreateForm(form: DriverCreateForm): DriverFormErrors {
  const errors: DriverFormErrors = {}
  if (!form.full_name.trim()) errors.full_name = 'required'
  if (!form.carrier_company_id.trim()) errors.carrier_company_id = 'required'
  if (!form.license_country.trim() || form.license_country.trim().length !== 2) {
    errors.license_country = 'countryCode'
  }
  if (!form.preferred_locale.trim()) errors.preferred_locale = 'required'
  return errors
}

export function validateVehicleCreateForm(form: VehicleCreateForm): VehicleFormErrors {
  const errors: VehicleFormErrors = {}
  if (!form.plate_number.trim()) errors.plate_number = 'required'
  if (!form.carrier_company_id.trim()) errors.carrier_company_id = 'required'
  if (!form.vehicle_type.trim()) errors.vehicle_type = 'required'
  if (!form.equipment_type.trim()) errors.equipment_type = 'required'
  if (!form.registration_country.trim() || form.registration_country.trim().length !== 2) {
    errors.registration_country = 'countryCode'
  }
  return errors
}

export function hasFormErrors<T extends object>(errors: T): boolean {
  return Object.keys(errors).length > 0
}

export function getNextShipmentStatus(status: string): string | null {
  return NEXT_STATUS[status] ?? null
}

export function canCancelShipment(status: string): boolean {
  return !CANCEL_FORBIDDEN.has(status)
}

export function canAssignDriver(status: string): boolean {
  return ['CARRIER_ASSIGNED', 'ACCEPTED_BY_CARRIER', 'VEHICLE_ASSIGNED'].includes(status)
}

export function canAssignVehicle(status: string): boolean {
  return ['CARRIER_ASSIGNED', 'ACCEPTED_BY_CARRIER', 'DRIVER_ASSIGNED'].includes(status)
}

export function formatShipmentDate(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export function shortId(value?: string | null): string {
  if (!value) return '—'
  return value.length > 8 ? `${value.slice(0, 8)}…` : value
}
