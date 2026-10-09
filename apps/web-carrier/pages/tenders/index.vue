<script setup lang="ts">
import { Badge, Button, Card, CustomerPageHeader, Input, Select, Table } from '@freight-platform/ui'
import PortalState from '../../components/PortalState.vue'
import { viewKindFromError, type PortalViewKind } from '../../domain/viewState'
import type { TenderSummary } from '../../composables/useCarrierApi'

const { t } = useI18n()
const localePath = useLocalePath()
const api = useCarrierApi()
const items = ref<TenderSummary[]>([])
const kind = ref<PortalViewKind>('loading')
const detail = ref('')
const status = ref('')
const search = ref('')
const responseFilter = ref('')

const filterOptions = [
  { value: '', label: t('carrier.filterAny') },
  { value: 'OPEN_FOR_RESPONSE', label: 'OPEN_FOR_RESPONSE' },
  { value: 'RESPONDED', label: 'RESPONDED' },
  { value: 'NOT_RESPONDED', label: 'NOT_RESPONDED' },
  { value: 'CLOSED', label: 'CLOSED' },
]

async function load() {
  kind.value = 'loading'
  items.value = []
  detail.value = ''
  try {
    const data = await api.listTenders({
      status: status.value.trim() || undefined,
      search: search.value.trim() || undefined,
      response_filter: responseFilter.value || undefined,
    })
    items.value = data.items ?? []
    kind.value = items.value.length === 0 ? 'empty' : 'ready'
  } catch (error) {
    items.value = []
    kind.value = viewKindFromError(error)
    detail.value = error instanceof Error ? error.message : ''
  }
}

onMounted(load)

function titleFor(state: PortalViewKind) {
  if (state === 'empty') return t('carrier.emptyTenders')
  if (state === 'forbidden') return t('carrier.forbidden')
  if (state === 'not_found') return t('carrier.notFound')
  if (state === 'loading') return t('carrier.loading')
  return t('carrier.unavailable')
}
</script>

<template>
  <section data-testid="tender-inbox">
    <CustomerPageHeader :title="t('carrier.tendersTitle')" />
    <Card>
      <form class="filters" @submit.prevent="load">
        <Input v-model="search" name="search" :label="t('carrier.search')" />
        <Input v-model="status" name="status" :label="t('carrier.status')" />
        <Select v-model="responseFilter" name="response_filter" :label="t('carrier.responseFilter')" :options="filterOptions" />
        <Button type="submit">{{ t('carrier.apply') }}</Button>
      </form>
    </Card>
    <PortalState v-if="kind !== 'ready'" :kind="kind" :title="titleFor(kind)" :description="detail" />
    <Card v-else>
      <Table :columns="[t('carrier.tendersTitle'), t('carrier.status'), '']">
        <tr v-for="item in items" :key="item.id">
          <td>
            <div>{{ item.rfx_number }}</div>
            <div>{{ item.title }}</div>
          </td>
          <td><Badge :label="item.status || item.own_response_status || ''" /></td>
          <td>
            <NuxtLink :to="localePath(`/tenders/${item.id}`)" data-testid="open-tender">{{ t('carrier.open') }}</NuxtLink>
          </td>
        </tr>
      </Table>
    </Card>
  </section>
</template>

<style scoped>
.filters {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(12rem, 1fr));
  gap: 0.75rem;
  align-items: end;
}
</style>
