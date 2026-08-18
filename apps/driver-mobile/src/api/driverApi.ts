import type { HttpClient } from '@/api/client'
import type { LoginRequest, LoginResponse } from '@/types/auth'
import type {
  DriverDelayRequest,
  DriverDelayResponse,
  DriverExceptionRequest,
  DriverExceptionResponse,
  DriverMe,
  DriverOperationalEventRequest,
  DriverOperationalEventResponse,
  DriverPODUploadCompleteRequest,
  DriverPODUploadCompleteResponse,
  DriverPODUploadIntent,
  DriverPODUploadRequest,
  DriverShipmentDetail,
  DriverShipmentListResponse,
} from '@/types/driver'

export function createDriverApi(http: HttpClient) {
  return {
    login(payload: LoginRequest & { tenant_id: string }) {
      return http.request<LoginResponse>('POST', '/api/v1/auth/login', {
        body: payload,
        skipAuth: true,
      })
    },

    getMyProfile() {
      return http.request<DriverMe>('GET', '/api/v1/driver/me')
    },

    getMyShipments(query?: { status?: string; limit?: number; offset?: number }) {
      return http.request<DriverShipmentListResponse>('GET', '/api/v1/driver/me/shipments', {
        query,
      })
    },

    getShipment(shipmentId: string) {
      return http.request<DriverShipmentDetail>(
        'GET',
        `/api/v1/driver/me/shipments/${encodeURIComponent(shipmentId)}`,
      )
    },

    reportDelay(shipmentId: string, body: DriverDelayRequest) {
      return http.request<DriverDelayResponse>(
        'POST',
        `/api/v1/driver/me/shipments/${encodeURIComponent(shipmentId)}/delays`,
        { body, idempotencyKey: body.idempotencyKey },
      )
    },

    reportProblem(shipmentId: string, body: DriverExceptionRequest) {
      return http.request<DriverExceptionResponse>(
        'POST',
        `/api/v1/driver/me/shipments/${encodeURIComponent(shipmentId)}/exceptions`,
        { body, idempotencyKey: body.idempotencyKey },
      )
    },

    recordEvent(shipmentId: string, body: DriverOperationalEventRequest) {
      return http.request<DriverOperationalEventResponse>(
        'POST',
        `/api/v1/driver/me/shipments/${encodeURIComponent(shipmentId)}/events`,
        { body, idempotencyKey: body.idempotencyKey },
      )
    },

    initiatePODUpload(shipmentId: string, body: DriverPODUploadRequest) {
      return http.request<DriverPODUploadIntent>(
        'POST',
        `/api/v1/driver/me/shipments/${encodeURIComponent(shipmentId)}/pod/uploads`,
        { body, idempotencyKey: body.idempotencyKey },
      )
    },

    uploadPODContent(
      shipmentId: string,
      uploadId: string,
      uploadToken: string,
      content: Blob | ArrayBuffer | Uint8Array,
      mimeType: string,
    ) {
      return http.request<{ status: string }>(
        'PUT',
        `/api/v1/driver/me/shipments/${encodeURIComponent(shipmentId)}/pod/uploads/${encodeURIComponent(uploadId)}/content`,
        {
          rawBody: content,
          extraHeaders: {
            'X-Upload-Token': uploadToken,
            'Content-Type': mimeType,
          },
        },
      )
    },

    completePODUpload(
      shipmentId: string,
      uploadId: string,
      body: DriverPODUploadCompleteRequest,
    ) {
      return http.request<DriverPODUploadCompleteResponse>(
        'POST',
        `/api/v1/driver/me/shipments/${encodeURIComponent(shipmentId)}/pod/uploads/${encodeURIComponent(uploadId)}/complete`,
        { body },
      )
    },
  }
}

export type DriverApi = ReturnType<typeof createDriverApi>
