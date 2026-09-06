/** RFx v3.0C carrier response routes — parity anchor vs OpenAPI. */

export type HttpMethod = 'GET' | 'POST' | 'PATCH'

export interface CarrierApiRouteSpec {
  method: HttpMethod
  path: string
  caller: string
  requestDto?: string
  responseDto: string
  operationId: string
}

const EVENT = '{id}'

export const CARRIER_RESPONSE_API_ROUTES = {
  getCarrierResponse: {
    method: 'GET',
    path: `/api/v1/rfx-events/${EVENT}/carrier-response`,
    caller: 'getCarrierResponse',
    responseDto: 'RfxCarrierResponseWorkspace',
    operationId: 'getCarrierResponse',
  },
  startCarrierResponse: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/carrier-response/start`,
    caller: 'startCarrierResponse',
    responseDto: 'RfxCarrierResponseWorkspace',
    operationId: 'startCarrierResponse',
  },
  patchCarrierAnswers: {
    method: 'PATCH',
    path: `/api/v1/rfx-events/${EVENT}/carrier-response/answers`,
    caller: 'patchCarrierAnswers',
    requestDto: 'RfxCarrierAnswerBatchPatch',
    responseDto: 'RfxCarrierResponseSaveResult',
    operationId: 'patchCarrierAnswers',
  },
  validateCarrierResponse: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/carrier-response/validate`,
    caller: 'validateCarrierResponse',
    responseDto: 'RfxCarrierResponseValidationResult',
    operationId: 'validateCarrierResponse',
  },
  submitCarrierResponse: {
    method: 'POST',
    path: `/api/v1/rfx-events/${EVENT}/carrier-response/submit`,
    caller: 'submitCarrierResponse',
    requestDto: 'RfxCarrierResponseSubmitRequest',
    responseDto: 'RfxCarrierResponseSubmitResult',
    operationId: 'submitCarrierResponse',
  },
  getCarrierResponseSummary: {
    method: 'GET',
    path: `/api/v1/rfx-events/${EVENT}/carrier-response/summary`,
    caller: 'getCarrierResponseSummary',
    responseDto: 'RfxCarrierResponseValidationResult',
    operationId: 'getCarrierResponseSummary',
  },
} as const satisfies Record<string, CarrierApiRouteSpec>

export const FRONTEND_CARRIER_RESPONSE_PARITY: readonly CarrierApiRouteSpec[] =
  Object.values(CARRIER_RESPONSE_API_ROUTES)

export function carrierResponseApiPath(eventId: string, suffix = ''): string {
  const base = `/api/v1/rfx-events/${encodeURIComponent(eventId)}/carrier-response`
  return suffix ? `${base}/${suffix.replace(/^\//, '')}` : base
}

/** Route matrix row for documentation / tests. */
export interface CarrierRouteMatrixRow {
  frontendMethod: string
  httpMethod: HttpMethod
  path: string
  requestDto?: string
  responseDto: string
  operationId: string
}

export function buildCarrierRouteMatrix(): CarrierRouteMatrixRow[] {
  return FRONTEND_CARRIER_RESPONSE_PARITY.map((route) => ({
    frontendMethod: route.caller,
    httpMethod: route.method,
    path: route.path,
    requestDto: route.requestDto,
    responseDto: route.responseDto,
    operationId: route.operationId,
  }))
}
