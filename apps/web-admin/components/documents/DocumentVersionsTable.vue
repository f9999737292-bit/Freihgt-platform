<script setup lang="ts">
import { formatDocumentDate, formatPayloadJson, type DocumentVersion } from '~/types/document'

defineProps<{ versions: DocumentVersion[]; loading?: boolean }>()
</script>

<template>
  <UiCard>
    <template #header>
      <h3 class="card-title">{{ $t('documents.versions') }}</h3>
    </template>

    <div v-if="loading" class="text-muted">{{ $t('common.loading') }}</div>
    <UiEmptyState v-else-if="!versions.length" :title="$t('documents.noVersions')" />

    <UiTable
      v-else
      :columns="[
        $t('documents.versionNumber'),
        $t('freightRequests.createdAt'),
        $t('documents.payloadJson'),
        $t('documents.payloadXmlPath'),
        $t('documents.pdfFilePath'),
      ]"
    >
      <tr v-for="version in versions" :key="version.id">
        <td>{{ version.version_number }}</td>
        <td>{{ formatDocumentDate(version.created_at) }}</td>
        <td><pre class="payload-preview">{{ formatPayloadJson(version.payload_json) }}</pre></td>
        <td>{{ version.payload_xml_path || '—' }}</td>
        <td>{{ version.pdf_file_path || '—' }}</td>
      </tr>
    </UiTable>
  </UiCard>
</template>

<style scoped>
.card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
}

.payload-preview {
  margin: 0;
  max-width: 280px;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.75rem;
  color: var(--color-text-muted);
}
</style>
