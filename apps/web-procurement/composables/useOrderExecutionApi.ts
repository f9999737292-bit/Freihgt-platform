import type {
  BuyerTransportOrderItem,
  CarrierTransportOrderItem,
  ExecuteTransportOrderResult,
  OrderExecutionView,
} from '~/types/orderExecution'

export function useOrderExecutionApi() {
  const { apiGet, apiPost } = useApi()

  async function getOrderExecution(orderId: string, companyId: string, actor: 'BUYER' | 'CARRIER') {
    const params = new URLSearchParams({ company_id: companyId, actor })
    return apiGet<OrderExecutionView>(
      `/api/v1/order-execution/transport-orders/${encodeURIComponent(orderId)}?${params.toString()}`,
    )
  }

  async function listCarrierTransportOrders(carrierCompanyId: string, limit = 50, offset = 0) {
    const params = new URLSearchParams({
      carrier_company_id: carrierCompanyId,
      limit: String(limit),
      offset: String(offset),
    })
    const data = await apiGet<{ items: CarrierTransportOrderItem[]; total: number }>(
      `/api/v1/carrier/transport-orders?${params.toString()}`,
    )
    return data.items ?? []
  }

  async function listBuyerTransportOrders(buyerCompanyId: string, limit = 50, offset = 0) {
    const params = new URLSearchParams({
      buyer_company_id: buyerCompanyId,
      limit: String(limit),
      offset: String(offset),
    })
    const data = await apiGet<{ items: BuyerTransportOrderItem[]; total: number }>(
      `/api/v1/order-execution/buyer/transport-orders?${params.toString()}`,
    )
    return data.items ?? []
  }

  async function executeTransportOrder(orderId: string, carrierCompanyId: string, shipmentNumber: string) {
    const params = new URLSearchParams({ carrier_company_id: carrierCompanyId })
    return apiPost<ExecuteTransportOrderResult>(
      `/api/v1/order-execution/transport-orders/${encodeURIComponent(orderId)}/execute?${params.toString()}`,
      { shipment_number: shipmentNumber },
    )
  }

  async function startExecution(orderId: string, carrierCompanyId: string) {
    const params = new URLSearchParams({ carrier_company_id: carrierCompanyId })
    return apiPost<{ id: string; status: string }>(
      `/api/v1/order-execution/transport-orders/${encodeURIComponent(orderId)}/start?${params.toString()}`,
      {},
    )
  }

  async function assignDriver(shipmentId: string, driverId: string) {
    return apiPost(`/api/v1/shipments/${encodeURIComponent(shipmentId)}/assign-driver`, { driver_id: driverId })
  }

  async function assignVehicle(shipmentId: string, vehicleId: string) {
    return apiPost(`/api/v1/shipments/${encodeURIComponent(shipmentId)}/assign-vehicle`, { vehicle_id: vehicleId })
  }

  return {
    getOrderExecution,
    listCarrierTransportOrders,
    listBuyerTransportOrders,
    executeTransportOrder,
    startExecution,
    assignDriver,
    assignVehicle,
  }
}
