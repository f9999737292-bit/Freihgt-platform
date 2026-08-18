import { ref } from 'vue'
import type { RequestResult } from '@/types/api'

export function useSubmissionLock() {
  const submitting = ref(false)

  async function runOnce<T>(action: () => Promise<RequestResult<T>>): Promise<RequestResult<T>> {
    if (submitting.value) {
      return { outcome: 'REQUEST_NOT_SENT' }
    }
    submitting.value = true
    try {
      return await action()
    } finally {
      submitting.value = false
    }
  }

  return { submitting, runOnce }
}

export function draftKey(kind: 'delay' | 'problem' | 'milestone' | 'pod', shipmentId: string) {
  return `freight-driver-draft:${kind}:${shipmentId}`
}

export function loadDraft<T>(key: string): T | null {
  try {
    const raw = sessionStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : null
  } catch {
    return null
  }
}

export function saveDraft<T>(key: string, value: T) {
  sessionStorage.setItem(key, JSON.stringify(value))
}

export function clearDraft(key: string) {
  sessionStorage.removeItem(key)
}
