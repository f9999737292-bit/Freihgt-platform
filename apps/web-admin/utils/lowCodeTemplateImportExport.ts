import {
  MAX_TEMPLATE_IMPORT_PAYLOAD_BYTES,
  TEMPLATE_EXPORT_SCHEMA_VERSION,
  type ImportConflictStrategy,
  type ImportMode,
  type ImportPreviewRequest,
  type TemplateExportEnvelope,
} from '~/types/lowCode'

export class TemplateImportJsonError extends Error {
  constructor(
    message: string,
    readonly code:
      | 'INVALID_JSON'
      | 'UNSUPPORTED_SCHEMA'
      | 'MISSING_TEMPLATE'
      | 'PAYLOAD_TOO_LARGE'
      | 'UNSUPPORTED_FIELD'
      | 'FORBIDDEN_FIELD',
  ) {
    super(message)
    this.name = 'TemplateImportJsonError'
  }
}

const ALLOWED_IMPORT_TOP_LEVEL_KEYS = new Set([
  'schema_version',
  'mode',
  'conflict_strategy',
  'target_code',
  'allow_system_fields',
  'template',
  'source_metadata',
  'source',
  'exported_at',
  'metadata',
])

const FORBIDDEN_IMPORT_TOP_LEVEL_KEYS = new Set([
  'custom_values',
  'audit_events',
  'values',
  'execute',
  'script',
  'publish',
])

export function buildTemplateExportFilename(envelope: TemplateExportEnvelope): string {
  const { entity_type, code, version } = envelope.template
  const safeCode = code.replace(/[^a-zA-Z0-9._-]/g, '_')
  return `lowcode-template-${entity_type.toLowerCase()}-${safeCode}-v${version}.json`
}

export function parseTemplateImportJsonText(
  rawText: string,
  options: {
    conflictStrategy: ImportConflictStrategy
    targetCode?: string
    mode?: ImportMode
  },
): ImportPreviewRequest {
  const trimmed = rawText.trim()
  if (!trimmed) {
    throw new TemplateImportJsonError('EMPTY', 'INVALID_JSON')
  }

  const byteLength = new TextEncoder().encode(trimmed).length
  if (byteLength > MAX_TEMPLATE_IMPORT_PAYLOAD_BYTES) {
    throw new TemplateImportJsonError('PAYLOAD_TOO_LARGE', 'PAYLOAD_TOO_LARGE')
  }

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    throw new TemplateImportJsonError('INVALID_JSON', 'INVALID_JSON')
  }

  return buildImportPreviewRequestFromParsed(parsed, options)
}

function validateImportTopLevelKeys(body: Record<string, unknown>): void {
  for (const key of Object.keys(body)) {
    if (FORBIDDEN_IMPORT_TOP_LEVEL_KEYS.has(key)) {
      throw new TemplateImportJsonError(key, 'FORBIDDEN_FIELD')
    }
    if (!ALLOWED_IMPORT_TOP_LEVEL_KEYS.has(key)) {
      throw new TemplateImportJsonError(key, 'UNSUPPORTED_FIELD')
    }
  }
}

export function buildImportPreviewRequestFromParsed(
  parsed: unknown,
  options: {
    conflictStrategy: ImportConflictStrategy
    targetCode?: string
    mode?: ImportMode
  },
): ImportPreviewRequest {
  if (!parsed || typeof parsed !== 'object') {
    throw new TemplateImportJsonError('INVALID_JSON', 'INVALID_JSON')
  }

  const body = parsed as Record<string, unknown>
  validateImportTopLevelKeys(body)

  const schemaVersion = typeof body.schema_version === 'string' ? body.schema_version.trim() : ''
  if (!schemaVersion) {
    throw new TemplateImportJsonError('MISSING_SCHEMA', 'UNSUPPORTED_SCHEMA')
  }
  if (schemaVersion !== TEMPLATE_EXPORT_SCHEMA_VERSION) {
    throw new TemplateImportJsonError(schemaVersion, 'UNSUPPORTED_SCHEMA')
  }

  const template = body.template
  if (!template || typeof template !== 'object') {
    throw new TemplateImportJsonError('MISSING_TEMPLATE', 'MISSING_TEMPLATE')
  }

  const targetCode = options.targetCode?.trim() || undefined
  const request: ImportPreviewRequest = {
    schema_version: schemaVersion,
    mode: options.mode ?? 'CREATE_DRAFT',
    conflict_strategy: options.conflictStrategy,
    template: template as ImportPreviewRequest['template'],
  }

  if (targetCode) {
    request.target_code = targetCode
  }

  if (body.source_metadata && typeof body.source_metadata === 'object') {
    request.source_metadata = body.source_metadata as ImportPreviewRequest['source_metadata']
  } else if (body.source && typeof body.source === 'object') {
    request.source = body.source as ImportPreviewRequest['source']
  }

  if (body.metadata && typeof body.metadata === 'object') {
    request.metadata = body.metadata as ImportPreviewRequest['metadata']
  }
  if (typeof body.exported_at === 'string' && body.exported_at.trim()) {
    request.exported_at = body.exported_at.trim()
  }

  return request
}

export function formatTemplateExportJson(envelope: TemplateExportEnvelope): string {
  return JSON.stringify(envelope, null, 2)
}

export function downloadJsonFile(filename: string, content: string): void {
  const blob = new Blob([content], { type: 'application/json;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}
