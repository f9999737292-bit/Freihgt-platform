<script setup lang="ts">
import { emptyI18nMap } from '~/utils/rfxTemplateI18n'
import { formatRfxApiError } from '~/utils/rfxApiError'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; created: [id: string] }>()

const { createTemplate } = useRfxTemplateApi()
const { pushToast } = useToast()
const { t, locale } = useI18n()
const router = useRouter()

const saving = ref(false)
const templateCode = ref('')
const name = ref('')

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      templateCode.value = ''
      name.value = ''
    }
  },
)

async function handleCreate() {
  if (!templateCode.value.trim() || !name.value.trim()) return
  saving.value = true
  try {
    const result = await createTemplate({
      template_code: templateCode.value.trim(),
      name_i18n: { ...emptyI18nMap(locale.value), [locale.value]: name.value.trim() },
    })
    pushToast('success', t('rfx.templates.create'))
    emit('created', result.template.id)
    emit('close')
    await router.push(`/rfx/templates/${result.template.id}`)
  } catch (e) {
    pushToast('error', formatRfxApiError(e, t))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="open" class="modal-backdrop" role="dialog" aria-modal="true" @click.self="emit('close')">
    <div class="modal">
      <h2>{{ $t('rfx.templates.create') }}</h2>
      <label class="modal__label">
        {{ $t('rfx.templates.columns.code') }}
        <input v-model="templateCode" required />
      </label>
      <label class="modal__label">
        {{ $t('rfx.templates.columns.name') }}
        <input v-model="name" required />
      </label>
      <div class="modal__actions">
        <button type="button" class="btn btn--secondary" @click="emit('close')">{{ $t('common.cancel') }}</button>
        <button type="button" class="btn btn--primary" :disabled="saving" @click="handleCreate">
          {{ $t('common.create') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop { position: fixed; inset: 0; background: rgba(0,0,0,.45); display: grid; place-items: center; z-index: 1000; padding: 1rem; }
.modal { background: #fff; border-radius: var(--radius-lg); padding: 1.5rem; width: min(100%, 28rem); display: flex; flex-direction: column; gap: 0.75rem; }
.modal__label { display: flex; flex-direction: column; gap: 0.375rem; font-size: 0.875rem; }
.modal__actions { display: flex; justify-content: flex-end; gap: 0.5rem; }
</style>
