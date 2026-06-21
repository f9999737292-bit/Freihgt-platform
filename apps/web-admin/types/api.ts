export interface ApiErrorBody {
  error: {
    code: string
    message: string
    details: Record<string, unknown>
  }
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  limit: number
  offset: number
}

export interface LoginRequest {
  tenant_id: string
  email: string
  password: string
}

export interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
  user: AuthUser
}

export interface AuthUser {
  id: string
  tenant_id: string
  email: string
  full_name: string
  preferred_locale: string
  status?: string
}

export interface GatewayHealth {
  status: string
  service?: string
}

export type ApiGatewayStatus = 'online' | 'offline' | 'checking'
