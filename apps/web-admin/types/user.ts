import { PREFERRED_LOCALES } from '~/types/company'
import type { CompanyMemberRole } from '~/types/company'

export const USER_STATUSES = ['ACTIVE', 'DELETED'] as const

export interface User {
  id: string
  tenant_id: string
  email: string
  phone?: string | null
  full_name: string
  preferred_locale: string
  status: string
  created_at?: string
  updated_at?: string
}

export interface CreateUserPayload {
  tenant_id: string
  email: string
  phone?: string
  full_name: string
  password: string
  preferred_locale: string
}

export interface UpdateUserPayload {
  full_name?: string
  phone?: string
  preferred_locale?: string
  status?: string
}

export interface ListUsersFilters {
  search?: string
  status?: string
  limit?: number
  offset?: number
}

export interface UserCompanyMembership {
  membership_id: string
  company_id: string
  legal_name: string
  short_name?: string | null
  company_type: string
  position?: string | null
  membership_status: string
  roles: CompanyMemberRole[]
}

export interface UserFormErrors {
  full_name?: string
  email?: string
  password?: string
  preferred_locale?: string
}

export interface AddMemberFormErrors {
  user_id?: string
  position?: string
}

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function emptyCreateUserForm(): {
  email: string
  phone: string
  full_name: string
  password: string
  preferred_locale: string
} {
  return {
    email: '',
    phone: '',
    full_name: '',
    password: '',
    preferred_locale: 'ru-RU',
  }
}

export function validateCreateUserForm(payload: {
  full_name: string
  email: string
  password: string
  preferred_locale: string
}): UserFormErrors {
  const errors: UserFormErrors = {}

  if (!payload.full_name.trim()) {
    errors.full_name = 'required'
  }
  if (!payload.email.trim()) {
    errors.email = 'required'
  } else if (!EMAIL_PATTERN.test(payload.email.trim())) {
    errors.email = 'invalid'
  }
  if (!payload.password.trim()) {
    errors.password = 'required'
  } else if (payload.password.trim().length < 8) {
    errors.password = 'minLength'
  }
  if (!payload.preferred_locale.trim()) {
    errors.preferred_locale = 'required'
  } else if (!PREFERRED_LOCALES.includes(payload.preferred_locale as (typeof PREFERRED_LOCALES)[number])) {
    errors.preferred_locale = 'invalid'
  }

  return errors
}

export function validateAddMemberForm(payload: { user_id: string; position: string }): AddMemberFormErrors {
  const errors: AddMemberFormErrors = {}

  if (!payload.user_id.trim()) {
    errors.user_id = 'required'
  }
  if (!payload.position.trim()) {
    errors.position = 'required'
  }

  return errors
}

export function hasUserFormErrors(errors: UserFormErrors | AddMemberFormErrors): boolean {
  return Object.keys(errors).length > 0
}

export function formatUserDate(value?: string | null): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}
