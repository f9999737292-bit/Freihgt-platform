import { describe, expect, it } from 'vitest'
import {
  buildCarrierRouteMatrix,
  CARRIER_RESPONSE_API_ROUTES,
  FRONTEND_CARRIER_RESPONSE_PARITY,
} from '~/utils/carrierResponseApiRoutes'

describe('carrierResponseApiRoutes parity', () => {
  it('exports six canonical carrier-response routes', () => {
    expect(FRONTEND_CARRIER_RESPONSE_PARITY).toHaveLength(6)
  })

  it('matches frozen v3.0C API matrix', () => {
    const matrix = buildCarrierRouteMatrix()
    expect(matrix).toEqual([
      {
        frontendMethod: 'getCarrierResponse',
        httpMethod: 'GET',
        path: '/api/v1/rfx-events/{id}/carrier-response',
        requestDto: undefined,
        responseDto: 'RfxCarrierResponseWorkspace',
        operationId: 'getCarrierResponse',
      },
      {
        frontendMethod: 'startCarrierResponse',
        httpMethod: 'POST',
        path: '/api/v1/rfx-events/{id}/carrier-response/start',
        requestDto: undefined,
        responseDto: 'RfxCarrierResponseWorkspace',
        operationId: 'startCarrierResponse',
      },
      {
        frontendMethod: 'patchCarrierAnswers',
        httpMethod: 'PATCH',
        path: '/api/v1/rfx-events/{id}/carrier-response/answers',
        requestDto: 'RfxCarrierAnswerBatchPatch',
        responseDto: 'RfxCarrierResponseSaveResult',
        operationId: 'patchCarrierAnswers',
      },
      {
        frontendMethod: 'validateCarrierResponse',
        httpMethod: 'POST',
        path: '/api/v1/rfx-events/{id}/carrier-response/validate',
        requestDto: undefined,
        responseDto: 'RfxCarrierResponseValidationResult',
        operationId: 'validateCarrierResponse',
      },
      {
        frontendMethod: 'submitCarrierResponse',
        httpMethod: 'POST',
        path: '/api/v1/rfx-events/{id}/carrier-response/submit',
        requestDto: 'RfxCarrierResponseSubmitRequest',
        responseDto: 'RfxCarrierResponseSubmitResult',
        operationId: 'submitCarrierResponse',
      },
      {
        frontendMethod: 'getCarrierResponseSummary',
        httpMethod: 'GET',
        path: '/api/v1/rfx-events/{id}/carrier-response/summary',
        requestDto: undefined,
        responseDto: 'RfxCarrierResponseValidationResult',
        operationId: 'getCarrierResponseSummary',
      },
    ])
  })

  it('has no duplicate operation ids', () => {
    const ids = Object.values(CARRIER_RESPONSE_API_ROUTES).map((r) => r.operationId)
    expect(new Set(ids).size).toBe(ids.length)
  })
})
