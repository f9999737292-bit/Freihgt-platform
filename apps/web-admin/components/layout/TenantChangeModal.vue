<script setup lang="ts">
const props = defineProps<{ open: boolean; initialTenantId?: string }>()
const emit = defineEmits<{ close: [] }>()

const { applyTenant } = useTenantContext()
const { t } = useI18n()

const tenantInput = ref('')
const saving = ref(false)

watch(
  () => props.open,
  (open) => {
    if (!open) return
    tenantInput.value = props.initialTenantId || ''
  },
)

async function save() {
  saving.value = true
  try {
    const ok = await applyTenant(tenantInput.value)
    if (ok) {
      emit('close')
    }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal :open="open" :title="$t('tenant.change')" @close="$emit('close')">
    <UiInput v-model="tenantInput" :label="$t('tenant.tenantId')" required />
    <template #footer>
      <UiButton variant="secondary" @click="$emit('close')">{{ $t('common.cancel') }}</UiButton>
      <UiButton :loading="saving" @click="save">{{ $t('tenant.save') }}</UiButton>
    </template>
  </UiModal>
</template>
