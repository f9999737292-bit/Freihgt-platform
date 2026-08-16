export interface LoginRequest {
  email: string
  password: string
}

export interface AuthUser {
  id: string
  tenant_id: string
  email: string
  full_name: string
  preferred_locale?: string
  status: string
  roles?: string[]
}

export interface LoginResponse {
  access_token: string
  user: AuthUser
}
